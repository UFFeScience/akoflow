# Montage58: frozen Figure8 replay,2026-11-01

Six algorithm/configuration rows use four fresh physical deployments. Inputs,
calibration and native simulations were frozen before observing these replays.
No new outcome was used to refit the forecasts. All runs completed58 activities
without activity recovery. Durations are seconds.

| Environment | Algorithm | Planned | Simulated | Real | Absolute simulation error |
|---|---|---:|---:|---:|---:|
| HPC | PRISM-TIME |975.33|975.33|1174.07|16.93%|
| HPC | HEFT |585.02|1436.36|1608.76|10.72%|
| HPC | MOHEFT-TIME |585.02|1436.36|1608.76|10.72%|
| HPC+medium | PRISM-TIME |4168.83|4180.99|3353.96|24.66%|
| HPC+medium | HEFT |1791.42|4180.99|3353.96|24.66%|
| HPC+medium | MOHEFT-TIME |1845.23|4954.23|4054.02|22.21%|

Errors are100*abs(simulated-real)/real. All durations span first activity start
before launch overhead through final process completion; initial preparation/
admission and final workspace publication are excluded. Original model end
clocks and run wall durations remain in CSV/JSON. The plot range is descriptive:
max(P,S,R)-min(P,S,R),with percentage100*range/P,not a confidence interval.

HEFT/MOHEFT share the identical logical placement/lane graph in HPC. PRISM v4
selected its native HEFT anchor in medium,so PRISM/HEFT share that deployment.
Explicit plan IDs,executed representative IDs,relationship labels,run IDs,
request/plan/native hashes and simulation-replay identities are exported.
Planned values preserve each algorithm's own estimate; simulation uses the
same frozen operational runtime-stage model for every plan. Near P/S agreement
does not imply accurate real prediction; residual errors remain material.

| Physical deployment | Real run | Verified HPC nodes | Verified precedence relations |
|---|---|---:|---:|
| HPC HEFT/MOHEFT |fig8-guard-heft-shared-20261101-v2-hpc-real-attempt3|58|134|
| HPC PRISM |fig8-guard-prism-time-fresh-20261101-v1-hpc-real|58|114|
| Medium PRISM/HEFT |fig8-guard-prism-heft-shared-20261101-v1-medium-real|4|147|
| Medium MOHEFT |fig8-guard-moheft-time-20261101-v1-medium-real|7|154|

All final v3 audits have zero assignment,node or ordering violations.
Historical PRISM HPC1144.38s is preserved separately: its native trace includes
Diablo04 fallback for one activity and its request snapshot carries a different
plan ID. The recovered historical medium4143.89s is also separate. Failed
initial infrastructure attempts are recorded in excluded-attempts.json; an
observer-only restart for a pending node inspection is documented separately.

Each fresh deployment has four final PNGs verified by size,SHA256 and PNG
parsing,complete task logs,all58 task telemetry exports,and archived Slurm
scripts/job evidence. Products are byte-identical across algorithms within each
environment. Cross-environment pixel differences are quantified in the supplied
crosscheck; no assertion of bitwise equality across configurations is made.

Both medium deployments' two owned VMs,boot disks and firewall rules were
verified absent in full provider inventories. Sixteen temporary credential
files were removed across the four VM generations. Owned HPC roots/home job
files were removed only after preservation and empty squeue. Permanent inputs,
images,network and base credentials remain. No real executions remain active.

Raw evidence and cleanup proofs are under outputs/fig8-guard-real-20261101/.
The dedicated operational catalog is storage/fig8-guard-20261101/database.db;
automatic schema recreation is disabled and reset is refused during execution.
Paper data mirror these CSV/JSON files; Figure8 is rendered from that mirror.
Single deployments do not establish statistical superiority.
