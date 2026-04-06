---
id: runtimes
title: Runtimes
sidebar_label: Runtimes
---

A **runtime** in AkôFlow is the execution backend where containerized tasks run. Each runtime adapter implements the same interface but connects to a different infrastructure. This lets the same workflow specification execute on a laptop, a Kubernetes cluster, or an HPC supercomputer — just by changing the `runtime` field in the YAML.

---

## The runtime interface

Every runtime adapter implements this contract:

```go
type IRuntime interface {
    StartConnection() error
    StopConnection() error

    ApplyJob(workflowID int, activityID int) bool     // launch a task
    DeleteJob(workflowID int, activityID int) bool    // cancel a task

    GetMetrics(workflowID int, activityID int) string // collect resource usage
    GetLogs(workflow, activity) string                // collect stdout/stderr

    GetStatus(workflowID int, activityID int) string
    VerifyActivitiesWasFinished(workflow) bool        // check task completion

    HealthCheck() bool                               // ping the backend
}
```

The **Worker goroutine** calls `ApplyJob`. The **Monitor goroutine** calls `VerifyActivitiesWasFinished`, `GetMetrics`, and `GetLogs`. The **HealthCheck goroutine** calls `HealthCheck`.

---

## Runtime selection

The runtime to use for a task is determined by the `runtime` field in the workflow YAML. It is resolved by name at scheduling time:

```go
// pkg/server/runtimes/runtime.go
const (
    RUNTIME_K8S         = "k8s"
    RUNTIME_DOCKER      = "docker"
    RUNTIME_SINGULARITY = "singularity"
    RUNTIME_HPC         = "hpc"
    RUNTIME_LOCAL       = "local"
)
```

Names starting with `k8s` resolve to the Kubernetes adapter; names starting with `hpc` resolve to the HPC adapter. This prefix matching allows you to name runtimes descriptively — e.g., `k8s-aws-us-east-1` or `hpc-sdumont` — while still mapping to the correct adapter.

---

## Kubernetes runtime (`k8s`)

### What it is

The Kubernetes runtime submits tasks as **Kubernetes Jobs** against a remote cluster API. It is the primary runtime for cloud deployments (AWS EKS, GCP GKE, on-premise K8s clusters).

### How it connects

```
AkôFlow Engine
│
├─ HealthCheck goroutine (every 5s)
│   ├─ GET {K8S_API_SERVER_HOST}/healthz            → check cluster health
│   ├─ GET /apis/metrics.k8s.io/v1beta1/nodes       → collect node CPU/mem
│   └─ GET /api/v1/nodes                            → discover worker nodes
│
├─ Worker goroutine (on task dispatch)
│   ├─ POST /api/v1/namespaces/{ns}/persistentvolumeclaims  → create PVC
│   ├─ POST /apis/batch/v1/namespaces/{ns}/jobs             → create Job (task container)
│   └─ POST /api/v1/namespaces/{ns}/serviceaccounts         → RBAC setup
│
└─ Monitor goroutine (every 1s)
    ├─ GET /apis/batch/v1/namespaces/{ns}/jobs/{name}        → check completion
    └─ GET /api/v1/namespaces/{ns}/pods/{name}/log           → collect logs
```

The connector communicates with the Kubernetes API over **HTTPS with TLS** (certificate verification is currently disabled to support self-signed certificates in private clusters). Authentication is via a **Bearer token**.

### Environment variables

| Variable | Description | Example |
|---|---|---|
| `K8S_API_SERVER_HOST` | URL of the Kubernetes API server | `https://1.2.3.4:6443` |
| `K8S_API_SERVER_TOKEN` | Bearer token for authentication | `eyJhbG...` |

For named runtimes (e.g., `k8s-aws`), prefix the variable with the runtime name:

| Variable | Description |
|---|---|
| `K8S_AWS_API_SERVER_HOST` | API server URL for runtime `k8s-aws` |
| `K8S_AWS_API_SERVER_TOKEN` | Token for runtime `k8s-aws` |

The convention is: `{RUNTIME_NAME}_{KEY}` in uppercase. The runtime entity resolves keys using this pattern automatically.

