param(
    [ValidateSet("postgres", "mysql", "sqlite")]
    [string]$Driver = "postgres",
    [string]$BaseUrl = "",
    [string]$Vegeta = "",
    [int]$ReadRate = 1000,
    [int]$WriteRate = 100,
    [string]$ReadDuration = "15s",
    [string]$WriteDuration = "10s",
    [int]$CooldownSeconds = 5,
    [string]$MetricsPath = "/metrics",
    [string]$AdminPassword = "",
    [string]$OutputDir = ""
)

# Driver-agnostic load test. The database under test is whichever ContentX
# instance is answering on $BaseUrl; -Driver only selects the raw output folder
# so PostgreSQL / MySQL / SQLite results stay separate and comparable.
# Scenarios, rates and durations are identical across drivers by design.

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")

# Driver-aware default BaseUrl. Previously the default was always :18080, so a
# `-Driver mysql` run without -BaseUrl would silently hit a running PostgreSQL
# instance on :18080 and write its results into the MySQL output folder (S1-2).
if (-not $BaseUrl) {
    switch ($Driver) {
        "postgres" { $BaseUrl = "http://127.0.0.1:18080" }
        "mysql"    { $BaseUrl = "http://127.0.0.1:18090" }
        "sqlite"   { $BaseUrl = "http://127.0.0.1:18081" }
    }
}

# Resolve vegeta: explicit -Vegeta > PATH > project tools dirs. Do not depend on
# a personal session directory (S1-2).
function Resolve-VegetaPath {
    param([string]$Explicit)
    if ($Explicit) {
        if (-not (Test-Path $Explicit)) { throw "Vegeta not found at: $Explicit" }
        return (Resolve-Path $Explicit).Path
    }
    $cmd = Get-Command vegeta -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $candidates = @(
        (Join-Path $PSScriptRoot "tools\vegeta.exe"),
        (Join-Path $PSScriptRoot "..\..\tools\vegeta.exe"),
        (Join-Path $PSScriptRoot "..\..\bin\vegeta.exe")
    )
    foreach ($c in $candidates) {
        if (Test-Path $c) { return (Resolve-Path $c).Path }
    }
    throw "Vegeta not found. Install it on PATH, place vegeta.exe in scripts/benchmark/tools/ or repo tools/, or pass -Vegeta <path>."
}
$Vegeta = Resolve-VegetaPath -Explicit $Vegeta

if (-not $OutputDir) {
    $OutputDir = "reports\benchmarks\raw\$Driver"
}
$outputPath = Join-Path $repoRoot $OutputDir
New-Item -ItemType Directory -Force -Path $outputPath | Out-Null

# Resolve the admin password: explicit param wins, otherwise read from .env.
if (-not $AdminPassword) {
    $envFile = Join-Path $repoRoot ".env"
    $passwordLine = Get-Content $envFile | Where-Object { $_ -like "ADMIN_PASSWORD=*" } | Select-Object -First 1
    if (-not $passwordLine) {
        throw "ADMIN_PASSWORD is missing from .env (or pass -AdminPassword)"
    }
    $AdminPassword = $passwordLine.Substring("ADMIN_PASSWORD=".Length)
}
$loginBody = @{ username = "admin"; password = $AdminPassword } | ConvertTo-Json -Compress
$login = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/v1/auth/login" -ContentType "application/json" -Body $loginBody
$token = $login.data.token.access_token
$authHeader = "Authorization: Bearer $token"

$list = Invoke-RestMethod -Uri "$BaseUrl/api/v1/articles?page=1&page_size=1" -Headers @{ Authorization = "Bearer $token" }
$articleID = $list.data.items[0].id
if (-not $articleID) {
    throw "No article found. Seed the $Driver database first (see scripts/benchmark/seed_$Driver*.sql)."
}

