#!/usr/bin/env bash
# C3: Linux in-container benchmark runner.
#
# Runs the same four Vegeta scenarios as run-benchmark.ps1, but from inside a
# Linux container on the same Docker network as the ContentX app. This eliminates
# Windows client-side port exhaustion (the "Only one usage of each socket
# address" errors seen in the historical MySQL run) so any timeout/queueing
# observed is attributable to the app/database, not the load generator host.
#
# Produces the same JSON files (article-list.json, article-detail.json,
# graphql.json, concurrent-write.json, run-metadata.json, *.metrics.txt) that
# generate-report.ps1 consumes, so report generation stays on the host.
#
# Required env:
#   BASE_URL         e.g. http://app:8080  (internal Docker DNS)
#   ADMIN_PASSWORD   admin password for the ContentX instance
#   OUTPUT_DIR       mounted volume where JSON files are written (absolute, in-container)
#   GIT_SHA          short git SHA (captured on host, passed in via env)
#   GIT_BRANCH       git branch name
# Optional env (defaults match run-benchmark.ps1):
#   READ_RATE=1000  WRITE_RATE=100  READ_DURATION=15s  WRITE_DURATION=10s
#   COOLDOWN_SECONDS=5  METRICS_PATH=/metrics
#   DRIVER=mysql  (only used for the run-metadata.json record)
set -euo pipefail

: "${BASE_URL:?BASE_URL is required}"
: "${ADMIN_PASSWORD:?ADMIN_PASSWORD is required}"
: "${OUTPUT_DIR:?OUTPUT_DIR is required}"
: "${GIT_SHA:?GIT_SHA is required}"
: "${GIT_BRANCH:?GIT_BRANCH is required}"

READ_RATE="${READ_RATE:-1000}"
WRITE_RATE="${WRITE_RATE:-100}"
READ_DURATION="${READ_DURATION:-15s}"
WRITE_DURATION="${WRITE_DURATION:-10s}"
COOLDOWN_SECONDS="${COOLDOWN_SECONDS:-5}"
METRICS_PATH="${METRICS_PATH:-/metrics}"
DRIVER="${DRIVER:-mysql}"

mkdir -p "$OUTPUT_DIR"

echo "=== ContentX Linux benchmark: driver=$DRIVER base=$BASE_URL cooldown=${COOLDOWN_SECONDS}s ==="
vegeta -version || { echo "vegeta not found in PATH"; exit 1; }

# --- Login ---
LOGIN_RESP=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"admin\",\"password\":\"$ADMIN_PASSWORD\"}")
TOKEN=$(echo "$LOGIN_RESP" | jq -r '.data.token.access_token')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo "Login failed. Response: $LOGIN_RESP"
    exit 1
fi
AUTH_HEADER="Authorization: Bearer $TOKEN"

# --- Pick article + count ---
LIST_RESP=$(curl -s "$BASE_URL/api/v1/articles?page=1&page_size=1" -H "$AUTH_HEADER")
ARTICLE_ID=$(echo "$LIST_RESP" | jq -r '.data.items[0].id')
ARTICLE_COUNT=$(echo "$LIST_RESP" | jq -r '.data.total')
if [ -z "$ARTICLE_ID" ] || [ "$ARTICLE_ID" = "null" ]; then
    echo "No article found. Seed the $DRIVER database first."
    exit 1
fi
echo "Article id=$ARTICLE_ID count=$ARTICLE_COUNT"

# --- Response body sizes (one sample per scenario, for the metadata record) ---
LIST_BYTES=$(curl -s "$BASE_URL/api/v1/articles?page=1&page_size=20" -H "$AUTH_HEADER" | wc -c)
DETAIL_BYTES=$(curl -s "$BASE_URL/api/v1/articles/$ARTICLE_ID" -H "$AUTH_HEADER" | wc -c)
GRAPHQL_QUERY='{"query":"{ articles(page:1,pageSize:20){ total items{ id title slug excerpt } } }"}'
GRAPHQL_BYTES=$(printf '%s' "$GRAPHQL_QUERY" | curl -s -X POST "$BASE_URL/api/v1/graphql" \
    -H "$AUTH_HEADER" -H 'Content-Type: application/json' --data-binary @- | wc -c)

# --- Body files for POST/PUT scenarios ---
printf '%s' "$GRAPHQL_QUERY" > "$OUTPUT_DIR/graphql-body.json"
printf '%s' '{"title":"Concurrent benchmark update","content":"ContentX concurrent write benchmark payload","revision_note":"vegeta benchmark"}' > "$OUTPUT_DIR/write-body.json"

# --- Metrics snapshot helper (best-effort, Prometheus endpoint) ---
metrics_snapshot() {
    local label="$1"
    local line
    line="[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $label base=$BASE_URL"
    local m
    m=$(curl -s --max-time 3 "$BASE_URL$METRICS_PATH" || true)
    if [ -n "$m" ]; then
        local goroutines open_fds db_wait
        goroutines=$(echo "$m" | awk '/^go_goroutines /{print $2; exit}')
        open_fds=$(echo "$m" | awk '/^process_open_fds /{print $2; exit}')
        db_wait=$(echo "$m" | awk '/db_connections.*waiting|sql_db_connections_waiting/{print $2; exit}')
        line="$line goroutines=${goroutines:-n/a} open_fds=${open_fds:-n/a}"
        [ -n "$db_wait" ] && line="$line db_waiting=$db_wait"
    else
        line="$line metrics=unavailable"
    fi
    echo "$line"
}

