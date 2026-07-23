# MySQL Benchmark Baseline

- **Date**: 2026-07-23T12:41:03+0000
- **Git SHA**: 0f5d624
- **Branch**: main
- **Driver**: mysql
- **Article count**: 10000
- **Base URL**: http://app:8080
- **Rates**: read=1000 req/s, write=100 req/s
- **Durations**: read=15s, write=10s
- **Cooldown**: 5s between scenarios

## Results

| Scenario | Requests | Rate (req/s) | Throughput (req/s) | Mean (ms) | P50 (ms) | P95 (ms) | P99 (ms) | Max (ms) | Resp (KB) | Success | Status Codes | Errors |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| article-list | 15000 | 1000.1 | 351.0 | 17175.378 | 18664.101 | 29981.309 | 30000.951 | 30014.797 | 25.9 | 94.70% | 0=795, 200=14205 | 1 |
| article-detail | 15000 | 1000.0 | 999.9 | 2.519 | 2.44 | 3.112 | 4.391 | 8.35 | 1.4 | 100% | 200=15000 | 0 |
| graphql | 15000 | 1000.0 | 217.4 | 24008.388 | 28045.165 | 30001.013 | 30002.497 | 30009.9 | 1.5 | 62.77% | 0=5584, 200=9416 | 1 |
| concurrent-write | 1000 | 100.1 | 98.0 | 161.146 | 149.203 | 286.882 | 339.656 | 394.756 | 1.5 | 100% | 200=1000 | 0 |

## Units

- All latencies are in **milliseconds (ms)**, converted from Vegeta's nanosecond timestamps (raw_ns / 1,000,000).
- Response size is in **kilobytes (KB)**, converted from bytes (raw_bytes / 1,024).
- Rate and throughput are in **requests per second (req/s)**.
- Success is a percentage (0-100%), converted from Vegeta's 0-1 ratio.

## Consistency Check

All table values match their JSON source after unit conversion. **PASS**

## Raw Data

Vegeta JSON files are in D:\Obsidian\03_Projects\contentx-main\reports\benchmarks\raw\mysql :

- article-list.json
- article-detail.json
- graphql.json
- concurrent-write.json

## Reproduction

```powershell
# 1. Start the mysql stack (see SOP.md section 5 for driver-specific instructions)
# 2. Seed 10,000 articles
# 3. Run the benchmark
powershell -ExecutionPolicy Bypass -File scripts\benchmark\run-benchmark.ps1 -Driver mysql
# 4. Regenerate this report
powershell -ExecutionPolicy Bypass -File scripts\benchmark\generate-report.ps1 -Driver mysql
```

