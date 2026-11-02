# Recalibrated PRISM Time: planning, simulation and real-plan identity

Both HPC-only and HPC + medium were replanned from their calibrated frozen
requests and independently simulated using the native SimGrid production
executor capability/trace checks. Default PRISM configuration, 25 candidates
per case. No real execution was started. No paper data was overwritten.

| Environment | New planned end clock (s) | Native simulated end clock (s) | Existing real activity span (s) |
|---|---:|---:|---:|
| HPC | 1011.9735398911325 | 1011.9735398911325 | 1144.379519701004, historical equivalent placement |
| HPC + medium | 4581.695528246053 | 4593.705528246053 | 5212.838178634644, **different placement** |

Planner/native values retain their end-clock contracts; real observations
use activity spans, so these columns are not an error calculation. The medium
planner/native difference is 12.01 s (0.26213%).

## HPC reference

The historical request was located under
`storage/montage-runtime-stage-calibration-20261011/prism-fastest-20261011-v1/`.
Its 58 assignment IDs and planned resources were checked against the 58
completed activities of `montage-prism-fastest-real-20261011-v1`.
The real observation is 1144.379519701004 s.

Relative to that executed plan, the recalibrated plan changes zero resource
placements, zero per-lane orders and zero activity priorities. 37 logical
core IDs differ. The old-to-new core mapping is a bijection, and the entire
activity-level lane-predecessor graph is identical. Both requests expose
64 homogeneous slots of the same Diablo03 machine. Figure 8 resource/core
ID prefixes are explicitly removed as compatibility aliases, not fuzzy
machine matching. Thus the schedules are equivalent **up to interchangeable
logical core labels in the model**, not byte-identical plan records.

The historical real can remain as an observation of equivalent executable
placement, clearly labeled as historical and using different calibration.
It is not a new real run, nor proof that real physical cores are perfectly
interchangeable or that the historical runtime protocol matches today's.
The previous blanket statement that this HPC plan necessarily lacked an
equivalent real observation was too strong; this audit resolves that point.

## HPC + medium reference

The exact submitted request of the strict-order real run
`prism-time-medium-strict-20261031-v1-medium-real` was checked against its
58 completed activity assignment IDs and resources. New plan versus that
executed plan:

- 19 activities change resource.
- 26 change logical core ID.
- 41 change per-lane order.
- 55 differ in at least one executable assignment field; counts overlap.
- Priorities do not change; lane-predecessor graphs differ.
- New distribution: 25 HPC, 12 medium VM 1, 21 medium VM 2.

The 5212.84 s real observation cannot be attached to this new plan.
A fresh real run is required to evaluate the changed placement. The old
real remains valid for its own original plan and calibrated replay.

## Reproducibility

`replan.go` now derives plan/run IDs from the output directory and updates
assignment IDs consistently, preventing HPC and medium diagnostics from
sharing a misleading hardcoded medium plan ID.

New candidate sets, planning inputs/configuration, selected plans and native
traces are private in `outputs/prism-time-recalibrated-final-20261101-hpc/`
and `outputs/prism-time-recalibrated-final-20261101-medium/`.
The committed `hpc-recalibrated-plan-audit.json` and
`medium-recalibrated-plan-audit.json` contain input SHA-256 digests and all
per-activity assignment differences. Run the standalone Go `compare_plan.go`
with OLD_EXECUTION_REQUEST, NEW_PLAN and REAL_TRACE to reproduce each audit.
