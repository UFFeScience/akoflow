---
title: Define a workflow
---

# Define a workflow

A workflow lists the activities in a scientific computation and the order in which they run. AkôFlow saves versions of that definition, so a plan or past run always points to the workflow version it used.

You can create one in Desktop or import portable YAML. AkôFlow assigns IDs and converts the resource limits in the imported definition when it saves the first version.

## The activity model

An activity has a `kind`, one or more `capabilities`, a command, resource requirements, a retry/timeout policy, and optional simulation or service settings.

| Field | Meaning |
| --- | --- |
| `kind` | `task`, `service`, or `interactive` in the persisted model. Portable workflow imports currently create `task` activities. |
| `capabilities` | The modes the activity supports: `real`, `simulation`, or `interactive`. |
| `command` | Executable reference, entrypoint, arguments, environment, and working directory. |
| `resources` | Normalized CPU, memory, storage, and optional GPU demand. In portable input, use `cpuLimit` and `memoryLimit`. |
| `simulation` | Model, duration, FLOPs, and optional parameters. Supplying it makes a portable activity simulation-capable. |
| `policy` | Timeout, maximum attempts, and retry delay in the persisted model. |
| `dependsOn` | Control dependencies, written with activity names in portable input. |
| `dataDependencies` | Producer-to-consumer data edges with a logical name and byte size. |

For real execution, an activity needs `command.entrypoint` and `command.executable`. An executable can point to an OCI image or another supported artifact source and includes a delivery strategy. The legacy `spec.image`, activity `image`, and `run` shorthands are still accepted, but new definitions should prefer `command` and `command.executable`.

## Using AkôFlow Desktop

1. Open **Workflows → Workflow definitions**.
2. Choose **Create workflow**.
3. Enter the workflow name and namespace.
4. Add activities. For each activity, select an executable artifact, enter the command, CPU and memory limits, and comma-separated predecessor names.
5. Check the live DAG preview and create the workflow.
6. Open the definition to inspect its activities and dependencies. From the definition page you can also export or duplicate it.

<img src={require('@site/static/img/interface/workflows/definitions.png').default} alt="AkôFlow Desktop Workflow definitions catalog showing Import YAML, Create workflow, versioned workflow rows, and activity counts." />

*The catalog is the entry point for either path: use **Import YAML** for a portable definition or **Create workflow** to enter activities in Desktop. After creation, open the row to inspect the immutable version and its DAG.*

The import action accepts the same portable YAML format as the API. Export removes generated IDs and resolved runtime paths so that the result can be imported as a new definition.

## Using the API

Complete [API connection setup](../../tutorials/api-access) before running these commands.

The checked-in SimGrid example uses legacy shorthand. To try the portable simulation fields directly, save this as `workflow.yaml`. The empty `command` means these activities are simulation-only; a real run needs an executable and entrypoint as described in the [workflow specification](../../internal/workflow-spec).

```yaml
name: portable-simulation-demo
spec:
  namespace: examples
  activities:
    - name: prepare
      command: {}
      cpuLimit: "1"
      memoryLimit: 128Mi
      simulation:
        model: deterministic
        durationSeconds: 4
    - name: analyze
      command: {}
      cpuLimit: "2"
      memoryLimit: 1Gi
      dependsOn: [prepare]
      simulation:
        model: deterministic
        durationSeconds: 12
```

Create or import it:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/yaml' \
  --data-binary @workflow.yaml \
  "$AKOFLOW_API_URL/workflow-definitions/"
```

Useful definition operations are:

```bash
# List definitions
curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/workflow-definitions/"

# Read one definition
curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/workflow-definitions/portable-simulation-demo/"

# Export portable YAML
curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -o exported-workflow.yaml \
  "$AKOFLOW_API_URL/workflow-definitions/portable-simulation-demo/export/"
```

The create response contains the saved definition. Use its `version.id` when creating a planning session.

## Next step

Once the workflow, execution scope, resources, and network topology exist, [create a planning session](./planning.md).
