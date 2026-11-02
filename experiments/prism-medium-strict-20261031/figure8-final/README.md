# Montage 58 — six independent Figure 8 executions

Figure 8 now uses explicitly shared representative observations from
`../figure8-shared-summary/`. This directory preserves the six independent
measured executions and their original provenance unchanged.

All six algorithm/configuration pairs now have distinct physical execution IDs. Two additional deployments replace the previously shared observations: MOHEFT-TIME/HPC and HEFT/HPC+medium. The original four physical executions and all historical observations remain preserved. No plan, simulation, calibration, network model or input was refitted or changed.

| Environment | Algorithm | Planned (s) | Simulated (s) | Real (s) | Absolute simulation error |
|---|---|---:|---:|---:|---:|
| HPC | PRISM-TIME | 975.33 | 975.33 | 1174.07 | 16.93% |
| HPC | HEFT | 585.02 | 1436.36 | 1608.76 | 10.72% |
| HPC | MOHEFT-TIME | 585.02 | 1436.36 | 1261.64 | 13.85% |
| HPC + medium | PRISM-TIME | 4168.83 | 4180.99 | 3353.96 | 24.66% |
| HPC + medium | HEFT | 1791.42 | 4180.99 | 3173.32 | 31.75% |
| HPC + medium | MOHEFT-TIME | 1845.23 | 4954.23 | 4054.02 | 22.21% |

Errors are 100 *abs(simulated-real)/real. All plotted times use the common boundary from first activity start, before launch overhead, through final process completion; initial preparation/admission and final workspace publication are excluded. Original model end clocks and run wall durations remain in CSV/JSON. The gray plot range is max(P,S,R)-min(P,S,R), with 100 *range/P; it is descriptive, not a confidence interval.

HEFT and MOHEFT-TIME retain equivalent HPC placement/lane graphs. In medium, PRISM-TIME selected its native HEFT candidate. These equivalent plans were nevertheless executed independently, so different real times do not establish an algorithmic scheduling advantage. Each method retains its own planned estimate; all simulations use the same frozen operational model. Six single deployments do not establish statistical superiority.

| Case | Real execution ID |
|---|---|
| prism-time/hpc | fig8-guard-prism-time-fresh-20261101-v1-hpc-real |
| heft/hpc | fig8-guard-heft-shared-20261101-v2-hpc-real-attempt3 |
| moheft/hpc | fig8-independent-moheft-time-20261101-v1-hpc-real |
| prism-time/medium | fig8-guard-prism-heft-shared-20261101-v1-medium-real |
| heft/medium | fig8-independent-heft-time-20261101-v1-medium-real |
| moheft/medium | fig8-guard-moheft-time-20261101-v1-medium-real |

Earlier IDs containing the word `shared` retain their original historical names; each is used by only one row in the final independent comparison.

All six executions completed 58/58 without activity recovery and passed final v3 assignment, physical HPC node and precedence audits. Four final PNGs and 58 activity telemetry exports per execution were preserved, together with native job files and Slurm scripts. Product differences are quantified in final-products-crosscheck.json.

Owned cloud VMs, boot disks, firewall rules, temporary credentials and HPC/local script directories were removed only after evidence preservation and terminal-state verification. Permanent data, images and base credentials remain. Cleanup proofs and raw evidence are under outputs/fig8-independent-real-20261101/ and the four original directories referenced in deployments.json.

The two observer-only startup corrections are documented separately; neither restarted an activity or altered a scientific execution. Historical 1144.38s and recovered 4143.89s observations remain excluded from this frozen cohort.

PNG crosscheck: all HPC products are byte-identical. Within medium, the independent HEFT replay differs from PRISM-TIME in at most 0.105% of pixels; maximum 8-bit channel difference 22 and maximum channel RMSE 0.08736. MOHEFT/PRISM products remain byte-identical in medium. These measured differences are preserved without assigning an unverified cause.
