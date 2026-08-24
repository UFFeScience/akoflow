# Kind real-execution example

This example creates a local Kind cluster and lets Akoflow execute the two-step
workflow in `dag.yaml` through the Kubernetes HTTPS API. Akoflow does not invoke
`kubectl`; it authenticates with the ServiceAccount bearer token configured in
its environment.

`akoflow-access.yaml` grants `cluster-admin` intentionally. This makes the Kind
development environment suitable for jobs, services, resource discovery,
metrics and future Kubernetes integrations. Do not use this access profile in
production.

## 1. Create the cluster

Run on the host:

```bash
cd /Users/ovvesley/Workspace/akoflow-workflow-engine

kind create cluster --config examples/kind/cluster.yaml
kubectl apply -f examples/kind/akoflow-access.yaml
```

If the cluster already exists, only the `kubectl apply` command is required.

## 2. Create the API token

Run on the host:

```bash
AKOFLOW_TOKEN="$(kubectl -n akoflow create token akoflow-runtime --duration=24h)"
test -n "$AKOFLOW_TOKEN"
```

The token expires after 24 hours. Generate a replacement with the same command.

## 3. Find the published API port

```bash
AKOFLOW_K8S_PORT="$(docker port akoflow-control-plane 6443/tcp | awk -F: '{print $NF}')"
test -n "$AKOFLOW_K8S_PORT"
echo "$AKOFLOW_K8S_PORT"
```

For the current example cluster this is normally a random host port, such as
`63395`. Akoflow reaches it through `host.docker.internal`.

## 4. Configure the development container

Set the container name if Docker assigned a different one:

```bash
AKOFLOW_CONTAINER="adoring_matsumoto"
```

Write the API configuration without printing the token:

```bash
docker exec \
  -e GENERATED_AKOFLOW_TOKEN="$AKOFLOW_TOKEN" \
  -e GENERATED_K8S_PORT="$AKOFLOW_K8S_PORT" \
  "$AKOFLOW_CONTAINER" \
  sh -lc '
    cd /app
    touch .env
    sed -i \
      -e "/^K8S_API_SERVER_HOST=/d" \
      -e "/^K8S_API_SERVER_TOKEN=/d" \
      -e "/^K8S_API_SERVER_CA_FILE=/d" \
      -e "/^K8S_API_SERVER_INSECURE_SKIP_TLS_VERIFY=/d" \
      .env
    printf "%s\n" \
      "K8S_API_SERVER_HOST=host.docker.internal:${GENERATED_K8S_PORT}" \
      "K8S_API_SERVER_TOKEN=${GENERATED_AKOFLOW_TOKEN}" \
      "K8S_API_SERVER_INSECURE_SKIP_TLS_VERIFY=true" \
      >> .env
  '
```

Keep these application settings in `/app/.env` as well:

```env
AKOFLOW_DATABASE_PATH="$PWD/storage/kind-demo-v3.db"
AKOFLOW_HTTP_ADDRESS=":8080"
AKOFLOW_NAMESPACE="akoflow"
AKOFLOW_KUBERNETES_HISTORY_CLEANUP_ENABLED="true"
AKOFLOW_KUBERNETES_HISTORY_CLEANUP_INTERVAL="15m"
AKOFLOW_KUBERNETES_HISTORY_RETENTION="24h"
```

The history cleaner runs once when the server starts and then at the configured
interval. It deletes only completed or failed Jobs labeled as managed by
Akoflow whose completion time is older than the retention period. Associated
Pods and Services are removed as well; active Jobs and Pods are preserved.

Stop and restart `Launch AkoFlow Server` after changing `.env` because the
Kubernetes client is composed during application startup.

## 5. Verify authentication and permissions

Test the exact network path used by Akoflow:

```bash
docker exec \
  -e K8S_API_SERVER_TOKEN="$AKOFLOW_TOKEN" \
  -e K8S_API_SERVER_PORT="$AKOFLOW_K8S_PORT" \
  "$AKOFLOW_CONTAINER" \
  sh -lc '
    curl -sk \
      -H "Authorization: Bearer $K8S_API_SERVER_TOKEN" \
      "https://host.docker.internal:${K8S_API_SERVER_PORT}/apis/batch/v1/namespaces/akoflow/jobs"
  '
```

Verify the Akoflow server:

```bash
docker exec "$AKOFLOW_CONTAINER" curl -fsS http://localhost:8080/
```

The expected response is `ok`.

## 6. Register and execute the DAG

Run the commands below directly inside the development container:

```bash
cd /app
```

Create the persistent workflow-data volume from the host before submitting the run:

```bash
kubectl apply -f examples/kind/storage.yaml
kubectl -n akoflow get pvc akoflow-data
```

The Kind `standard` StorageClass uses `WaitForFirstConsumer`, so the claim stays
`Pending` until the first activity Pod is scheduled. It becomes `Bound` at that point.

Register the environment:

```bash
curl -fsS \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/kind/environment.yaml \
  http://localhost:8080/akoflow-api/environments/
```

Register the versioned network topology. The topology is independent from the
environment catalog, so its endpoints can belong to one environment or span
multiple environments:

```bash
curl -fsS \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/kind/scope.yaml \
  http://localhost:8080/akoflow-api/execution-scopes/

curl -fsS \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/kind/topology.yaml \
  http://localhost:8080/akoflow-api/network-topologies/
```

Register the workflow:

```bash
curl -fsS \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/kind/dag.yaml \
  http://localhost:8080/akoflow-api/workflow-definitions/
```

Create and validate the schedule plan. This request explicitly combines the
plan, workflow version and resources used by the validator:

```bash
curl -fsS \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/kind/requests/plan-request.yaml \
  http://localhost:8080/akoflow-api/schedule-plans/
```

Submit the real execution. The request contains the immutable plan and the
environment snapshot needed by the worker, so execution does not depend on
mutable catalog data:

```bash
curl -fsS \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/kind/requests/execution-request.yaml \
  http://localhost:8080/akoflow-api/execution-runs/
```

Watch the Kubernetes jobs from the host:

```bash
kubectl -n akoflow get jobs,pods -w
```

After completion, inspect the activity logs:

```bash
kubectl -n akoflow logs job/akoflow-kind-dag-run-v1-prepare
kubectl -n akoflow logs job/akoflow-kind-dag-run-v1-process
```

Inspect the persisted run, activity projections and immutable domain events:

```bash
curl -fsS \
  http://localhost:8080/akoflow-api/execution-runs/kind-dag-run-v1/
```

The response contains `run`, `activities` and `events`. Lifecycle changes are
written atomically to the execution projection, `domain_events` and the durable
outbox delivery in `queue_jobs`.

This example uses fixed identifiers. To run it again against the same database,
change the run ID in `run.yaml`, or start with a new development database.

### Resource identity

`kind-worker` is Akoflow's stable logical resource ID. Its `providerId` is
`akoflow-worker`, the real Kubernetes node name created by Kind. The plan refers
to the logical ID, while the Kubernetes adapter uses `providerId` in the pod's
`kubernetes.io/hostname` node selector.

## Example definitions

- `environment.yaml`: Kind environment, Kubernetes runtime and worker resource.
- `topology.yaml`: versioned links between the two Kind resources.
- `dag.yaml`: two activities with a control dependency.
- `plan.yaml`: explicit schedule that places both activities on `kind-worker`.
- `run.yaml`: real execution request identity.
- `akoflow-access.yaml`: development ServiceAccount and cluster-wide binding.
- `requests/*.yaml`: composite plan and execution payloads ready for direct `curl` calls.
