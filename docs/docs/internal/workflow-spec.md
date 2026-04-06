---
id: workflow-spec
title: Workflow Specification
sidebar_label: Workflow Spec
---

This document describes every field in an AkôFlow workflow YAML file. A workflow definition tells AkôFlow what to run, where to run it, how much resources to allocate, and how tasks depend on each other.

---

## Complete example

```yaml
name: etl-pipeline

spec:
  image: "python:3.11"
  namespace: "akoflow"
  runtime: "k8s-aws"
  schedule: "memory-optimized"

  storagePolicy:
    type: distributed
    storageClassName: "gp2"
    storageSize: "10Gi"

  mountPath: "/data"
  volumes:
    - "/local/datasets"

  activities:
    - name: "ingest"
      cpuLimit: 0.5
      memoryLimit: 512Mi
      run: |
        python ingest.py --output /data/raw

    - name: "deduplicate"
      dependsOn: ["ingest"]
      cpuLimit: 2.0
      memoryLimit: 4Gi
      keepDisk: true
      run: |
        python dedup.py --input /data/raw --output /data/clean

    - name: "predict-us"
      dependsOn: ["deduplicate"]
      runtime: "k8s-aws"
      nodeSelector: "region=us-east-1"
      cpuLimit: 4.0
      memoryLimit: 8Gi
      run: |
        python predict.py --region us --input /data/clean

    - name: "predict-eu"
      dependsOn: ["deduplicate"]
      runtime: "k8s-gcp"
      nodeSelector: "region=europe-west1"
      cpuLimit: 4.0
      memoryLimit: 8Gi
      run: |
        python predict.py --region eu --input /data/clean

    - name: "aggregate"
      dependsOn: ["predict-us", "predict-eu"]
      cpuLimit: 0.5
      memoryLimit: 512Mi
      run: |
        python aggregate.py --output /data/final
```

---

## Top-level fields

### `name`

- **Type**: string
- **Required**: yes
- **Description**: Unique identifier for the workflow. Used internally for naming resources, jobs, and logs.
- **Convention**: lowercase-hyphenated, e.g., `wf-my-pipeline`

---

## `spec` block

The `spec` section defines the execution context for the entire workflow.

---

### `spec.image`

- **Type**: string
- **Required**: yes
- **Description**: Default container image for all activities. Each activity runs inside a container built from this image.
- **Examples**: `python:3.11`, `ubuntu:22.04`, `ghcr.io/myorg/myimage:1.0`

Activities can override this by specifying their own `image` field.

---

### `spec.namespace`

- **Type**: string
- **Default**: `"akoflow"`
- **Description**: The Kubernetes namespace (or AkôFlow logical namespace) where the workflow runs. Isolates resources between environments or teams.

---

### `spec.runtime`

- **Type**: string
- **Default**: engine's default runtime
- **Description**: The default runtime for all activities. Determines which infrastructure backend executes tasks.

Available built-in values:

| Value | Backend |
|---|---|
| `k8s` | Kubernetes (any cluster) |
| `k8s-{name}` | Named Kubernetes cluster (e.g., `k8s-aws`) |
| `hpc-{name}` | Named HPC/SLURM cluster (e.g., `hpc-sdumont`) |
| `singularity` | Singularity on local machine |
| `local` | Bare shell process on the local machine |
| `docker` | Docker (stub, not yet fully implemented) |

Individual activities can override this with their own `runtime` field — enabling cross-environment execution within a single workflow.

→ See [Runtimes](../runtimes) for a full explanation of each backend and its environment variables.

---

### `spec.schedule`

- **Type**: string
- **Default**: engine's default AkôScore policy
- **Description**: Name of the scheduling policy to apply when assigning tasks to nodes. References a registered schedule (plugin) in the engine.

If omitted, the engine uses its built-in AkôScore function (balancing makespan and memory utilization).

```yaml
spec:
  schedule: "memory-optimized"   # custom Go plugin registered in the engine
```

