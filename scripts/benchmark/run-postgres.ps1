param(
    [string]$BaseUrl = "http://127.0.0.1:18080",
    [string]$Vegeta = "$env:USERPROFILE\.codex\visualizations\2026\07\22\019f898d-9f31-7920-b627-8e28e7f7c3d5\bin\vegeta.exe",
    [int]$ReadRate = 1000,
    [int]$WriteRate = 100,
    [string]$ReadDuration = "15s",
    [string]$WriteDuration = "10s",
    [string]$OutputDir = "reports\benchmarks\raw\postgres"
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$outputPath = Join-Path $repoRoot $OutputDir
New-Item -ItemType Directory -Force -Path $outputPath | Out-Null

if (-not (Test-Path $Vegeta)) {
    throw "Vegeta not found at $Vegeta"
}

$envFile = Join-Path $repoRoot ".env"
$passwordLine = Get-Content $envFile | Where-Object { $_ -like "ADMIN_PASSWORD=*" } | Select-Object -First 1
if (-not $passwordLine) {
    throw "ADMIN_PASSWORD is missing from .env"
}
$password = $passwordLine.Substring("ADMIN_PASSWORD=".Length)
$loginBody = @{ username = "admin"; password = $password } | ConvertTo-Json -Compress
$login = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/v1/auth/login" -ContentType "application/json" -Body $loginBody
$token = $login.data.token.access_token
$authHeader = "Authorization: Bearer $token"

$list = Invoke-RestMethod -Uri "$BaseUrl/api/v1/articles?page=1&page_size=1" -Headers @{ Authorization = "Bearer $token" }
$articleID = $list.data.items[0].id
if (-not $articleID) {
    throw "No article found. Run seed_postgres.sql first."
}

$utf8 = New-Object System.Text.UTF8Encoding($false)
$graphqlBodyPath = Join-Path $outputPath "graphql-body.json"
[IO.File]::WriteAllText($graphqlBodyPath, '{"query":"{ articles(page:1,pageSize:20){ total items{ id title slug excerpt } } }"}', $utf8)
$writeBodyPath = Join-Path $outputPath "write-body.json"
[IO.File]::WriteAllText($writeBodyPath, '{"title":"Concurrent benchmark update","content":"ContentX concurrent write benchmark payload","revision_note":"vegeta benchmark"}', $utf8)

function Invoke-VegetaCase {
    param(
        [string]$Name,
        [string]$Method,
        [string]$Url,
        [int]$Rate,
        [string]$Duration,
        [string]$BodyPath = ""
    )

    $resultPath = Join-Path $outputPath "$Name.bin"
    $jsonPath = Join-Path $outputPath "$Name.json"
    $target = "$Method $Url"
    $attackArgs = @("attack", "-rate=$Rate", "-duration=$Duration", "-header=$authHeader", "-output=$resultPath")
    if ($BodyPath) {
        $attackArgs += "-header=Content-Type: application/json"
        $attackArgs += "-body=$BodyPath"
    }
    $target | & $Vegeta @attackArgs
    & $Vegeta report -type=json $resultPath | Set-Content -Encoding UTF8 $jsonPath
    & $Vegeta report $resultPath
}

Invoke-VegetaCase -Name "article-list" -Method "GET" -Url "$BaseUrl/api/v1/articles?page=1&page_size=20" -Rate $ReadRate -Duration $ReadDuration
Invoke-VegetaCase -Name "article-detail" -Method "GET" -Url "$BaseUrl/api/v1/articles/$articleID" -Rate $ReadRate -Duration $ReadDuration
Invoke-VegetaCase -Name "graphql" -Method "POST" -Url "$BaseUrl/api/v1/graphql" -Rate $ReadRate -Duration $ReadDuration -BodyPath $graphqlBodyPath
Invoke-VegetaCase -Name "concurrent-write" -Method "PUT" -Url "$BaseUrl/api/v1/articles/$articleID" -Rate $WriteRate -Duration $WriteDuration -BodyPath $writeBodyPath

Write-Host "Raw benchmark reports written to $outputPath"
