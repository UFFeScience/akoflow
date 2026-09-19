# Real transfer strategy matrix

This bundle prepares, but does not automatically execute, small real workflows
across the environments currently registered in the development instance:

- Local: `local-environment-entrypoint`
- HPC/SLURM: `hpc-environment-ssh-connection-partition-testing`
- GCP: `montage-gcp-e2-medium-1`

The six directional two-stage workflows verify file content and SHA-256 after
each handoff. `workflow-roundtrip.json` additionally exercises HPC → GCP →
Local → HPC in one run.

Run `./generate.sh` after changing `cases.tsv`. Import `scope.json` and
`topology.json` before importing workflows and plans. The bandwidth values in
the topology are modeling inputs, not measured network capacity.

With `AKOFLOW_API_TOKEN` set, `./import.sh` imports the complete bundle. It is
not an upsert: use it on a clean instance or remove/version conflicting IDs.
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

Suggested execution order:

1. Local → HPC and HPC → Local.
2. Local → GCP and GCP → Local.
3. HPC → GCP and GCP → HPC.
4. The round-trip workflow.

Run one cloud workflow at a time and wait for the cloud lifecycle operation to
settle before starting the next one.