### Storage modes

The Kubernetes runtime supports two storage modes, controlled by `storagePolicy.type` in the YAML:

- **`distributed`** — each Worker node gets its own PVC; tasks write locally, data is not shared between nodes
- **`standalone`** — a single shared PVC is mounted on all nodes; tasks can read each other's outputs directly

---

## Singularity runtime (`singularity`)

### What it is

The Singularity runtime runs tasks as **local Singularity container instances** on the same machine as the Engine. It is designed for HPC environments where Docker is unavailable (due to security restrictions) but Singularity is installed.

### How it connects

Unlike Kubernetes, the Singularity connector does not talk to a remote API. It **executes shell commands directly** on the local host:

```
AkôFlow Engine (running on HPC login node or equivalent)
│
├─ Worker goroutine (on task dispatch)
│   └─ exec: singularity run --bind /data:/data {image} {command}
│       → returns PID of spawned process
│
└─ Monitor goroutine (every 1s)
    └─ exec: bash akf_monitor_singularity.sh {pid}
        ├─ reports TOTAL_CPU=(...%) TOTAL_MEM=(...%)
        ├─ reports ##START_LOG_OUTPUT## ... ##END_LOG_OUTPUT##
        └─ reports #NO_PROCESS_FOUND  (when process ends)
```

The monitoring script (`akf_monitor_singularity.sh`) is a bash script embedded in the engine that polls the process by PID using standard UNIX tools. When the process is no longer found, the Monitor marks the task as `Finished`.

### Environment variables

The Singularity runtime does not require remote connection variables — it runs locally. No `SINGULARITY_*` environment variables are needed for the connector itself.

---

## HPC runtime (`hpc`)

### What it is

The HPC runtime submits tasks to a **SLURM-managed HPC cluster** via SSH. It is the runtime for national supercomputers and institutional HPC systems. The Engine does not need to run on the HPC cluster itself — it connects remotely.

### How it connects

```
AkôFlow Engine (running anywhere with VPN/network access)
│
├─ HealthCheck goroutine (every 5s)
│   ├─ check: VPN connected?
│   └─ ssh {USER}@{HOST_CLUSTER}: sinfo -p {QUEUE}
│
├─ Worker goroutine (on task dispatch)
│   ├─ rsync: sync data volumes to HPC (local → remote mount path)
│   ├─ ssh: mkdir -p {MOUNT_PATH}
│   └─ ssh: sbatch {sbatch_script}
│       → sbatch submits a Singularity-wrapped job to SLURM
│       → returns SLURM Job ID (stored as ProcId)
│
└─ Monitor goroutine (every 1s)
    └─ ssh: cat {MOUNT_PATH}/akoflow_finished_{wf}_{activity}.txt
        └─ if file contains "AKOFLOW_JOB_FINISHED" → mark Finished
           (the Singularity container writes this sentinel file on completion)
```

The HPC runtime wraps tasks as **Singularity containers submitted via sbatch**. Volume synchronization uses `rsync` over SSH. Completion detection uses a sentinel file written by the container at the end of execution.

### VPN requirement

The HPC runtime checks for an active VPN connection before attempting any remote commands. If the VPN is not connected, the task is skipped until the next Monitor cycle.

### Environment variables

Env vars follow the `{RUNTIME_NAME}_{KEY}` convention. For a runtime named `hpc-sdumont`:

| Variable | Description | Example |
|---|---|---|
| `HPC_SDUMONT_USER` | SSH username | `myuser` |
| `HPC_SDUMONT_HOST_CLUSTER` | Hostname of the HPC login node | `sdumont.lncc.br` |
| `HPC_SDUMONT_QUEUE` | SLURM partition/queue name | `sequana_gpu` |
| `HPC_SDUMONT_MOUNT_PATH` | Working directory on the HPC node | `/scratch/myuser/akoflow` |
| `HPC_SDUMONT_GATEWAY` | VPN gateway (if applicable) | `vpn.lncc.br` |
| `HPC_SDUMONT_GROUP` | SLURM account/group | `mygroup` |
| `HPC_SDUMONT_PROJECT` | Project identifier | `myproject` |

