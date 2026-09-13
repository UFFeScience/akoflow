---
title: Connect a Kubernetes environment
description: Configure a Kubernetes runtime, credential, namespace, resources, storage, and validation path for real execution.
---

# Connect a Kubernetes environment

Use this guide to connect an existing Kubernetes cluster so AkôFlow can run container activities as Jobs. You need access to a namespace and its service account. The [Kind real-execution Showcase](../../showcase/kubernetes-real-execution) provides a local example to try before using a shared cluster.

Use this runtime for container workloads that must become Kubernetes Jobs. Do not use it to simulate a cluster: use [SimGrid](./simgrid) for modeled infrastructure. Do not put a bearer token in a workflow, plan, repository, or screenshot.

## Prerequisites

- A reachable Kubernetes API endpoint from the machine that runs the AkôFlow daemon.
- `kubectl` access as a cluster administrator only for the initial namespace and service-account setup.
- A namespace dedicated to AkôFlow-managed workloads.
- Images that every intended node can pull. The built-in runtime passes the activity image directly to Kubernetes; it does not configure registry credentials on your behalf.
- A storage class or shared storage mechanism when activities exchange files or need persisted artifacts.

## 1. Create a namespace and a least-privilege service account

The local Kind bundle uses `cluster-admin` only because it is disposable. In a shared cluster, create a namespace-scoped service account and grant only the API actions that AkôFlow uses for that namespace. The runtime creates, lists, gets, and deletes Jobs, Pods, Services, and PersistentVolumeClaims; it reads Pod logs. Workspace transfer and the interactive console also create Pods and use the Pod `exec` subresource. Discovery additionally lists cluster-scoped Nodes.

Start with this namespace-scoped baseline, then add only the cluster-scoped `nodes` read permission if you will use discovery:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: akoflow
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: akoflow-runtime
  namespace: akoflow
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: akoflow-runtime
  namespace: akoflow
