# Real transfer strategy analysis — 2026-09-19

## Accepted matrix run

The `r3` batch submitted all seven workflows together. The environment lease
kept exactly one execution active while the remaining requests waited. All
seven runs completed.

| Route | Wall time (s) | Measured makespan (s) | Cost (USD) |
| --- | ---: | ---: | ---: |
| local → HPC | 62 | 61.829 | 0 |
| HPC → local | 40 | 21.005 | 0 |
| local → cloud | 242 | 170.466 | 0.000029257 |
| cloud → local | 125 | 8.875 | 0.000027789 |
| HPC → cloud | 186 | 97.850 | 0.000027724 |
| cloud → HPC | 196 | 138.147 | 0.000028034 |
| HPC → cloud → local → HPC | 254 | 238.454 | 0.000028829 |

Wall time includes infrastructure start/provision/validation/stop operations.
Measured makespan starts at the first activity runtime and ends at the last
activity runtime, so it intentionally excludes infrastructure work outside
that interval.

The cloud cost is not just `activity runtime × compute price`. The execution
total also includes observed instance idle time inside the activity window and
prorated disk cost. This explains why the execution cost is slightly higher
than the cloud task cost.

## Transfer observations

- The BusyBox SIF is 2,256,896 logical bytes. A previously materialized SIF can
  produce `0 transferred bytes` while retaining the logical size in the
  `source-push` observation.
- Workspace payloads are intentionally tiny (89–121 logical bytes). The
  measured network bytes are higher (307–620 bytes) because rsync protocol
  traffic is recorded separately from logical file bytes.
- The repaired cloud → HPC route completed through the gateway with 105
  logical/transferred bytes and 614 network bytes. Its workspace transfer took
  30.993 seconds in the accepted `r3` run.

## Failures found and corrected

1. The rsync remote-shell string single-quoted every SSH token. Nested quotes
   in `ProxyCommand` absorbed later options such as `-A`. The builder now leaves
   ordinary tokens unquoted and quotes only compound values; a test exercises
   the real rsync parser and verifies the exact SSH argument vector.
2. The SLURM `EXIT` trap inherited `set -e`. A best-effort final metric sample
   could abort the trap before the terminal sentinel was written. The trap now
   captures the activity exit code and disables `errexit` before auxiliary
   cleanup. The rerun published terminal sentinels normally.
3. Local image execution used attached `docker run --rm` and depended on an
   in-memory `Wait` result. It now starts detached, inspects Docker's persistent
   terminal state, catalogs outputs, persists the handle, and removes the
   container only after successful persistence.
4. Short Docker activities could finish before the first polling interval and
   therefore have no telemetry row. Local and SSH/cloud Docker adapters now
   collect an initial sample during start, and the controller persists start
   metrics. The focused `r5` validation recorded one local and one cloud sample
   for activities lasting 0.119 and 4.347 seconds.

## Follow-up validation

- `r4` repeated the full roundtrip with detached local Docker execution and
  completed in 314 seconds wall time. The local activity published `Exited (0)`
  in 0.122 seconds and was observed without relying on the process waiter.
- `r5` repeated local → cloud with initial telemetry enabled and completed in
  217 seconds wall time. It recorded two Docker telemetry rows (local and cloud)
  and the cloud task cost reconciled as `4.347 s × 0.0000093071416667/s` plus
  the execution-level prorated disk component.
- All instances created by these validations were destroyed. No execution job,
  cloud operation, or environment lease from the matrix remained active.
- SQLite `PRAGMA integrity_check` returned `ok`.

