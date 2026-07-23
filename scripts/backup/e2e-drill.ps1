# B5 end-to-end drill: real PostgreSQL container -> backup -> drop DB -> restore -> data consistent
#
# Two scenarios are exercised, satisfying all Round-2 exit gates:
#   Scenario A: API restore over a working DB (proves the restore endpoint).
#               Business tables are TRUNCATEd (auth tables kept intact so the
#               admin token still validates), then the restore endpoint is
#               called. Row counts must return to baseline.
#   Scenario B: Total-loss recovery via direct psql (the realistic disaster
#               path). DROP SCHEMA wipes everything including auth, so the
#               restore endpoint CANNOT be used (auth middleware queries the
#               users table). Recovery is performed by piping the backup file
#               through psql inside the app container. Row counts must return
#               to baseline.
#
# Prerequisites:
#   - Docker Desktop running
#   - contentx stack up via `docker compose up -d` (app on $BaseUrl, db container $DbContainer)
#   - app image bundles postgresql-client (Dockerfile: apk add postgresql-client)
#   - .env has ADMIN_PASSWORD / POSTGRES_USER / POSTGRES_DB / POSTGRES_PASSWORD
#
# Records Git SHA + commands + results to a report file. Exit 0 = pass, 1 = fail.

param(
    [string]$BaseUrl = "http://127.0.0.1:18080",
    [string]$DbContainer = "contentx-db",
    [string]$AppContainer = "contentx",
    [string]$ReportDir = "reports\backup"
)

$ErrorActionPreference = "Stop"
$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$reportPath = Join-Path $repoRoot $ReportDir
New-Item -ItemType Directory -Force -Path $reportPath | Out-Null
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$reportFile = Join-Path $reportPath "e2e-drill-$stamp.md"
$scratch = Join-Path $reportPath "raw-$stamp"
New-Item -ItemType Directory -Force -Path $scratch | Out-Null

function Log([string]$msg) {
    Write-Output "[$(Get-Date -Format 'HH:mm:ss')] $msg"
}
function LogCmd([string]$cmd) { Log "RUN: $cmd" }

# Read .env
$envFile = Join-Path $repoRoot ".env"
function Get-EnvVar([string]$key) {
    $line = Get-Content $envFile | Where-Object { $_ -like "$key=*" } | Select-Object -First 1
    if (-not $line) { throw "$key missing from .env" }
    return $line.Substring("$key=".Length)
}
$adminPassword = Get-EnvVar "ADMIN_PASSWORD"
$pgUser = Get-EnvVar "POSTGRES_USER"
$pgDb = Get-EnvVar "POSTGRES_DB"

# Git SHA
$gitSha = (git -C $repoRoot rev-parse --short HEAD).Trim()
$gitDirty = (git -C $repoRoot status --porcelain).Trim()
$dirtyNote = if ($gitDirty) { "dirty (uncommitted working tree)" } else { "clean" }
Log "Git SHA: $gitSha ($dirtyNote)"

# psql helper: run SQL in db container, return raw output.
# Native commands (docker/psql) write NOTICEs to stderr; under
# ErrorActionPreference=Stop PS5.1 treats those as terminating, so we relax
# the preference locally and rely on $LASTEXITCODE for real failures.
function Invoke-Psql {
    param([string]$sql, [switch]$Tainted)
    $a = @("exec", $DbContainer, "psql", "-U", $pgUser, "-d", $pgDb, "-t", "-A", "-F", "`t", "-c", $sql)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $out = & docker @a 2>&1
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prev
    if (-not $Tainted -and $code -ne 0) { throw "psql failed: $out" }
    return $out
}

# psql helper against the app container (uses app's env for credentials).
# Used for Scenario B where the DB schema has been dropped and the restore
# endpoint cannot be used (auth middleware cannot query the users table).
function Invoke-PsqlInApp {
    param([string]$file)
    $sh = "PGPASSWORD=`$DB_PASSWORD psql -h `$DB_HOST -p `$DB_PORT -U `$DB_USER -d `$DB_NAME -f $file"
    $a = @("exec", $AppContainer, "sh", "-c", $sh)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $out = & docker @a 2>&1
    $code = $LASTEXITCODE
    $ErrorActionPreference = $prev
    if ($code -ne 0) { throw "psql in app failed: $out" }
    return $out
}

