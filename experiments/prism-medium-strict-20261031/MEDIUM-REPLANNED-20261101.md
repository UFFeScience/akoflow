# PRISM Time replanning — HPC + medium

The planner was rerun from the fully calibrated frozen request, without
changing live inventory or starting a real execution. Default PRISM Time
configuration: beam width 120, option count 25, ready branch limit 3,
interference aware. The source request has no execution interference matrix.

| Observation | Planned (s) | Native simulation end clock (s) |
|---|---:|---:|
| Old plan, original model | 3003.8829210098506 | 3007.1380910688645 |
| Old placement, calibrated simulation | historical prediction retained | 5405.0163518847 |
| Newly optimized plan, calibrated model | **4581.695528246053** | **4593.705528246053** |

The new native simulation is 12.01 s (0.26213%) above the new prediction.
Both values use their existing planner/native end-clock contracts, not the
activity-span normalization used to compare to real traces elsewhere.
The new simulation is about 15.01% below the calibrated replay of the old
placement. This is a model-based improvement, not a measured real speedup.

The selected plan assigns 25 activities to HPC, 12 to medium VM 1 and 21 to
medium VM 2 (58 total). The old plan assigned 30 activities to HPC.
There is no real observation of the new placement. The previous 5212.84 s
real result belongs to the old plan and must not be used as its real result.
The calibration itself was trained on that previous real execution, so
planner/simulator agreement does not independently establish real accuracy.

The source and old plan are preserved. Private outputs, all 25 candidates,
configuration, selected plan, new simulation request and validated native
trace are in `outputs/prism-time-medium-replanned-20261101/`.

Reproduce from the repository root:

```sh
go run experiments/prism-medium-strict-20261031/replan.go outputs/simulation-fidelity-20261101/all-calibrated-request.json NEW_OUTPUT
go run ./cmd/montage-tools verify_native_case NEW_OUTPUT/simulation-request.json akoflow-simgrid-strict-preparation /build/output/akoflow-simgrid-runner NEW_OUTPUT/native-verification
```

Native verification uses the production executor's capability and trace
validation checks; it is not merely a copied planner estimate. No new real
jobs or cloud provisioning were performed, and the article is not changed
by this diagnostic.