$utf8 = New-Object System.Text.UTF8Encoding($false)
$graphqlBodyPath = Join-Path $outputPath "graphql-body.json"
[IO.File]::WriteAllText($graphqlBodyPath, '{"query":"{ articles(page:1,pageSize:20){ total items{ id title slug excerpt } } }"}', $utf8)
$writeBodyPath = Join-Path $outputPath "write-body.json"
[IO.File]::WriteAllText($writeBodyPath, '{"title":"Concurrent benchmark update","content":"ContentX concurrent write benchmark payload","revision_note":"vegeta benchmark"}', $utf8)

# Best-effort snapshot of process metrics (goroutines, open FDs, DB connection
# gauges) from the Prometheus endpoint. Records port + resource state per
# scenario so cross-run comparisons are auditable (S1-2). Silently skipped if
# metrics are disabled or unreachable.
function Get-MetricsSnapshot {
    param([string]$Label, [string]$MetricsUrl)
    $line = "[$(Get-Date -Format 'yyyy-MM-ddTHH:mm:ss')] $Label base=$BaseUrl"
    try {
        $resp = Invoke-WebRequest -Uri $MetricsUrl -TimeoutSec 3 -UseBasicParsing
        $m = $resp.Content -split "`n"
        $goroutines = ($m | Where-Object { $_ -match '^go_goroutines ' } | Select-Object -First 1)
        $openFds    = ($m | Where-Object { $_ -match '^process_open_fds ' } | Select-Object -First 1)
        $dbWait     = ($m | Where-Object { $_ -match 'db_connections.*waiting|sql_db_connections_waiting' } | Select-Object -First 1)
        $line += " goroutines=$(if ($goroutines) { ($goroutines -split '\s+')[-1] } else { 'n/a' })"
        $line += " open_fds=$(if ($openFds) { ($openFds -split '\s+')[-1] } else { 'n/a' })"
        if ($dbWait) { $line += " db_waiting=$(($dbWait -split '\s+')[-1])" }
    } catch {
        $line += " metrics=unavailable"
    }
    return $line
}

function Invoke-VegetaCase {
    param(
        [string]$Name,
        [string]$Method,
        [string]$Url,
        [int]$Rate,
        [string]$Duration,
        [string]$BodyPath = ""
    )

    $metricsUrl = "$BaseUrl$MetricsPath"
    $preSnapshot  = Get-MetricsSnapshot -Label "$Name pre"  -MetricsUrl $metricsUrl
    Write-Host $preSnapshot

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

    $postSnapshot = Get-MetricsSnapshot -Label "$Name post" -MetricsUrl $metricsUrl
    Write-Host $postSnapshot
    "$preSnapshot`n$postSnapshot" | Set-Content -Encoding UTF8 (Join-Path $outputPath "$Name.metrics.txt")
}

Write-Host "=== ContentX benchmark: driver=$Driver base=$BaseUrl vegeta=$Vegeta cooldown=${CooldownSeconds}s ==="
Invoke-VegetaCase -Name "article-list" -Method "GET" -Url "$BaseUrl/api/v1/articles?page=1&page_size=20" -Rate $ReadRate -Duration $ReadDuration
Start-Sleep -Seconds $CooldownSeconds
Invoke-VegetaCase -Name "article-detail" -Method "GET" -Url "$BaseUrl/api/v1/articles/$articleID" -Rate $ReadRate -Duration $ReadDuration
Start-Sleep -Seconds $CooldownSeconds
Invoke-VegetaCase -Name "graphql" -Method "POST" -Url "$BaseUrl/api/v1/graphql" -Rate $ReadRate -Duration $ReadDuration -BodyPath $graphqlBodyPath
Start-Sleep -Seconds $CooldownSeconds
Invoke-VegetaCase -Name "concurrent-write" -Method "PUT" -Url "$BaseUrl/api/v1/articles/$articleID" -Rate $WriteRate -Duration $WriteDuration -BodyPath $writeBodyPath

Write-Host "Raw benchmark reports written to $outputPath"