# Row-count snapshot: returns hashtable of all public tables
function Get-RowCounts() {
    $sql = "SELECT COALESCE(json_object_agg(tablename, cnt), '{}'::json) FROM (SELECT tablename, (xpath('/row/c/text()', query_to_xml(format('SELECT count(*) AS c FROM public.%I', tablename), false, true, '')))[1]::text::int AS cnt FROM pg_tables WHERE schemaname='public') sub;"
    $raw = Invoke-Psql $sql
    $raw = ($raw | Where-Object { $_ -match '^\{' }) -join "`n"
    $obj = $raw | ConvertFrom-Json
    $ht = @{}
    foreach ($p in $obj.PSObject.Properties) { $ht[$p.Name] = [int]$p.Value }
    return $ht
}

# Compare two row-count hashtables; returns array of regression strings.
# activity_logs is excluded from strict comparison: the ActivityLogger middleware
# appends a row for every API call, so the drill itself (backup, restore, health
# checks) naturally grows this table. It is operational metadata, not business
# data, so a small positive delta is expected and not a restore regression.
function Compare-RowCounts($baseline, $after) {
    $skipTables = @('activity_logs')
    $regs = New-Object System.Collections.ArrayList
    foreach ($t in ($baseline.Keys | Sort-Object)) {
        if ($skipTables -contains $t) { continue }
        $b = $baseline[$t]
        if (-not $after.ContainsKey($t)) {
            [void]$regs.Add("table $t missing after restore (baseline=$b)")
        } else {
            $a = $after[$t]
            if ($a -ne $b) {
                [void]$regs.Add("table $t count mismatch: baseline=$b after=$a")
            }
        }
    }
    foreach ($t in ($after.Keys | Sort-Object)) {
        if ($skipTables -contains $t) { continue }
        if (-not $baseline.ContainsKey($t)) {
            [void]$regs.Add("table $t appeared after restore (after=$($after[$t]))")
        }
    }
    return ,$regs
}

# HTTP helpers
function Login() {
    $body = @{ username = "admin"; password = $adminPassword } | ConvertTo-Json -Compress
    $resp = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/v1/auth/login" -ContentType "application/json" -Body $body
    return $resp.data.token.access_token
}
function ApiPost {
    param([string]$token, [string]$path, [string]$query = "")
    $uri = "$BaseUrl$path"
    if ($query) { $uri = $uri + "?" + $query }
    Log "ApiPost uri=$uri"
    return Invoke-RestMethod -Method Post -Uri $uri -Headers @{ Authorization = "Bearer $token" }
}
function ApiGet {
    param([string]$token, [string]$path)
    $uri = "$BaseUrl$path"
    Log "ApiGet uri=$uri"
    return Invoke-RestMethod -Method Get -Uri $uri -Headers @{ Authorization = "Bearer $token" }
}

# --- Drill start ---
Log "=== B5 end-to-end drill start ==="
Log "BaseUrl=$BaseUrl  DbContainer=$DbContainer  AppContainer=$AppContainer"
Log "PG_USER=$pgUser  PG_DB=$pgDb"

# ============================================================
# Shared step: admin login + baseline + backup
# ============================================================
Log "`n--- Shared: login + baseline + backup ---"
LogCmd "POST /api/v1/auth/login (admin)"
$token = Login
Log "Login OK, token length=$($token.Length)"

Log "Capture baseline row counts (all public tables)"
$baseline = Get-RowCounts
Log "Baseline table count: $($baseline.Count)"
foreach ($k in ($baseline.Keys | Sort-Object)) { Log "  $k = $($baseline[$k])" }

LogCmd "POST /api/v1/admin/backup?type=db"
$bk = ApiPost $token "/api/v1/admin/backup" "type=db"
$bkFile = $bk.data.path
if (-not $bkFile) { throw "backup response missing path: $($bk | ConvertTo-Json -Depth 5)" }
Log "Backup file: $bkFile"