SSH key management: the connector supports base64-encoded private key, public key, and SSH config injection via metadata, enabling keyless authentication setup at runtime.

---

## Local runtime (`local`)

### What it is

The Local runtime executes tasks as **plain shell processes** on the same machine as the Engine, without any container isolation. It is intended for lightweight development workflows where containerization overhead is undesirable.

### How it connects

```
AkôFlow Engine
│
└─ Worker goroutine (on task dispatch)
    └─ exec: bash -c "{task commands}"
        → starts subprocess, returns PID
        → Monitor polls via akf_monitor script (same as Singularity)
```

The Local connector uses `os/exec` to spawn a subprocess with `SysProcAttr` set so the process is detached. The PID is stored as `ProcId` and monitored by the Monitor goroutine.

### Environment variables

None required for the connector. The runtime reads the standard `LOCAL_*` prefixed variables if configured.

---

## Docker runtime (`docker`)

The Docker runtime (`docker`) is present in the codebase as an adapter stub. It is intended for Docker-native execution without Kubernetes, but is not yet fully implemented. Use the `local` or `k8s` runtimes for production workloads.

---

## Environment variable conventions

All runtime configuration is passed through **environment variables** following a consistent naming pattern:

```
{RUNTIME_NAME}_{KEY}
```

Where `RUNTIME_NAME` is the normalized name prefix (e.g., `K8S`, `HPC`, `SINGULARITY`) and `KEY` is the specific configuration parameter.

The engine reads all environment variables at startup, groups them by runtime prefix, and stores them in the runtime's metadata map. The `runtime_entity.GetCurrentRuntimeMetadata(key)` method resolves a key for a given runtime:

```go
// For runtime named "k8s-aws", looking up "API_SERVER_HOST":
// → looks for "K8S_AWS_API_SERVER_HOST" in environment
func (r *Runtime) GetCurrentRuntimeMetadata(key string) string {
    lookupKey := strings.ToUpper(r.Name + "_" + key)
    return r.Metadata[lookupKey]
}
```

This means you can configure multiple clusters of the same type by giving them distinct names:

```bash
# Two Kubernetes clusters
K8S_AWS_API_SERVER_HOST=https://aws-cluster:6443
K8S_AWS_API_SERVER_TOKEN=eyJ...

K8S_GCP_API_SERVER_HOST=https://gcp-cluster:6443
K8S_GCP_API_SERVER_TOKEN=eyJ...

# Reference in workflow YAML:
# runtime: "k8s-aws"   or   runtime: "k8s-gcp"
```

---

## Runtime selection in workflow YAML

Each **activity** can target a specific runtime:

```yaml
name: cross-cloud-pipeline
spec:
  image: "python:3.11"
  activities:
    - name: "process-us-data"
      runtime: "k8s-aws"        # runs on AWS Kubernetes cluster
      memoryLimit: 4Gi
      cpuLimit: 2.0
      run: |
        python process.py --region us

    - name: "process-eu-data"
      runtime: "k8s-gcp"        # runs on GCP Kubernetes cluster
      memoryLimit: 4Gi
      cpuLimit: 2.0
      run: |
        python process.py --region eu

    - name: "train-model"
      runtime: "hpc-sdumont"     # runs on HPC cluster via SLURM
      dependsOn: ["process-us-data", "process-eu-data"]
      memoryLimit: 32Gi
      cpuLimit: 16.0
      run: |
        python train.py --data /scratch/data --output /scratch/model
```

If no `runtime` is specified for an activity, it inherits the workflow-level runtime (or the default configured runtime).

---

## Runtime health states

Each runtime has a status in the database, managed by the HealthCheck goroutine:

| Status | Meaning |
|---|---|
| `READY` | Runtime responded to health check; nodes are available for scheduling |
| `NOT_READY` | Health check failed; AkôScore will not assign tasks to nodes in this runtime |

The HealthCheck runs every 5 seconds, so a runtime that recovers from a failure will be re-enabled within 5 seconds.
