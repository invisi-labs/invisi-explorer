# Settlement replay and Explorer catch-up — 2026-11-24

RPC now batches native full-block, chain-ID and code reads in lazy groups of at most four blocks / twelve native calls. Each block retains its canonical identity, chain ID and code-hash checks; receipts and fresh terminal anchors stay on the existing path. Native transaction/withdrawal roots are verified, including preserving verified transactions when loading uncle bodies. Explicit unsupported/oversized batch transport falls back to ordinary calls; malformed or missing results fail closed.

Explorer prefetches up to four complete finalized 128-block windows and projects them in order. A single writer transaction checks the saved boundary and commits each complete window's operations, public views and checkpoint atomically. Bootstrap, near-head scans and durable dense-page cursors retain sequential scanning. A 30-second prefetch-only deadline falls back to durable scanning; parent cancellation still stops. Each prefetch window is bounded to eight pages and 8 MiB of serialized data.

## Actual-data comparison

Measured on cluster `ap-sin-01`, node `ap-sin-01-w3`, one isolated profiling pod limited to **2 CPUs / 1 GiB**. Both API variants used the same production native historical-state node. A measurement-only, read-only SQLite adapter used the exact production `Batch`, `Block` and `Checkpoint` queries with four reader connections against the live RPC index. Both URI `mode=ro` / `query_only` and a read-only PVC mount prevented database writes. Explorer received facts through JSON-RPC and owned a separate temporary SQLite database for every run.

The fixed sample is **144417200–144417711**, 512 blocks containing one actual transaction. Boundaries were checked against the native node and native finalized height before serving the fixture. Three rounds used forward/reversed/forward case order. Values below are median elapsed time; native counters cover the measured sync, excluding setup.

| Change | Median elapsed | Speedup | Native HTTP requests | Native logical calls |
|---|---:|---:|---:|---:|
| baseline | 9.867 s | 1.00x | 1577 | 1577 |
| rpc-only | 2.921 s | 3.38x | 169 | 1577 |
| explorer-only | 5.438 s | 1.81x | 1579 | 1579 |
| combined | 0.826 s | 11.95x | 171 | 1579 |

Combined improvement: **11.95x**, with **89.2% fewer native HTTP requests**. Logical calls increase by two because the prefetch path adds fresh boundary checks; chain/code validation was not removed.

All twelve runs produced one canonical operation with the identical full stored payload SHA-256:

`fbf21618f7050623326e15f509f074fc7d6679c0e690bdcfb1a2a149e9668d68`

An additional twelve-run control using fixed public indexed batches and the same real native node measured 9.519 s baseline versus 0.840 s combined (medians), with the same payload hash. The live read-only index comparison above includes real RPC database reads and is the primary result. [Raw samples and environment](benchmarks/settlement-catchup-20261124.json).

## Limits and correctness

This is a selected sparse interval, not a complete deployment-to-tip replay or production throughput guarantee. The Explorer test database uses pod `emptyDir`; production uses Longhorn. Optional recognition/root/broadcaster workers are not included. The real chain and production services continued running. No production checkpoint, database content, container image, domain or policy was changed. A full database-copy setup was abandoned after a profiling-job cleanup conflict; temporary copies were removed, and final measurements used enforced read-only access instead. Public-node trials that could not serve historical state are excluded.

Persistent settlement projection caching remains a later phase. This implementation keeps the existing public JSON-RPC method/cursor contract and native logical checks, optimizing transport grouping and bounded prefetch.

Both projects passed full Go race suites, vet and builds. Regression coverage includes reordered/missing/wrong native results, code/chain/body mismatches, explicit small-batch fallback, failed direct transactions, dense paging, uncle-body replacement, stale/out-of-order windows, interrupted terminal staging, atomic scope rejection, slow-prefetch fallback and diagnostics retention. One independent review's six findings were reproduced and fixed. Optional PostgreSQL/local-EVM suites were not enabled. A separate test verifies read-only measurement-query equivalence and rejects writes.

## Reproduce

In an authorized cluster environment, run RPC's `TestOnlineSettlementProfileServers` with `INVISI_SYNC_PROFILE_NODE_FILE` pointing to the existing native endpoint file, `INVISI_SYNC_PROFILE_PROTOCOL` pointing to Protocol RPC, and `INVISI_SYNC_PROFILE_READONLY_DATABASE` pointing to a read-only index mount. Set `INVISI_SYNC_PROFILE_FROM=144417200` and `INVISI_SYNC_PROFILE_BLOCKS=512`. The test serves baseline/candidate APIs on loopback ports 18881/18882 and runs no indexer.

From the same pod, run Explorer's `TestOnlineCatchUpComparison` with `INVISI_SYNC_PROFILE_BASELINE=http://127.0.0.1:18881`, `INVISI_SYNC_PROFILE_CANDIDATE=http://127.0.0.1:18882`, the same bounds, `INVISI_SYNC_PROFILE_REQUIRE_TRANSACTIONS=1` and an optional `INVISI_SYNC_PROFILE_REPORT` output path. It compares complete canonical payload hashes. Both tests skip without explicit opt-in environment variables. Native endpoint credentials must remain in the existing mounted file and out of logs/reports.