run_case() {
    local name="$1" method="$2" url="$3" rate="$4" duration="$5" body_path="${6:-}"
    local pre post
    pre=$(metrics_snapshot "$name pre")
    echo "$pre"

    local result_path="$OUTPUT_DIR/$name.bin"
    local json_path="$OUTPUT_DIR/$name.json"
    local target="$method $url"
    local attack_args=(-rate="$rate" -duration="$duration" -header="$AUTH_HEADER" -output="$result_path")
    if [ -n "$body_path" ]; then
        attack_args+=(-header="Content-Type: application/json" -body="$body_path")
    fi
    printf '%s' "$target" | vegeta attack "${attack_args[@]}"
    vegeta report -type=json "$result_path" > "$json_path"
    vegeta report "$result_path"

    post=$(metrics_snapshot "$name post")
    echo "$post"
    { echo "$pre"; echo "$post"; } > "$OUTPUT_DIR/$name.metrics.txt"
}

# --- Capture run metadata (C1) ---
# Build the metadata JSON. Note: app_config.goroutines is sampled pre-run.
APP_GOROUTINES=$(curl -s --max-time 3 "$BASE_URL$METRICS_PATH" | awk '/^go_goroutines /{print $2; exit}' || echo "")
META_JSON=$(jq -n \
    --arg timestamp "$(date +%Y-%m-%dT%H:%M:%S%z)" \
    --arg git_sha "$GIT_SHA" \
    --arg git_branch "$GIT_BRANCH" \
    --arg driver "$DRIVER" \
    --arg base_url "$BASE_URL" \
    --argjson article_count "$ARTICLE_COUNT" \
    --argjson article_id "$ARTICLE_ID" \
    --argjson read_rate "$READ_RATE" \
    --argjson write_rate "$WRITE_RATE" \
    --arg read_duration "$READ_DURATION" \
    --arg write_duration "$WRITE_DURATION" \
    --argjson cooldown_seconds "$COOLDOWN_SECONDS" \
    --argjson list_bytes "$LIST_BYTES" \
    --argjson detail_bytes "$DETAIL_BYTES" \
    --argjson graphql_bytes "$GRAPHQL_BYTES" \
    --arg app_goroutines "${APP_GOROUTINES:-n/a}" \
    '{
        timestamp: $timestamp,
        git_sha: $git_sha,
        git_branch: $git_branch,
        git_dirty: false,
        runner: "linux-container",
        driver: $driver,
        base_url: $base_url,
        article_count: $article_count,
        article_id: $article_id,
        read_rate: $read_rate,
        write_rate: $write_rate,
        read_duration: $read_duration,
        write_duration: $write_duration,
        cooldown_seconds: $cooldown_seconds,
        scenarios: [
            {name:"article-list",     method:"GET", url:"/api/v1/articles?page=1&page_size=20", rate:$read_rate, duration:$read_duration, response_bytes:$list_bytes},
            {name:"article-detail",   method:"GET", url:("/api/v1/articles/" + ($article_id|tostring)), rate:$read_rate, duration:$read_duration, response_bytes:$detail_bytes},
            {name:"graphql",          method:"POST", url:"/api/v1/graphql", rate:$read_rate, duration:$read_duration, response_bytes:$graphql_bytes},
            {name:"concurrent-write", method:"PUT", url:("/api/v1/articles/" + ($article_id|tostring)), rate:$write_rate, duration:$write_duration, response_bytes:0}
        ],
        app_config: {driver:$driver, goroutines:$app_goroutines}
    }')
echo "$META_JSON" > "$OUTPUT_DIR/run-metadata.json"
echo "Run metadata: git_sha=$GIT_SHA article_count=$ARTICLE_COUNT driver=$DRIVER -> $OUTPUT_DIR/run-metadata.json"

# --- Run scenarios (same order/rates as run-benchmark.ps1) ---
run_case "article-list"   "GET" "$BASE_URL/api/v1/articles?page=1&page_size=20" "$READ_RATE" "$READ_DURATION"
sleep "$COOLDOWN_SECONDS"
run_case "article-detail" "GET" "$BASE_URL/api/v1/articles/$ARTICLE_ID"        "$READ_RATE" "$READ_DURATION"
sleep "$COOLDOWN_SECONDS"
run_case "graphql"        "POST" "$BASE_URL/api/v1/graphql"                     "$READ_RATE" "$READ_DURATION" "$OUTPUT_DIR/graphql-body.json"
sleep "$COOLDOWN_SECONDS"
run_case "concurrent-write" "PUT" "$BASE_URL/api/v1/articles/$ARTICLE_ID"       "$WRITE_RATE" "$WRITE_DURATION" "$OUTPUT_DIR/write-body.json"

echo "Raw benchmark reports written to $OUTPUT_DIR"
