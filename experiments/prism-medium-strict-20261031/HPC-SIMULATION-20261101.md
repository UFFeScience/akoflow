# Montage 58 — PRISM Time, HPC only

Local simulation, November 1, 2026. No real jobs or cloud instances launched.
58/58 activities completed in both final simulations.

New PRISM Time plan: `prism-time-hpc-simulation-20261101-v1-hpc-0b36f85ac583`.
Infrastructure and compute input: frozen Figure 8 HPC-only campaign request.
Uses the corrected preparation lane barrier.

| Case | Simulator end clock (s) | Activity span (s) |
|---|---:|---:|
| Original stage calibration, new PRISM plan | 731.4037441142812 | 731.4037441142812 |
| Measured HPC stages, identical placement | 1011.9735398911325 | 975.3303340234638 |

The new plan prediction with original calibration is 731.4037441142812 s.
The second row is an execution sensitivity, not a newly optimized calibrated
plan: same assignments and compute profiles, with runtime-stage calibration
trained on the previous strict HPC+medium real execution. It is not an HPC-only
real observation. HPC cost is zero in the input pricing model, not evidence
that the infrastructure has no economic cost.

The second row includes initial preparation/admission and final publication
in the simulator end clock. Activity span removes these boundary intervals
using the definition in SIMULATION-FIDELITY.md. No independent real run of
this newly selected PRISM placement was made, so no prediction error is claimed.

Daemon runs:

- `prism-time-hpc-simulation-20261101-v3-hpc-simulation`: original calibration.
- `prism-time-hpc-simulation-20261101-calibrated-v2-hpc-simulation`: full stage calibration.

Native offline verification and daemon agree exactly on 1011.9735398911325 s.
An intermediate diagnostic `...-calibrated-hpc-simulation` yielded
921.0261541031687 s because campaign loading replaced calibrated assignment
launch metadata with the original plan metadata. It is superseded by v2,
which retains the calibrated assignment overhead (659.368546962738 s summed
over 58 activities, overlapping across lanes). Do not use that intermediate
run as the full calibrated result.

Private requests, native verification, candidates and daemon responses:
`outputs/prism-time-hpc-simulation-20261101/`. Rejected submission snapshots
are retained as evidence; they did not execute scientific activities.