# Validate backup content
LogCmd "GET /api/v1/admin/backup/$bkFile/download"
$dlUri = "$BaseUrl/api/v1/admin/backup/$bkFile/download"
$dlFile = Join-Path $scratch $bkFile
Invoke-WebRequest -Uri $dlUri -Headers @{ Authorization = "Bearer $token" } -OutFile $dlFile
$dlSize = (Get-Item $dlFile).Length
$dlContent = Get-Content $dlFile -Raw
$hasCreateTable = $dlContent -match 'CREATE TABLE'
$hasSchemaMigrations = $dlContent -match 'schema_migrations'
$hasCopyArticles = $dlContent -match 'COPY.*articles' -or $dlContent -match 'INSERT INTO.*articles'
Log "Backup size: $dlSize bytes; CREATE TABLE: $hasCreateTable; schema_migrations: $hasSchemaMigrations; articles data: $hasCopyArticles"
if (-not ($hasCreateTable -and $hasSchemaMigrations)) { throw "backup file incomplete: missing CREATE TABLE or schema_migrations" }

# ============================================================
# Scenario A: API restore over a working DB
#   Truncate business tables (keep auth tables) -> restore via API -> verify
# ============================================================
Log "`n--- Scenario A: API restore over working DB ---"

# Truncate all business tables except auth-critical ones. The restore endpoint
# uses pg_dump --clean --if-exists, so it will DROP+CREATE every table (including
# auth tables) and restore all data. Auth tables are kept intact BEFORE the
# restore call so the admin token still validates at the auth middleware.
$authTables = @('users','roles','permissions','role_permissions','schema_migrations')
$truncateSql = @'
DO $$ DECLARE t text; BEGIN FOR t IN SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename NOT IN ('users','roles','permissions','role_permissions','schema_migrations') LOOP EXECUTE 'TRUNCATE TABLE public.' || quote_ident(t) || ' CASCADE'; END LOOP; END $$;
'@
LogCmd "TRUNCATE all business tables (keep auth tables) via psql in $DbContainer"
Invoke-Psql $truncateSql | Out-Null

# Confirm auth still works and business data is empty
LogCmd "GET /api/v1/articles?page=1&page_size=1 (expect total=0 after truncate)"
$preRestoreList = ApiGet $token "/api/v1/articles?page=1&page_size=1"
$preRestoreTotal = $preRestoreList.data.total
Log "After truncate: articles total=$preRestoreTotal (expect 0)"
if ($preRestoreTotal -ne 0) { Log "WARN: expected 0 articles after truncate, got $preRestoreTotal" }

# Capture pre-restore row counts (business tables should be 0, auth tables intact)
$preRestoreCounts = Get-RowCounts
$preRestoreArticles = $preRestoreCounts['articles']
Log "Pre-restore articles count via psql: $preRestoreArticles (expect 0)"

# Restore via API endpoint
LogCmd "POST /api/v1/admin/backup/$bkFile/restore"
$restoreResp = ApiPost $token "/api/v1/admin/backup/$bkFile/restore"
$restoreJson = $restoreResp | ConvertTo-Json -Depth 6 -Compress
Log "Restore response: $restoreJson"
if ($restoreResp.data.warning) { Log "WARNING: $($restoreResp.data.warning) -- $($restoreResp.data.details)" }

# Capture post-restore row counts
Log "Capture post-restore row counts (Scenario A)"
$afterA = Get-RowCounts
Log "Post-restore table count: $($afterA.Count)"

# Consistency check
Log "Consistency check (Scenario A): baseline vs post-restore"
$regsA = Compare-RowCounts $baseline $afterA
$skipTables = @('activity_logs')
foreach ($t in ($baseline.Keys | Sort-Object)) {
    $b = $baseline[$t]
    $a = if ($afterA.ContainsKey($t)) { $afterA[$t] } else { "MISSING" }
    if ($skipTables -contains $t) {
        Log "  SKIP $t : baseline=$b  after=$a (drill-generated writes)"
        continue
    }
    $status = if ($a -eq $b) { "OK" } else { "FAIL" }
    Log "  $status $t : baseline=$b  after=$a"
}

# Functional check
LogCmd "GET /api/v1/articles?page=1&page_size=1 (expect total=10000 after restore)"
$listRespA = ApiGet $token "/api/v1/articles?page=1&page_size=1"
$listTotalA = $listRespA.data.total
Log "Scenario A functional check: articles total=$listTotalA"
$scenarioAPass = ($regsA.Count -eq 0) -and ($listTotalA -ge 1)

