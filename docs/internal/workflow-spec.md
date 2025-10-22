# AkôFlow Workflow Specification Documentation

This document provides a comprehensive guide to the structure and purpose of each field in an AkôFlow workflow definition file (YAML format). The workflow specification outlines how a workflow operates, including its containers, storage, and activities (steps).

---

## Overview

A workflow file includes:
- A workflow name
- Runtime configuration (e.g., container image, namespace, storage policy)
- A list of activities (tasks) executed sequentially or in parallel

### Example:

```yaml
name: wf-hello-world
spec:
    image: "alpine:latest"
    namespace: "akoflow"
    storageClassName: "hostpath"
    storageSize: "32Mi"
    storagePolicy:
        type: distributed
    mountPath: "/data"
    activities:
        - name: "a"
            memoryLimit: 500Mi
            cpuLimit: 0.5
            run: |
                ls -la
                echo "Hello from a"
                echo "Hello from a" > a.txt

        - name: "b"
            memoryLimit: 500Mi
            cpuLimit: 0.5
            run: |
                ls -la
                echo "Hello from b"
                echo "Hello from b" > b.txt
```

---

## Top-Level Fields

### `name`
- **Type**: string
- **Required**: ✅
- **Description**: A unique identifier for the workflow. It is used internally by AkôFlow for naming resources, jobs, and logs. Follow a lowercase-hyphenated convention (e.g., `wf-hello-world`).

---

### `spec`

The `spec` section defines runtime and execution details for the workflow. It determines how and where the workflow is executed.

---

#### `image`
- **Type**: string
- **Required**: ✅
- **Example**: `"alpine:latest"`
- **Description**: The Docker or OCI image used to run all activities in the workflow. Each activity runs inside a container created from this image.

**Tip**: Use any valid image name from Docker Hub or a private registry, such as:
- `python:3.11`
- `ubuntu:22.04`
- `ghcr.io/akoflow/base:1.0`

---

#### `namespace`
- **Type**: string
- **Default**: `"default"` (if omitted)
- **Description**: The Kubernetes namespace (or AkôFlow logical namespace) where the workflow runs. This isolates resources between environments (e.g., `akoflow`, `test`, `production`).

---

#### `storageClassName`
- **Type**: string
- **Default**: System default storage class
- **Description**: Specifies the storage class for provisioning persistent volumes (PVCs). Common values include:
    - `hostpath` → For local clusters or Kind
    - `nfs-client` → For distributed NFS setups
    - `gp2` or `standard` → For cloud-managed storage

---

#### `storageSize`
- **Type**: string
- **Default**: `"1Gi"`
- **Description**: The disk space allocated for the workflow’s data volume. Accepts Kubernetes size units like `Mi`, `Gi`, or `Ti`.

**Example**:
```yaml
storageSize: "32Mi"
```
This allocates a 32-MiB persistent volume.

---

#### `storagePolicy`
- **Type**: object
- **Description**: Defines how storage is provisioned or shared across workflow activities.

**Subfields**:
- **`type`**: string  
    - **Description**: The storage mode or policy. Options include:
        - `distributed`: Each node/activity uses its own copy of the data (useful for parallel tasks).
        - `shared`: All activities share the same volume (useful for data passing).

**Example**:
```yaml
storagePolicy:
    type: distributed
```

---

#### `mountPath`
- **Type**: string
- **Default**: `"/data"`
- **Description**: The path inside the container where the storage volume is mounted. All activities can read/write files in this path.

**Example**:
```yaml
mountPath: "/data"
```

---

## Activities

### `activities`
- **Type**: array
- **Required**: ✅
- **Description**: A list of steps (or activities) that the workflow executes. Each activity specifies resource limits and commands to run inside the container.

---

### Activity Fields

Each item in the `activities` list includes:

| Field        | Type             | Required | Description                                      |
|--------------|------------------|----------|--------------------------------------------------|
| `name`       | string           | ✅        | Unique name for the activity.                   |
| `memoryLimit`| string           | Optional  | Maximum memory allocated to the container (e.g., `500Mi`). |
| `cpuLimit`   | float            | Optional  | CPU quota assigned to the container (e.g., `0.5` = half a CPU). |
| `run`        | string (multiline)| ✅       | The shell script or command sequence to execute inside the container. |

---

### Example Explanation

#### Activity `a`
```yaml
- name: "a"
    memoryLimit: 500Mi
    cpuLimit: 0.5
    run: |
        ls -la
        echo "Hello from a"
        echo "Hello from a" > a.txt
```
- Runs a lightweight container (`alpine:latest`)
- Lists all files in the working directory
- Writes a file `a.txt` containing the text “Hello from a”
- Consumes up to 500 MiB of RAM and 0.5 CPU core

#### Activity `b`
```yaml
- name: "b"
    memoryLimit: 500Mi
    cpuLimit: 0.5
    run: |
        ls -la
        echo "Hello from b"
        echo "Hello from b" > b.txt
```
- Executes similar steps to `a`, but independently creates its own file `b.txt`.

---

## Execution Model
- By default, activities execute sequentially in the order they appear.
- Future versions will support `dependsOn` and parallel execution fields for DAG-style control.

---

## Example File Summary

| Section               | Purpose                                   |
|-----------------------|-------------------------------------------|
| `name`                | Identifies the workflow                  |
| `spec.image`          | Container base image                     |
| `spec.namespace`      | Logical namespace for execution          |
| `spec.storageClassName`| Defines persistent volume type           |
| `spec.storageSize`    | Defines disk capacity                    |
| `spec.storagePolicy`  | Controls how volumes are shared/distributed |
| `spec.mountPath`      | Where data is mounted inside the container |
| `activities`          | Defines workflow steps (commands, resources) |

---

## Notes
- All numeric resource values follow Kubernetes conventions.
- Mount paths should be absolute.
- When using distributed storage, ensure your provisioner supports multiple PVCs.
- Output files written to the mount path can be used by subsequent steps or stored as workflow artifacts.

---

Would you like to extend this into a versioned specification document (e.g., `akoflow-spec-v1.md`) with a formal YAML schema definition and validation rules?
