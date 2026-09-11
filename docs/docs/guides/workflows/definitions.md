---
title: Workflow definitions
---

# Workflow definitions

A workflow definition is the reusable description of a scientific computation. AkôFlow stores a stable definition and an immutable, versioned graph of activities. Plans and runs refer to the workflow **version ID**, so a past execution remains traceable to the graph that produced it.

The authoring format is intentionally smaller than the persisted domain model. AkôFlow normalizes activity names into IDs, creates the first workflow version, expands dependencies, converts CPU and memory limits, and records execution capabilities.

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

<!-- screenshot: Workflow definitions list with the Create workflow and Import actions numbered -->

<!-- screenshot: Create workflow form with definition fields, executable selector, activity fields, and DAG preview annotated -->

<!-- screenshot: Workflow detail showing immutable version, activity graph, and export/duplicate actions -->

The import action accepts the same portable YAML format as the API. Export removes generated IDs and resolved runtime paths so that the result can be imported as a new definition.

## Using the API

Set the API address and, when API authentication is enabled, its bearer token:

```bash
export AKOFLOW_API_URL="http://127.0.0.1:<port>/akoflow-api"
export AKOFLOW_API_TOKEN="<token>"
```

The simulation example in `examples/simulation/workflow.yaml` uses the legacy shorthand. This equivalent command-oriented definition shows the preferred portable shape:

```yaml
name: simulation-example-workflow
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
  "$AKOFLOW_API_URL/workflow-definitions/simulation-example-workflow/"

# Export portable YAML
curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -o exported-workflow.yaml \
  "$AKOFLOW_API_URL/workflow-definitions/simulation-example-workflow/export/"
```

The create response is the normalized `WorkflowDefinition`. Use `version.id` from that response when creating a planning session.

## Next step

Once the workflow, execution scope, resources, and network topology exist, [create a planning session](./planning.md).