$scenarioAResult = if ($scenarioAPass) { "PASS" } else { "FAIL" }
Log "=== Scenario A conclusion: $scenarioAResult ==="

# ============================================================
# Scenario B: Total-loss recovery via direct psql
#   DROP SCHEMA -> direct psql restore from app container -> verify
# ============================================================
Log "`n--- Scenario B: Total-loss recovery via direct psql ---"

# Drop entire public schema (simulates catastrophic DB loss)
LogCmd "DROP SCHEMA public CASCADE; CREATE SCHEMA public; (via psql in $DbContainer)"
Invoke-Psql "DROP SCHEMA public CASCADE; CREATE SCHEMA public; GRANT ALL ON SCHEMA public TO $pgUser; GRANT ALL ON SCHEMA public TO public;" | Out-Null
Log "public schema dropped and recreated (simulating total DB loss)"

# Confirm DB empty
$emptyCheck = Invoke-Psql "SELECT COUNT(*) FROM pg_tables WHERE schemaname='public';" -Tainted
Log "public table count after drop: $emptyCheck"

# Attempt API restore (expected to fail with 401 because users table is gone)
# This documents the auth-DB circular dependency constraint.
LogCmd "POST /api/v1/admin/backup/$bkFile/restore (expect 401 - auth table gone)"
$apiRestoreFailed = $false
$apiRestoreErr = ""
try {
    $ignored = ApiPost $token "/api/v1/admin/backup/$bkFile/restore"
    Log "UNEXPECTED: API restore succeeded after DROP SCHEMA (token may still be cached)"
} catch {
    $apiRestoreFailed = $true
    $apiRestoreErr = $_.Exception.Message
    Log "Expected failure: API restore rejected after total DB loss: $apiRestoreErr"
}

# Restore via direct psql from the app container (the realistic disaster path)
$bkPathInApp = "/app/backups/$bkFile"
LogCmd "docker exec $AppContainer sh -c 'PGPASSWORD=`$DB_PASSWORD psql -h `$DB_HOST -U `$DB_USER -d `$DB_NAME -f $bkPathInApp'"
$psqlOut = Invoke-PsqlInApp $bkPathInApp
Log "psql restore completed (output truncated): $($psqlOut | Select-Object -Last 3)"

# Capture post-restore row counts
Log "Capture post-restore row counts (Scenario B)"
$afterB = Get-RowCounts
Log "Post-restore table count: $($afterB.Count)"

# Consistency check
Log "Consistency check (Scenario B): baseline vs post-restore"
$regsB = Compare-RowCounts $baseline $afterB
foreach ($t in ($baseline.Keys | Sort-Object)) {
    $b = $baseline[$t]
    $a = if ($afterB.ContainsKey($t)) { $afterB[$t] } else { "MISSING" }
    if ($skipTables -contains $t) {
        Log "  SKIP $t : baseline=$b  after=$a (drill-generated writes)"
        continue
    }
    $status = if ($a -eq $b) { "OK" } else { "FAIL" }
    Log "  $status $t : baseline=$b  after=$a"
}

# Functional check: login must work again (users table restored)
LogCmd "POST /api/v1/auth/login (expect OK after psql restore)"
$scenarioBFuncLogin = $false
try {
    $tokenB = Login
    if ($tokenB) { $scenarioBFuncLogin = $true; Log "Login OK after psql restore" }
} catch {
    Log "Login FAILED after psql restore: $($_.Exception.Message)"
}

$listTotalB = -1
if ($scenarioBFuncLogin) {
    LogCmd "GET /api/v1/articles?page=1&page_size=1 (expect total=10000)"
    $listRespB = ApiGet $tokenB "/api/v1/articles?page=1&page_size=1"
    $listTotalB = $listRespB.data.total
    Log "Scenario B functional check: articles total=$listTotalB"
}
$scenarioBPass = ($regsB.Count -eq 0) -and $scenarioBFuncLogin -and ($listTotalB -ge 1)

$scenarioBResult = if ($scenarioBPass) { "PASS" } else { "FAIL" }
Log "=== Scenario B conclusion: $scenarioBResult ==="

