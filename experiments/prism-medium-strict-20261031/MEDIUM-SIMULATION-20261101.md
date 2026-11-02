# Montage 58 — calibrated PRISM Time, HPC + medium

Daemon simulation completed with 58/58 activities. Run:
`prism-time-medium-simulation-20261101-calibrated-medium-simulation`.

Uses the unchanged placement
`fig8-prism-time-matched-20261031-v1-medium-0e673f4ddfbb` and the complete
runtime-stage / effective transfer-wall sensitivity request documented in
SIMULATION-FIDELITY.md. Assignment launch overhead metadata is retained.

- Simulator end clock: **5405.0163518847 s**, identical to offline native verification.
- Comparable activity span: **5389.668488382293 s**.
- Previous strict real activity span: **5212.838178634644 s**.
- Activity-span difference: **+176.83030974764915 s (+3.3922079237450045%)**.
- Transferred bytes: 8189950503.

This repeats the calibrated simulation through the local daemon; no new real
execution or cloud provisioning occurred. The real trace was itself used to
fit this calibration. The 3.39% is a retrospective fit error, not independent
predictive validation. Original 3007.1380910688645 s simulation remains intact.

Private submitted request, status response and full activity trace are under
`outputs/prism-time-medium-simulation-20261101/`.