→ See [Concepts — AkôScore](../concepts#akôscore--the-scheduling-function) for details on writing custom scheduling policies.

---

### `spec.storagePolicy`

- **Type**: object
- **Description**: Controls how persistent storage is provisioned for the workflow.

**Subfields:**

#### `storagePolicy.type`

- **Type**: string
- **Values**: `distributed` | `standalone`
- **Description**:
  - `distributed` — each Worker node gets its own independent volume; tasks cannot read each other's files directly
  - `standalone` — all nodes share a single volume; tasks can read outputs produced by other tasks without additional coordination

#### `storagePolicy.storageClassName`

- **Type**: string
- **Description**: Kubernetes StorageClass for PVC provisioning. Common values:
  - `hostpath` → local clusters or Kind
  - `nfs-client` → distributed NFS
  - `gp2` or `standard` → cloud-managed storage (AWS, GCP)

#### `storagePolicy.storageSize`

- **Type**: string
- **Default**: `"1Gi"`
- **Description**: Disk capacity allocated for the workflow's data volume. Uses Kubernetes size notation: `Mi`, `Gi`, `Ti`.

---

### `spec.mountPath`

- **Type**: string
- **Default**: `"/data"`
- **Description**: The path inside the container where the storage volume is mounted. All activities read and write files through this path.

---

### `spec.volumes`

- **Type**: array of strings
- **Description**: Local paths on the Engine host to synchronize with the execution environment before workflow starts. Used primarily by the HPC runtime to `rsync` data to the remote cluster before job submission.

```yaml
volumes:
  - "/local/datasets/customers.csv"
  - "/local/models/baseline"
```

---

## `activities` block

### `activities`

- **Type**: array
- **Required**: yes
- **Description**: The ordered list of tasks in the workflow. AkôFlow constructs a DAG from the `dependsOn` fields and executes tasks as their dependencies complete.

---

## Activity fields

Each item in `activities` supports the following fields:

---

### `name`

- **Type**: string
- **Required**: yes
- **Description**: Unique identifier for the activity within the workflow. Used in `dependsOn` references, logs, and provenance records.

---

### `run`

- **Type**: multiline string
- **Required**: yes
- **Description**: Shell commands to execute inside the container. Written as a bash script. The container's working directory is set to `mountPath`.

```yaml
run: |
  echo "starting"
  python train.py --epochs 10 --output /data/model
  echo "done"
```

---

### `image`

- **Type**: string
- **Default**: inherits `spec.image`
- **Description**: Override the container image for this specific activity. Useful when different tasks need different runtime environments.

```yaml
- name: "gpu-step"
  image: "pytorch/pytorch:2.0-cuda11.7"
  run: python train_gpu.py
```

---

### `runtime`

- **Type**: string
- **Default**: inherits `spec.runtime`
- **Description**: Override the execution runtime for this activity. Enables cross-environment workflows where different tasks run on different infrastructure.

```yaml
- name: "eu-processing"
  runtime: "k8s-gcp-europe"
  run: python process_eu.py
```

---

### `dependsOn`

- **Type**: array of strings
- **Default**: `[]` (no dependencies — runs immediately)
- **Description**: Names of activities that must reach `Finished` state before this activity becomes `Ready`. This field defines the DAG structure.

```yaml
- name: "aggregate"
  dependsOn: ["predict-us", "predict-eu"]   # waits for both
```

Activities not listed in any `dependsOn` run in parallel from the start.

---

### `memoryLimit`

- **Type**: string
- **Default**: no limit
- **Description**: Maximum memory the container may use. Uses Kubernetes notation: `128Mi`, `1Gi`, `4Gi`. The AkôScore scheduler uses this value to find nodes with sufficient free memory.

Exceeding this limit may cause the container to be OOM-killed by the runtime.

---

### `cpuLimit`

- **Type**: float
- **Default**: no limit
- **Description**: Maximum CPU cores the container may use. `0.5` means half a core; `2.0` means two cores. The AkôScore scheduler uses this value to check CPU feasibility on candidate nodes.

---

### `nodeSelector`

- **Type**: string
- **Default**: none
- **Description**: Constrains task execution to nodes matching a label or property. The format and semantics depend on the runtime:
  - For Kubernetes: matches Kubernetes node labels (e.g., `region=us-east-1`, `gpu=true`)
  - For HPC: may match SLURM node properties or partition constraints

```yaml
- name: "gpu-training"
  nodeSelector: "accelerator=nvidia-tesla-v100"
```

---

### `keepDisk`

- **Type**: boolean
- **Default**: `false`
- **Description**: When `true`, the activity's storage volume is not deleted after the workflow completes. Useful for preserving intermediate results for inspection or reuse in subsequent runs.

```yaml
- name: "expensive-preprocessing"
  keepDisk: true
  run: python preprocess.py   # results kept for debugging
```

---

## Activity lifecycle fields (read-only)

These fields are set and updated by the engine during execution. They are visible in API responses and provenance records but should not be set in the workflow YAML.

| Field | Description |
|---|---|
| `status` | Current state: `Pending`, `Ready`, `Running`, `Finished`, `Failed` |
| `procId` | OS-level process ID or SLURM job ID assigned by the runtime |
| `createdAt` | Timestamp when the activity was created in the database |
| `startedAt` | Timestamp when the activity transitioned to `Running` |
| `finishedAt` | Timestamp when the activity transitioned to `Finished` or `Failed` |

---

## Field summary table

| Field | Scope | Required | Type | Description |
|---|---|---|---|---|
| `name` | Workflow | yes | string | Workflow identifier |
| `spec.image` | Workflow | yes | string | Default container image |
| `spec.namespace` | Workflow | no | string | Logical namespace |
| `spec.runtime` | Workflow | no | string | Default execution runtime |
| `spec.schedule` | Workflow | no | string | Custom AkôScore policy name |
| `spec.storagePolicy.type` | Workflow | no | string | `distributed` or `standalone` |
| `spec.storagePolicy.storageClassName` | Workflow | no | string | Kubernetes StorageClass |
| `spec.storagePolicy.storageSize` | Workflow | no | string | Volume size (e.g., `1Gi`) |
| `spec.mountPath` | Workflow | no | string | Mount path inside containers |
| `spec.volumes` | Workflow | no | string[] | Local paths to sync to runtime |
| `activities[].name` | Activity | yes | string | Unique activity name |
| `activities[].run` | Activity | yes | string | Shell commands to execute |
| `activities[].image` | Activity | no | string | Override container image |
| `activities[].runtime` | Activity | no | string | Override runtime |
| `activities[].dependsOn` | Activity | no | string[] | Names of prerequisite activities |
| `activities[].memoryLimit` | Activity | no | string | Max memory (e.g., `4Gi`) |
| `activities[].cpuLimit` | Activity | no | float | Max CPU cores (e.g., `2.0`) |
| `activities[].nodeSelector` | Activity | no | string | Node constraint |
| `activities[].keepDisk` | Activity | no | boolean | Preserve storage after completion |