# ============================================================
# Overall conclusion
# ============================================================
$drillPass = $scenarioAPass -and $scenarioBPass
$conclusion = if ($drillPass) { "PASS" } else { "FAIL" }
Log "`n=== Drill conclusion: $conclusion ==="
Log "Scenario A (API restore over working DB): $scenarioAResult"
Log "Scenario B (direct psql after total loss): $scenarioBResult"

# --- Write report ---
$baselineRows = ($baseline.Keys | Sort-Object | ForEach-Object { "| $_ | $($baseline[$_]) |" }) -join "`n"
$afterARows = ($afterA.Keys | Sort-Object | ForEach-Object { "| $_ | $($afterA[$_]) |" }) -join "`n"
$afterBRows = ($afterB.Keys | Sort-Object | ForEach-Object { "| $_ | $($afterB[$_]) |" }) -join "`n"
$regressionALines = ($regsA -join "`n")
$regressionBLines = ($regsB -join "`n")

$report = "# B5 End-to-End Drill Report`n`n"
$report += "- **Date**: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss zzz')`n"
$report += "- **Git SHA**: $gitSha`n"
$report += "- **Working tree**: $dirtyNote`n"
$report += "- **Environment**: Docker Desktop (postgres:16-alpine container $DbContainer), app $BaseUrl`n"
$report += "- **Backup file**: $bkFile ($dlSize bytes)`n"
$report += "- **Backup content**: CREATE TABLE=$hasCreateTable, schema_migrations=$hasSchemaMigrations, articles data=$hasCopyArticles`n"
$report += "- **Conclusion**: **$conclusion**`n"
$report += "  - Scenario A (API restore over working DB): **$scenarioAResult**`n"
$report += "  - Scenario B (direct psql after total loss): **$scenarioBResult**`n`n"

$report += "## Design constraint: auth-DB circular dependency`n`n"
$report += "The restore endpoint POST /api/v1/admin/backup/:file/restore requires admin authentication. The auth middleware (internal/middleware/auth.go) queries the users table (with Preload Role and Preload Role.Permissions) on every request. If the entire database is lost (DROP SCHEMA), the users table is gone and the endpoint returns 401 User not found -- so the restore endpoint **cannot** be used for total-DB-loss recovery.`n`n"
$report += "Two scenarios cover both the endpoint proof and the realistic disaster path:`n"
$report += "- **Scenario A** keeps auth tables intact (TRUNCATE business tables only) so the admin token still validates, then calls the restore endpoint. This proves the endpoint works.`n"
$report += "- **Scenario B** drops the entire schema (total loss) and restores via direct `psql -f` from the app container. This is the realistic disaster-recovery path operators must use.`n`n"
$report += "**Note on activity_logs**: the activity_logs table is excluded from strict row-count comparison. The ActivityLogger middleware appends a row for every API call, so the drill itself (backup, restore, functional checks) naturally grows this table. It is operational metadata, not business data; a small positive delta is expected and not a restore regression.`n`n"

$report += "## Scenario A: API restore over working DB`n`n"
$report += "1. **Login** POST /api/v1/auth/login -> OK`n"
$report += "2. **Baseline** -> $($baseline.Count) tables (articles=$($baseline['articles']))`n"
$report += "3. **Backup** POST /api/v1/admin/backup?type=db -> $bkFile ($dlSize bytes)`n"
$report += "4. **Truncate business tables** (keep users/roles/permissions/role_permissions/schema_migrations) -> articles=0`n"
$report += "5. **Auth check** GET /api/v1/articles -> total=$preRestoreTotal (expect 0)`n"
$report += "6. **Restore** POST /api/v1/admin/backup/$bkFile/restore -> $restoreJson`n"
$report += "7. **Post-restore** -> $($afterA.Count) tables (articles=$($afterA['articles']))`n"
$report += "8. **Consistency** -> $($regsA.Count) regression(s)`n"
$report += "9. **Functional** GET /api/v1/articles -> total=$listTotalA`n"
$report += "10. **Result**: **$scenarioAResult**`n`n"
$report += "Regressions (empty = none):`n"
$report += '```' + "`n$regressionALines`n" + '```' + "`n`n"

