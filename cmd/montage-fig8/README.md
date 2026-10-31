# Montage 58 / Figure 8 / HEFT

This command prepares and freezes the three Figure 8 cases, requests the native
HEFT planner, and submits the selected plan unchanged to simulation or execution.
Run from the repository root. Archived calibration inputs remain private under
`storage/`; they are required inputs, not generated observations.

```sh
go run ./cmd/montage-fig8 --ssh-ip CURRENT_PUBLIC_IPV4 \
  --transfer-profiles /path/to/evidenced-extra-route-profiles.json

go run ./cmd/montage-fig8 --action plan --case hpc
go run ./cmd/montage-fig8 --action run --case hpc --mode simulation
go run ./cmd/montage-fig8 --action run --case hpc --mode real
go run ./cmd/montage-fig8 --action report
```

Repeat for `micro` and `medium`. Register the generated complete environment,
workflow, scope, topology and capacity target JSON through the daemon API before
planning. Connection checks must succeed before registration. This tool never
resets the database, changes gcloud's active account, embeds credentials, or
registers connection secret values in environment definitions.

After all three simulations are complete, `--action run-all --mode real` executes
the real cases sequentially and refreshes the comparison after each case. It can
monitor an already submitted first case. It stops on failure and does not start
the next case or silently recover failed activities.

The cases use Diablo03 with 64 lanes; Diablo03 with one lane plus ten e2-micro
workers with one lane each; and Diablo03 with one lane plus two e2-medium workers
with two lanes each. Logical resources and runtimes are isolated by case; the
physical HPC node is shared, so real cases must run sequentially. A model-only
controller relay is present but cannot receive tasks. The micro target retains
its physical two vCPUs and explicitly limits scheduling capacity to one lane.

The source snapshot incorrectly labels existing v2 fixed service fields as v1.
Preparation corrects the schema declaration and validates the model without
changing coefficients. Extra transfer profiles can fill missing exact directed
class/strategy pairs; existing measurements are never replaced or borrowed from
other machine classes. Missing evidence remains an error, not a guessed rate.
These historical effective-wall fits are forecasts, not fresh observations.

The default uses the discovered home filesystem, matching the historical runs.
Every case declares the already implemented `consumer-barrier-v1` lifecycle:
preparation scratch is released at its barrier, and intermediate producer
workspaces are deleted after all downstream consumers materialize their inputs.
Final outputs remain retained. The conservative storage estimate can still exceed
the observed quota. Only with explicit user authorization, add
`--accept-hpc-quota-estimate` to the relevant run commands; the request records a
run-scoped exception while preserving the live observation and OS quota.

Optional `--workspace-root` selection uses a catalog storage on the same SSH connection.
The server observes both filesystem capacity and personal/group BeeGFS quota;
selecting a larger filesystem does not disable quota checks. Changing from the
historical home filesystem must be recorded when comparing old paper results.

Before real execution, register and build the pinned Montage OCI image as a SIF
using `/artifacts/docker/` and its build-run API, and wait for completion. The
Slurm adapter accepts a committed materialization only. Image build, image
transfer, cloud provisioning, runtime preparation and scientific compute must
remain distinguishable in result analysis.

Each run uses seed 1. Submitted requests and results are saved per run ID under
the output directory. An existing run is monitored, never submitted twice.
`--attempt 2` creates an explicit fresh attempt after a diagnosed failure; it does
not recover or reuse scientific tasks. Keep failed attempts as diagnostics and
never combine their timings with a successful fresh run. A real submission
requires a completed simulation with that exact plan ID and rejects overlap with
another active real execution. Real cloud targets use destroy-after-run lifecycle.

The defaults reference the October 2026 archive and home campaign; use `--source`,
`--campaign-id`, and `--output` explicitly when generating a different study.
The report exports JSON, CSV and HTML; pending or failed results remain missing,
never zero-valued successful observations. It distinguishes the run's reported
makespan from its full started-to-finished wall interval.
