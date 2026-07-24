# PostgreSQL Benchmark Baseline

- **Date**: 2026-07-23T12:52:47+0000
- **Git SHA**: 0f5d624
- **Branch**: main
- **Driver**: postgres
- **Article count**: 10000
- **Base URL**: http://contentx:8080
- **Rates**: read=1000 req/s, write=100 req/s
- **Durations**: read=15s, write=10s
- **Cooldown**: 5s between scenarios

## Results

| Scenario | Requests | Rate (req/s) | Throughput (req/s) | Mean (ms) | P50 (ms) | P95 (ms) | P99 (ms) | Max (ms) | Resp (KB) | Success | Status Codes | Errors |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| article-list | 15000 | 1000.0 | 999.5 | 9.041 | 8.293 | 13.354 | 21.754 | 51.933 | 27.9 | 100% | 200=15000 | 0 |
| article-detail | 15000 | 1000.0 | 1000.0 | 1.574 | 1.524 | 1.984 | 2.587 | 4.057 | 3.6 | 100% | 200=15000 | 0 |
| graphql | 15000 | 1000.1 | 999.5 | 8.739 | 8.239 | 11.219 | 18.698 | 44.937 | 2.4 | 100% | 200=15000 | 0 |
| concurrent-write | 1000 | 100.1 | 100.0 | 7.821 | 7.48 | 9.699 | 12.489 | 32.637 | 1.5 | 100% | 200=1000 | 0 |

## Units

- All latencies are in **milliseconds (ms)**, converted from Vegeta's nanosecond timestamps (raw_ns / 1,000,000).
- Response size is in **kilobytes (KB)**, converted from bytes (raw_bytes / 1,024).
- Rate and throughput are in **requests per second (req/s)**.
- Success is a percentage (0-100%), converted from Vegeta's 0-1 ratio.

## Consistency Check

All table values match their JSON source after unit conversion. **PASS**

## Raw Data

Vegeta JSON files are in `reports/benchmarks/raw/postgres/` :

- article-list.json
- article-detail.json
- graphql.json
- concurrent-write.json

## Reproduction

```powershell
# 1. Start the postgres stack (see SOP.md section 5 for driver-specific instructions)
# 2. Seed 10,000 articles
# 3. Run the benchmark
powershell -ExecutionPolicy Bypass -File scripts\benchmark\run-benchmark.ps1 -Driver postgres
# 4. Regenerate this report
powershell -ExecutionPolicy Bypass -File scripts\benchmark\generate-report.ps1 -Driver postgres
```

