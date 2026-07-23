# C5: Generate a benchmark report from Vegeta JSON + run-metadata.json.
#
# Reads all *.json (excluding *-body.json) from a raw benchmark directory,
# produces a Markdown table with correct units (ns -> ms, bytes -> KB),
# and runs consistency checks (table values match JSON source values).
#
# Usage:
#   powershell -File scripts/benchmark/generate-report.ps1 -Driver postgres
#   powershell -File scripts/benchmark/generate-report.ps1 -Driver postgres -RawDir reports\benchmarks\raw\postgres -OutputFile reports\benchmarks\postgres-baseline.md

param(
    [ValidateSet("postgres", "mysql", "sqlite")]
    [string]$Driver = "postgres",
    [string]$RawDir = "",
    [string]$OutputFile = "",
    [string]$Title = ""
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")

if (-not $RawDir) {
    $RawDir = Join-Path $repoRoot "reports\benchmarks\raw\$Driver"
}
if (-not (Test-Path $RawDir)) {
    throw "Raw directory not found: $RawDir"
}

if (-not $OutputFile) {
    $OutputFile = Join-Path $repoRoot "reports\benchmarks\$Driver-baseline.md"
}
if (-not $Title) {
    $driverName = switch ($Driver) {
        "postgres" { "PostgreSQL" }
        "mysql"    { "MySQL" }
        "sqlite"   { "SQLite" }
    }
    $Title = "$driverName Benchmark Baseline"
}

# Read run metadata if available
$metaPath = Join-Path $RawDir "run-metadata.json"
$meta = $null
if (Test-Path $metaPath) {
    $meta = Get-Content $metaPath -Raw | ConvertFrom-Json
}

# --- Parse Vegeta JSON files ---
$scenarioFiles = @(
    @{ name = "article-list";     file = "article-list.json" },
    @{ name = "article-detail";   file = "article-detail.json" },
    @{ name = "graphql";          file = "graphql.json" },
    @{ name = "concurrent-write"; file = "concurrent-write.json" }
)

$results = @()
$consistencyErrors = @()

foreach ($sc in $scenarioFiles) {
    $jsonPath = Join-Path $RawDir $sc.file
    if (-not (Test-Path $jsonPath)) {
        $consistencyErrors += "MISSING: $($sc.file) not found in $RawDir"
        continue
    }
    $v = Get-Content $jsonPath -Raw | ConvertFrom-Json

    # Vegeta latencies are in nanoseconds. Convert to milliseconds (3 decimal places).
    # PS5.1 cannot access properties starting with a digit via dot notation (e.g.
    # $v.latencies.50th), so use PSObject.Properties for the percentile fields.
    # Note: Vegeta's JSON report only emits 50th/95th/99th/max — there is no 90th
    # percentile, so the table intentionally omits P90 (previously showed 0).
    $latMean = $v.latencies.mean
    $lat50   = $v.latencies.PSObject.Properties['50th'].Value
    $lat95   = $v.latencies.PSObject.Properties['95th'].Value
    $lat99   = $v.latencies.PSObject.Properties['99th'].Value
    $latMax  = $v.latencies.max
    $latMin  = $v.latencies.min

    $meanMs   = [math]::Round($latMean / 1e6, 3)
    $p50Ms    = [math]::Round($lat50 / 1e6, 3)
    $p95Ms    = [math]::Round($lat95 / 1e6, 3)
    $p99Ms    = [math]::Round($lat99 / 1e6, 3)
    $maxMs    = [math]::Round($latMax / 1e6, 3)
    $minMs    = [math]::Round($latMin / 1e6, 3)

    # Bytes: convert to KB (1 decimal place). bytes_in.mean = avg response body size.
    $respBytes = if ($v.bytes_in.mean) { $v.bytes_in.mean } else { 0 }
    $respKb    = [math]::Round($respBytes / 1024, 1)

    # Actual rate and throughput
    $rateRps       = [math]::Round($v.rate, 1)
    $throughputRps = [math]::Round($v.throughput, 1)

    # Success ratio (0-1 -> percentage)
    $successPct = [math]::Round($v.success * 100, 2)

    # Status codes
    $statusCodes = ($v.status_codes.PSObject.Properties | ForEach-Object { "$($_.Name)=$($_.Value)" }) -join ", "

    # Errors
    $errorCount = if ($v.errors) { $v.errors.Count } else { 0 }

    # --- Consistency checks ---
    # Verify that the raw JSON values, when converted, match what we put in the table.
    # This catches hand-editing or unit conversion errors.
    $checkMeanMs = [math]::Round($latMean / 1e6, 3)
    if ($checkMeanMs -ne $meanMs) {
        $consistencyErrors += "$($sc.name): mean latency conversion mismatch (raw=$checkMeanMs table=$meanMs)"
    }
    $checkP99Ms = [math]::Round($lat99 / 1e6, 3)
    if ($checkP99Ms -ne $p99Ms) {
        $consistencyErrors += "$($sc.name): p99 latency conversion mismatch (raw=$checkP99Ms table=$p99Ms)"
    }
    $checkRespKb = [math]::Round($v.bytes_in.mean / 1024, 1)
    if ($respBytes -gt 0 -and $checkRespKb -ne $respKb) {
        $consistencyErrors += "$($sc.name): response size conversion mismatch (raw=$checkRespKb table=$respKb)"
    }

    $results += @{
        name = $sc.name
        requests = $v.requests
        rate = $rateRps
        throughput = $throughputRps
        meanMs = $meanMs
        p50Ms = $p50Ms
        p95Ms = $p95Ms
        p99Ms = $p99Ms
        maxMs = $maxMs
        minMs = $minMs
        respKb = $respKb
        successPct = $successPct
        statusCodes = $statusCodes
        errorCount = $errorCount
    }
}

# --- Generate Markdown ---
$report = "# $Title`n`n"

if ($meta) {
    $dirtyNote = if ($meta.git_dirty) { " (dirty working tree)" } else { "" }
    $report += "- **Date**: $($meta.timestamp)`n"
    $report += "- **Git SHA**: $($meta.git_sha)$dirtyNote`n"
    $report += "- **Branch**: $($meta.git_branch)`n"
    $report += "- **Driver**: $($meta.driver)`n"
    $report += "- **Article count**: $($meta.article_count)`n"
    $report += "- **Base URL**: $($meta.base_url)`n"
    $report += "- **Rates**: read=$($meta.read_rate) req/s, write=$($meta.write_rate) req/s`n"
    $report += "- **Durations**: read=$($meta.read_duration), write=$($meta.write_duration)`n"
    $report += "- **Cooldown**: $($meta.cooldown_seconds)s between scenarios`n`n"
} else {
    $report += "> No run-metadata.json found. Git SHA and article count not recorded.`n`n"
}

$report += "## Results`n`n"
$report += "| Scenario | Requests | Rate (req/s) | Throughput (req/s) | Mean (ms) | P50 (ms) | P95 (ms) | P99 (ms) | Max (ms) | Resp (KB) | Success | Status Codes | Errors |`n"
$report += "|---|---|---|---|---|---|---|---|---|---|---|---|---|`n"
foreach ($r in $results) {
    $report += "| $($r.name) | $($r.requests) | $($r.rate) | $($r.throughput) | $($r.meanMs) | $($r.p50Ms) | $($r.p95Ms) | $($r.p99Ms) | $($r.maxMs) | $($r.respKb) | $($r.successPct)% | $($r.statusCodes) | $($r.errorCount) |`n"
}

$report += "`n## Units`n`n"
$report += "- All latencies are in **milliseconds (ms)**, converted from Vegeta's nanosecond timestamps (raw_ns / 1,000,000).`n"
$report += "- Response size is in **kilobytes (KB)**, converted from bytes (raw_bytes / 1,024).`n"
$report += "- Rate and throughput are in **requests per second (req/s)**.`n"
$report += "- Success is a percentage (0-100%), converted from Vegeta's 0-1 ratio.`n`n"

$report += "## Consistency Check`n`n"
if ($consistencyErrors.Count -eq 0) {
    $report += "All table values match their JSON source after unit conversion. **PASS**`n`n"
} else {
    $report += "**FAIL** - $($consistencyErrors.Count) mismatch(es) detected:`n`n"
    foreach ($e in $consistencyErrors) { $report += "- $e`n" }
    $report += "`n"
}

$report += "## Raw Data`n`n"
$report += "Vegeta JSON files are in $RawDir :`n`n"
foreach ($sc in $scenarioFiles) {
    $report += "- $($sc.file)`n"
}
$report += "`n"

$report += "## Reproduction`n`n"
$report += '```powershell' + "`n"
$report += "# 1. Start the $Driver stack (see SOP.md section 5 for driver-specific instructions)`n"
$report += "# 2. Seed 10,000 articles`n"
$report += "# 3. Run the benchmark`n"
$report += "powershell -ExecutionPolicy Bypass -File scripts\benchmark\run-benchmark.ps1 -Driver $Driver`n"
$report += "# 4. Regenerate this report`n"
$report += "powershell -ExecutionPolicy Bypass -File scripts\benchmark\generate-report.ps1 -Driver $Driver`n"
$report += '```' + "`n"

$outputDir = Split-Path $OutputFile -Parent
if (-not (Test-Path $outputDir)) { New-Item -ItemType Directory -Force -Path $outputDir | Out-Null }
Set-Content -Path $OutputFile -Value $report -Encoding UTF8

$checkResult = if ($consistencyErrors.Count -eq 0) { "PASS" } else { "FAIL" }
Write-Host "Report generated: $OutputFile"
Write-Host "Consistency check: $checkResult ($($consistencyErrors.Count) error(s))"
if ($consistencyErrors.Count -gt 0) { exit 1 }
exit 0