rules:
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["create", "get", "list", "delete"]
  - apiGroups: [""]
    resources: ["pods", "services", "persistentvolumeclaims"]
    verbs: ["create", "get", "list", "delete"]
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get"]
  - apiGroups: [""]
    resources: ["pods/exec"]
    verbs: ["create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: akoflow-runtime
  namespace: akoflow
subjects:
  - kind: ServiceAccount
    name: akoflow-runtime
    namespace: akoflow
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: akoflow-runtime
```

Apply it, then verify the namespace permissions before registering the connection:

```bash
kubectl apply -f akoflow-runtime-rbac.yaml
kubectl auth can-i --as=system:serviceaccount:akoflow:akoflow-runtime create jobs -n akoflow
kubectl auth can-i --as=system:serviceaccount:akoflow:akoflow-runtime get pods/log -n akoflow
```

For node discovery, a separate `ClusterRole` and `ClusterRoleBinding` granting `get,list` on `nodes` is required. Keep it separate from the namespace role so an execution-only token does not automatically gain cluster inventory access.

## 2. Store the API credential outside the environment definition

Complete [API connection setup](../../tutorials/api-access). Generate a short-lived Kubernetes token and stream it to the credential endpoint without placing it in a command argument or a temporary file. Run this in Bash so `pipefail` catches a failed token request:

```bash
set -o pipefail
kubectl -n akoflow create token akoflow-runtime --duration=1h \
  | jq -R '{id:"research-kubernetes",token:.}' \
  | curl --fail-with-body \
      -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
      -H 'Content-Type: application/json' --data-binary @- \
      "$AKOFLOW_API_URL/kubernetes-tokens/" -o kubernetes-reference.json || exit 1
jq -e '.credentialRef' kubernetes-reference.json
```

Use the returned `credentialRef` in the connection you register. The Kubernetes client accepts `file:<path>` or `env:<variable>` references, or a `bearerToken` in connection configuration. The daemon-managed file reference keeps the token out of the versioned environment YAML.

For a production cluster, provide the API server certificate through `configuration.caFile`. `insecureSkipTlsVerify: true` is appropriate for the disposable Kind example only; do not copy it to a trusted cluster configuration.

## 3. Register the connection, runtime, and resources

In Desktop, open **Infrastructure → Environments**, create a real Kubernetes environment, add the connection, test it, and run discovery. Create or confirm a Kubernetes runtime whose configuration names the connection and namespace. Bind that runtime to every schedulable Kubernetes machine.

The relevant part of `examples/kind/environment.yaml` is:

```yaml title="Connection and resource excerpt from environment.yaml"
connections:
  - id: kind-akoflow-connection
    type: kubernetes
    endpoint: https://host.docker.internal:63395
    credentialRef: file:storage/credentials/kubernetes/kind-akoflow.token
    configuration:
      namespace: akoflow
      insecureSkipTlsVerify: true

runtimes:
  - id: kind-kubernetes
    driver: kubernetes
    mode: execution
    configuration:
      namespace: akoflow
      connectionId: kind-akoflow-connection
    capabilities:
      batch: true
      container: true
      cancellation: true

resources:
  - id: kind-worker
    type: kubernetes_machine
    providerId: akoflow-worker
    cpuCores: 4
    cpuCapacity: 4
    schedulable: true
    metadata:
      kubernetesNode: akoflow-worker

resourceRuntimeBindings:
  - resourceId: kind-worker
    runtimeId: kind-kubernetes
    enabled: true
```

`namespace` on the runtime takes precedence over the connection namespace. The runtime requires both an endpoint and a bearer token. Its connection health check lists Pods in the selected namespace; discovery lists Nodes and reports their readiness, allocatable CPU/memory, architecture, and container runtime information.

For a manually maintained resource, set `metadata.kubernetesNode` or `providerId` to the node's exact `kubernetes.io/hostname` value. AkôFlow emits that value as a Pod `nodeSelector`, so a plan that selects the resource is actually placed on the modeled node. An API URL is not a node name and must not be used as `providerId` for a Kubernetes-machine resource.

## 4. Attach storage and make images available

Use a PVC or NFS storage resource when the workflow needs a shared workspace. The Kind example defines a PVC and its runtime mount:

```yaml title="Storage excerpt from environment.yaml"
storages:
  - id: kind-akoflow-data
    type: pvc
    endpoint: pvc://akoflow/akoflow-data
    shared: true
    readOnly: false
    configuration:
      namespace: akoflow
      claimName: akoflow-data
      mountPath: /akoflow/data
    runtimeBindings:
      - runtimeId: kind-kubernetes
        default: true
        containerPath: /akoflow/data
```

The activity-level storage metadata identifies the claim and mount path. At run time, AkôFlow mounts the volume and records artifacts under a run/activity path. A PVC with `ReadWriteOnce` can only mount on one node at a time; when a plan pins activities or transfer pods to different nodes, select a storage class/access mode that supports that topology, or keep the relevant placement co-located.

Before launching a production workflow, verify image pull access from the selected namespace and nodes. The current runtime does not add `imagePullSecrets` to generated Pods. Configure image access at the cluster, service-account, or admission-policy layer before registering private images with AkôFlow.

## 5. Validate the connection and run a minimal workflow

In Desktop, test the connection, run discovery, inspect the resource inventory, then import a small workflow. Create a scope containing the environment version, generate or create a plan, choose **Real execution**, and inspect the completed run's activity logs and artifacts.

For an equivalent API validation, follow the complete [Kind README](https://github.com/UFFeScience/akoflow/tree/v1.0.8/examples/kind). It applies the cluster access and PVC, stores a short-lived token, and submits the environment, scope, topology, workflow, plan, and execution request in that order.

The exact Kind bundle completed on 2026-09-11 as `kind-dag-run-v8`. It created two Kubernetes Jobs, transferred 9 bytes through its workspace, and produced matching `result.txt` and `consumed.txt` files with checksum `sha256:cb064c1339ffa3d7777bcb0459de3dceddb9146156dde58065a4ac826b029aa7`.

## Troubleshoot and clean up

| Symptom | Check and recover |
| --- | --- |
| Health check says the namespace is unreachable | Verify the endpoint is reachable from the daemon—not merely from your laptop—and confirm the token can list Pods in the configured namespace. |
| Discovery cannot find nodes | Grant the separate cluster-scoped `nodes` read permission, then run discovery again. |
| A Job remains `Pending` | Run `kubectl describe pod <pod>` and check image pulling, requests, taints, the selected node, and PVC/node affinity. |
| Job creation returns forbidden | Compare the denied Kubernetes resource and verb with the namespace role above. Add only that missing permission. |
| Files are absent from the consumer | Check the storage binding, the producer's artifact path, and the storage access mode. For a PVC, confirm the node chosen by the plan is compatible with the claim. |
| Interactive console cannot start | The runtime must be able to create/get/delete Pods, and the daemon host must have `kubectl` available for the current interactive-console implementation. |

AkôFlow labels generated Jobs, Services, PVCs, and console Pods with `app.kubernetes.io/managed-by: akoflow`. Use that label to inspect resources before cleanup:

```bash
kubectl -n akoflow get jobs,pods,services,pvc \
  -l app.kubernetes.io/managed-by=akoflow
```

Delete only the run resources you intend to remove. For the disposable Kind environment, use the Showcase cleanup command: `kind delete cluster --name akoflow`.

Related material: [Kubernetes real execution](../../showcase/kubernetes-real-execution), [execution scopes](./execution-scopes), [storage](./storage), and [interactive console](../operations/interactive-console).
