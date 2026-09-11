# Kind real-execution example

This disposable Kind cluster runs the two-step `dag.yaml` workflow through the Kubernetes API. It verifies the complete path: a Job writes `result.txt`, AkôFlow materializes that workspace for the dependent Job, and the second Job writes `consumed.txt` with the same content.

`akoflow-access.yaml` deliberately grants `cluster-admin`. It is appropriate only for this local example; create a namespace-scoped least-privilege role for any shared cluster.

## Prerequisites

- Docker, `kind`, `kubectl`, `curl`, and `jq` on the host.
- A locally running AkôFlow daemon. The commands below use its default API URL.
- The repository root as the working directory.

## 1. Create the disposable cluster

```bash
kind create cluster --name akoflow --config examples/kind/cluster.yaml
kubectl --context kind-akoflow apply -f examples/kind/akoflow-access.yaml
kubectl --context kind-akoflow apply -f examples/kind/storage.yaml
kubectl --context kind-akoflow -n akoflow get serviceaccount,pvc
```

The `akoflow-data` PVC may remain `Pending` until the first Job is scheduled; that is normal for Kind's default storage class.

## 2. Store a short-lived Kubernetes credential

Generate a token without printing it, then send it to the daemon credential endpoint. `environment.yaml` references the resulting local credential file.

```bash
export AKOFLOW_API_URL="http://127.0.0.1:8080/akoflow-api"
export AKOFLOW_API_TOKEN="<daemon API token>"
KIND_TOKEN="$(kubectl --context kind-akoflow -n akoflow create token akoflow-runtime --duration=1h)"

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' \
  --data "{\"id\":\"kind-akoflow\",\"token\":\"$KIND_TOKEN\"}" \
  "$AKOFLOW_API_URL/kubernetes-tokens/"
unset KIND_TOKEN
```

The supplied endpoint is for a daemon running in Docker on macOS, where `host.docker.internal` reaches the Kind API. For another installation, replace the environment connection endpoint with an address reachable from that daemon.

## 3. Register the inputs and start the run

The order is significant: the environment and scope must exist before the topology, workflow, plan, and execution request.

```bash
post_yaml() {
  curl --fail-with-body \
    -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
    -H 'Content-Type: application/yaml' \
    --data-binary "@$2" "$AKOFLOW_API_URL/$1/"
}

post_yaml environments examples/kind/environment.yaml
post_yaml execution-scopes examples/kind/scope.yaml
post_yaml network-topologies examples/kind/topology.yaml
post_yaml workflow-definitions examples/kind/dag.yaml
post_yaml schedule-plans examples/kind/requests/plan-request.yaml
post_yaml execution-runs examples/kind/requests/execution-request.yaml
```

## 4. Verify Jobs, transferred output, and the persisted run

```bash
kubectl --context kind-akoflow -n akoflow get jobs,pods,pvc
kubectl --context kind-akoflow -n akoflow logs job/akoflow-kind-dag-run-v8-kind-dag-prepare
kubectl --context kind-akoflow -n akoflow logs job/akoflow-kind-dag-run-v8-kind-dag-process

curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/execution-runs/kind-dag-run-v8/" | jq .
```

On 2026-09-11, this exact bundle completed as `kind-dag-run-v8`: both Jobs completed, the workspace transfer moved 9 bytes, and `result.txt` and `consumed.txt` had checksum `sha256:cb064c1339ffa3d7777bcb0459de3dceddb9146156dde58065a4ac826b029aa7`. The observed makespan was 17.776 s; it includes Kubernetes scheduling and Pod startup, so it is intentionally longer than the two two-second shell sleeps.

If a run remains pending, inspect the pod events. A PVC with `ReadWriteOnce` must be mounted on the node selected by its planned resource; the example's workspace transfer pods preserve that placement.

## Cleanup

Only run this command for the disposable cluster created above:

```bash
kind delete cluster --name akoflow
```

## Files

- `cluster.yaml`: three-node Kind topology.
- `akoflow-access.yaml`: local-only ServiceAccount and access binding.
- `storage.yaml`: PVC used by the example.
- `environment.yaml`, `scope.yaml`, and `topology.yaml`: catalog inputs.
- `dag.yaml`: portable two-activity definition.
- `requests/plan-request.yaml` and `requests/execution-request.yaml`: complete API envelopes.
