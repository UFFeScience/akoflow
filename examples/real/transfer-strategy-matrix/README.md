# Real transfer strategy matrix

This bundle prepares, but does not automatically execute, small real workflows
across the environments currently registered in the development instance:

- Local: `local-environment-entrypoint`
- HPC/SLURM: `hpc-environment-ssh-connection-partition-testing`
- GCP: `montage-gcp-e2-medium-1`

The v2 six directional two-stage workflows verify file content and SHA-256 after
each handoff. `workflow-roundtrip.json` additionally exercises HPC → GCP →
Local → HPC in one run.

Run `./generate.sh` after changing `cases.tsv`. The generator passes complete
shell programs to `jq` as arguments; do not interpolate shell quotes directly
inside its filter. Import `scope.json` and
`topology.json` before importing workflows and plans. The bandwidth values in
the topology are modeling inputs, not measured network capacity.

Run `./verify.sh` before importing. It regenerates the six directional fixtures,
parses every JSON file, and executes all seven command chains in isolated local
workspaces. It does not contact HPC or provision cloud infrastructure.

Before submitting any workflow containing a Slurm activity, prepare the
BusyBox SIF in the AkôFlow artifact catalog:

```sh
./prepare-busybox-artifact.sh
```

The script is idempotent once `busybox` version `1.36` has an available amd64
SIF. Slurm never pulls `docker://` images on the compute node: AkôFlow resolves
the completed catalog build and materializes the SIF before `sbatch`.

With `AKOFLOW_API_TOKEN` set, `./import.sh` imports the complete v2 bundle. Its
IDs intentionally coexist with the invalid v1 batch so the failed execution
history remains inspectable. It is not an upsert: import v2 only once.
Submit one case later with, for example:

```sh
./submit-run.sh hpc-cloud transfer-matrix-hpc-cloud-run-1
./submit-run.sh roundtrip transfer-matrix-roundtrip-run-1
```

These workflows intentionally do not force an internal transfer strategy. The
strategy resolver must choose from the real endpoint topology. Validation must
compare the recorded `strategy`, `route.strategy`, `fallback`, logical bytes,
network bytes, timestamps, and checksums with the expected route.

The GCP target uses `stop-when-idle` in the current database. Before running
the matrix, confirm quota, firewall source ranges, lifecycle policy, and that
the target can reach the HPC endpoint only through an approved route. The HPC
`testing` partition and its site policies must also be approved for the probe.

The intended environment-lease validation submits all seven runs without
waiting between requests:

```sh
./submit-all.sh lease-test-1
```

All plans use the same execution scope, which contains Local, HPC and GCP.
AkôFlow must grant one environment lease, keep the remaining runs queued, and
start the next run only after the previous run reaches a terminal state and
releases its lease. The submission script does not implement client-side
serialization.

Logical order used only to read the resulting evidence:

1. Local → HPC and HPC → Local.
2. Local → GCP and GCP → Local.
3. HPC → GCP and GCP → HPC.
4. The round-trip workflow.

Do not resubmit a failed batch until its terminal run has released the lease
and any retained cloud instance has been stopped or destroyed. This avoids
turning retries into parallel provisioning costs.
