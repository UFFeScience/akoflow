# Preparation fidelity investigation — 2026-11-01

The 2,175.16 s residual is **non-compute time on the serial HPC path**,
not an isolated measurement of network service. It includes data readiness
(including work on other resources), preparation, scheduler admission,
launch, observation and publication. These intervals overlap with other
activities; aggregated transfer durations cannot simply be added to makespan.

## Confirmed behavior mismatch

The real strict-order executor waits for both workflow dependencies and the
previous assignment on the execution lane **before preparing the next task**.
The materialization simulator used to start transfers once producer data was
ready, even while the lane predecessor was running. That incorrectly hid
preparation behind other work. The native runner and PRISM evaluator now gate
preparation on lane readiness. Independent lanes can still transfer in parallel.
Native capability `preparationLaneBarrierVersion: 1` prevents silently using an
older executable with a materialized ordered plan.

## Offline results, frozen placement and compute profiles

Reference: `prism-time-medium-strict-20261031-v1-medium-real`, 58 activities,
5,212.838178634644 s. No new real execution was launched.

| Model | Activity span (s) | Absolute error / real |
|---|---:|---:|
| Original simulation | 3,007.14 | 42.31% |
| Lane preparation barrier only | 3,436.41 | 34.08% |
| Barrier + measured HPC runtime stages | 4,334.16 | 16.86% |
| Barrier + HPC/cloud stages + effective transfer wall calibration | 5,389.67 | 3.39% |

Only the second row isolates a code behavior change. The last two rows use
the reference execution itself as training data: **retrospective fits, not
independent accuracy validation**. Do not replace the original prediction in
historical results or claim a prospective 3.39% error. No makespan multiplier,
target duration, changed assignment or changed compute profile is used.

The frozen input had no modeled Slurm admission delay and stale preparation /
publication parameters. Existing runtime calibration fits nonnegative fixed
and per-byte terms for scan/merge/verification/publication, median admission
and fixed launch overhead, separately per runtime class. The transfer-wall
sensitivity uses measured route stages; these effective times can include
waiting/contention. It removes explicit bulk service groups to avoid counting
their queues twice. This is an exploratory effective-wall model, not proof of
queue-free network capacity. It must not become a universal default.

## Consistent measurement boundaries

Native `executed.makespanSeconds` ends after final publication and includes
initial preparation/admission. Its calibrated value is 5,405.0163518847 s.
The real reported activity span excludes initial preparation/admission and
final publication. For the diagnostic comparison above, use
`max(finishedAt - workspacePublicationSeconds) - min(startedAt)` in native
traces: 5,389.668488382293 s. Intermediate publication remains causal for
successors. The existing native/API metric contract has not been silently
changed. The Go analysis exports both boundaries explicitly.

## Reproduction and evidence

Private frozen requests/results remain under
`outputs/simulation-fidelity-20261101/`; they are not committed because request
snapshots can contain infrastructure configuration. The committed
`simulation-fidelity.json` records trace digests and metrics. Recompute with:

```sh
go run experiments/prism-medium-strict-20261031/simulation_fidelity.go
```

Generate requests with the existing Go scientific tooling, in this order:

```sh
go run ./cmd/montage-tools calibrate_runtime_stages hpc FROZEN_SIM_REQUEST STRICT_REAL_MONITOR HPC_REQUEST
go run ./cmd/montage-tools transfer_wall_sensitivity HPC_REQUEST STRICT_REAL_MONITOR TRANSFER_REQUEST
go run ./cmd/montage-tools calibrate_runtime_stages e2-medium TRANSFER_REQUEST STRICT_REAL_MONITOR ALL_REQUEST
```

Export each offline case using `simgrid.BuildOfflineCase`, then execute the
native runner against its `platform.xml` and `input.json`. The tested image is
`akoflow-simgrid-strict-preparation`. Regression tests verify that a transfer
waits until time 10 behind a lane predecessor, but starts at time 1 on an
independent lane; PRISM also must not reserve a service pool while blocked.

An independent run with frozen calibration, identical measurement boundaries,
and per-stage observations is required before reporting predictive accuracy.
The remaining residual should be checked against queue variation, effective
transfer-stage waiting and family-specific launch/observation costs.
