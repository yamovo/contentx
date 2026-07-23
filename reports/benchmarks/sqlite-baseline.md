# SQLite Benchmark Baseline

- **Date**: 2026-07-23T13:12:48+0000
- **Git SHA**: 0f5d624
- **Branch**: main
- **Driver**: sqlite
- **Article count**: 10000
- **Base URL**: http://app:8080
- **Rates**: read=1000 req/s, write=100 req/s
- **Durations**: read=15s, write=10s
- **Cooldown**: 5s between scenarios

## Results

| Scenario | Requests | Rate (req/s) | Throughput (req/s) | Mean (ms) | P50 (ms) | P95 (ms) | P99 (ms) | Max (ms) | Resp (KB) | Success | Status Codes | Errors |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| article-list | 15000 | 1000.1 | 733.9 | 3792.65 | 3557.849 | 7692.494 | 9310.831 | 14084.089 | 27.5 | 100% | 200=15000 | 0 |
| article-detail | 15000 | 1000.1 | 1000.0 | 0.913 | 0.851 | 1.311 | 1.767 | 4.246 | 3.6 | 100% | 200=15000 | 0 |
| graphql | 15000 | 1000.0 | 267.8 | 24439.704 | 26560.965 | 30001.488 | 30003.888 | 30013.507 | 1.6 | 72.00% | 0=4200, 200=10800 | 1 |
| concurrent-write | 1000 | 100.1 | 100.1 | 2.338 | 1.986 | 3.121 | 13.381 | 19.947 | 1.5 | 100% | 200=1000 | 0 |

## Units

- All latencies are in **milliseconds (ms)**, converted from Vegeta's nanosecond timestamps (raw_ns / 1,000,000).
- Response size is in **kilobytes (KB)**, converted from bytes (raw_bytes / 1,024).
- Rate and throughput are in **requests per second (req/s)**.
- Success is a percentage (0-100%), converted from Vegeta's 0-1 ratio.

## Consistency Check

All table values match their JSON source after unit conversion. **PASS**

## Raw Data

Vegeta JSON files are in D:\Obsidian\03_Projects\contentx-main\reports\benchmarks\raw\sqlite :

- article-list.json
- article-detail.json
- graphql.json
- concurrent-write.json

## Reproduction

```powershell
# 1. Start the sqlite stack (see SOP.md section 5 for driver-specific instructions)
# 2. Seed 10,000 articles
# 3. Run the benchmark
powershell -ExecutionPolicy Bypass -File scripts\benchmark\run-benchmark.ps1 -Driver sqlite
# 4. Regenerate this report
powershell -ExecutionPolicy Bypass -File scripts\benchmark\generate-report.ps1 -Driver sqlite
```