$report += "## Scenario B: Total-loss recovery via direct psql`n`n"
$report += "1. **Reuse backup** $bkFile from Scenario A`n"
$report += "2. **DROP SCHEMA** public CASCADE; CREATE SCHEMA public; -> table count after drop: $emptyCheck`n"
$report += "3. **API restore attempt** (expected 401): failed=$apiRestoreFailed; error=$apiRestoreErr`n"
$report += "4. **Direct psql restore** from app container: docker exec $AppContainer sh -c 'PGPASSWORD=`$DB_PASSWORD psql -h `$DB_HOST -U `$DB_USER -d `$DB_NAME -f $bkPathInApp' -> OK`n"
$report += "5. **Post-restore** -> $($afterB.Count) tables (articles=$($afterB['articles']))`n"
$report += "6. **Consistency** -> $($regsB.Count) regression(s)`n"
$report += "7. **Functional** login=$scenarioBFuncLogin, GET /api/v1/articles -> total=$listTotalB`n"
$report += "8. **Result**: **$scenarioBResult**`n`n"
$report += "Regressions (empty = none):`n"
$report += '```' + "`n$regressionBLines`n" + '```' + "`n`n"

$report += "## Baseline Row Counts`n`n| table | rows |`n|---|---|`n$baselineRows`n`n"
$report += "## Scenario A Post-Restore Row Counts`n`n| table | rows |`n|---|---|`n$afterARows`n`n"
$report += "## Scenario B Post-Restore Row Counts`n`n| table | rows |`n|---|---|`n$afterBRows`n`n"

$report += "## Reproduction Commands`n`n"
$report += '```powershell' + "`n"
$report += "# 0. Start stack (app image must include postgresql-client)`n"
$report += "docker compose up -d`n`n"
$report += "# Scenario A: API restore over working DB`n"
$report += '# 1. Login' + "`n"
$report += '$resp = Invoke-RestMethod -Method Post -Uri ' + "'$BaseUrl/api/v1/auth/login'" + " `n"
$report += "  -ContentType 'application/json' -Body '{""username"":""admin"",""password"":""<ADMIN_PASSWORD>""}'`n"
$report += '$token = $resp.data.token.access_token' + "`n`n"
$report += "# 2. Backup`n"
$report += '$bk = Invoke-RestMethod -Method Post -Uri ' + "'$BaseUrl/api/v1/admin/backup?type=db'" + " `n"
$report += '  -Headers @{ Authorization = ''Bearer '' + $token }' + "`n"
$report += '$bkFile = $bk.data.path' + "`n`n"
$report += "# 3. Truncate business tables (keep auth tables)`n"
$truncateCmdLine = "docker exec $DbContainer psql -U $pgUser -d $pgDb -c " + '"DO $$ DECLARE t text; BEGIN FOR t IN SELECT tablename FROM pg_tables WHERE schemaname=''public'' AND tablename NOT IN (''users'',''roles'',''permissions'',''role_permissions'',''schema_migrations'') LOOP EXECUTE ''TRUNCATE TABLE public.'' || quote_ident(t) || '' CASCADE''; END LOOP; END $$;"'
$report += $truncateCmdLine + "`n`n"
$report += "# 4. Restore via API`n"
$report += 'Invoke-RestMethod -Method Post -Uri ' + "'$BaseUrl/api/v1/admin/backup/$bkFile/restore'" + " `n"
$report += '  -Headers @{ Authorization = ''Bearer '' + $token }' + "`n`n"
$report += "# Scenario B: Total-loss recovery via direct psql`n"
$report += "# 5. Drop entire schema`n"
$report += "docker exec $DbContainer psql -U $pgUser -d $pgDb -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;'`n`n"
$report += "# 6. Restore via direct psql from app container (auth endpoint is unusable)`n"
$report += "docker exec $AppContainer sh -c 'PGPASSWORD=`$DB_PASSWORD psql -h `$DB_HOST -U `$DB_USER -d `$DB_NAME -f /app/backups/$bkFile'`n`n"
$report += "# 7. Verify`n"
$report += "docker exec $DbContainer psql -U $pgUser -d $pgDb -c 'SELECT count(*) FROM articles;'`n"
$report += '```' + "`n"

Set-Content -Path $reportFile -Value $report -Encoding UTF8
Log "Report written: $reportFile"

# Keep a copy of the raw backup file
Copy-Item $dlFile -Destination (Join-Path $reportPath "$stamp-$bkFile") -Force

if (-not $drillPass) { exit 1 }
exit 0
