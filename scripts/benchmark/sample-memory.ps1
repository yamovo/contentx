param(
    [string]$Container = "",
    [string]$ProcessName = "",
    [int]$Samples = 12,
    [int]$IntervalSeconds = 5
)

# Idle memory sampler for the 7.2 memory comparison. Measures resident memory
# under a unified idle condition (no load) so the 1,000 vs 10,000 article
# footprint is comparable across drivers.
#
# Docker (containerized app, e.g. PostgreSQL/MySQL stacks):
#   pwsh scripts/benchmark/sample-memory.ps1 -Container contentx-bench-app
# Local process (e.g. SQLite run directly):
#   pwsh scripts/benchmark/sample-memory.ps1 -ProcessName contentx

$ErrorActionPreference = "Stop"

if (-not $Container -and -not $ProcessName) {
    throw "Provide either -Container <name> or -ProcessName <name>."
}

function Convert-ToMiB {
    param([string]$Value)
    # docker stats MemUsage looks like "145.4MiB / 7.6GiB"; take the first token.
    $used = ($Value -split "/")[0].Trim()
    if ($used -match "([0-9.]+)\s*([A-Za-z]+)") {
        $num = [double]$Matches[1]
        switch ($Matches[2].ToUpper()) {
            "B"   { return $num / 1MB }
            "KIB" { return $num / 1024 }
            "MIB" { return $num }
            "GIB" { return $num * 1024 }
            default { return $num }
        }
    }
    return 0
}

$readings = @()
for ($i = 1; $i -le $Samples; $i++) {
    if ($Container) {
        $raw = docker stats --no-stream --format "{{.MemUsage}}" $Container 2>$null
        if (-not $raw) { throw "Container '$Container' not found or not running." }
        $mib = Convert-ToMiB $raw
    }
    else {
        $proc = Get-Process -Name $ProcessName -ErrorAction SilentlyContinue | Select-Object -First 1
        if (-not $proc) { throw "Process '$ProcessName' not found." }
        $mib = $proc.WorkingSet64 / 1MB
    }
    $readings += [math]::Round($mib, 1)
    Write-Host ("sample {0,2}/{1}: {2} MiB" -f $i, $Samples, [math]::Round($mib, 1))
    if ($i -lt $Samples) { Start-Sleep -Seconds $IntervalSeconds }
}

$mean = [math]::Round((($readings | Measure-Object -Average).Average), 1)
$max = [math]::Round((($readings | Measure-Object -Maximum).Maximum), 1)
$min = [math]::Round((($readings | Measure-Object -Minimum).Minimum), 1)

Write-Host ""
Write-Host "=== idle memory over $Samples samples ==="
Write-Host ("min={0} MiB  mean={1} MiB  max={2} MiB" -f $min, $mean, $max)
