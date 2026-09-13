# Hybrid local/Kind to GCP transfer checks

These two small real-execution workflows verify data handoff across different
runtimes. `local-cloud-workflow.yaml` writes `handoff.txt` on the local engine;
`kind-cloud-workflow.yaml` writes it in Kind. In each case the GCP activity
checks the exact content before producing `receipt.txt`.

The plans target an existing execution scope named `goal-hybrid-scope` with
`local-environment-initial`, `kind-akoflow-v1`, and `goal-gcp-worker-v1`, a
network topology named `goal-hybrid-network`, and a GCP capacity target named
`goal-gcp-worker-e2-small`. The target uses an Ubuntu image and the built-in
scientific-worker machine configuration so Docker is installed before the run.
Do not submit these plans against a different project or a broad SSH firewall
rule without reviewing the target first.

With those resources registered, import each workflow through
`POST /workflow-definitions/` and each plan through
`POST /schedule-plans/import/` using `Content-Type: application/yaml`. Then:

```sh
export AKOFLOW_API_URL=http://127.0.0.1:8080/akoflow-api
export AKOFLOW_API_TOKEN=<daemon-token>
sh examples/real/hybrid-cloud-transfer/submit-run.sh local goal-local-cloud-run-1
# Wait until the run and its cloud destruction operation finish.
sh examples/real/hybrid-cloud-transfer/submit-run.sh kind goal-kind-cloud-run-1
```

The submit script reads the stored plan, workflow, scope, resources and runtime
bindings, then sends a complete execution request. A short run-only request
is not sufficient for the execution queue.

Acceptance evidence from 2026-09-13 in project `sandbox-391923`:

| Run | Result | Bytes transferred | Observed makespan |
| --- | --- | ---: | ---: |
| `goal-local-cloud-run-v6` | 2/2 completed | 21 | 70.051 s |
| `goal-kind-cloud-run-v1` | 2/2 completed | 20 | 131.993 s |

Both runs used `e2-small` VMs and their lifecycle actions destroyed the VMs.
Verify this independently with the AkôFlow cloud-instance status and the GCP
Compute Engine instance list. A failed run may require investigating an active
transfer before the lifecycle guard will permit VM destruction.
