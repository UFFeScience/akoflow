# External AkôFlow plan generators

This standalone Go module contains two plan generators. It is deliberately not
imported by the AkôFlow server and does not register either algorithm as an
in-process planning plugin.

- `memory-plan` implements the weighted AkôScore from the supplied paper. It
  rejects CPU/memory-infeasible placements and maximizes
  `alpha*(1/runtime) + (1-alpha)*((freeMemory-requiredMemory)/maximumMemory)`.
- `fifo-plan` keeps the stable workflow/topological arrival order and assigns
  each activity to the resource where it can start first. It never reorders
  ready activities based on duration, memory, or priority.

Both commands account for overlapping CPU and memory reservations, dependency
completion, shortest-path network transfer time, boot/container overhead,
resource speedup, and activity-resource runtime/memory profiles. Their output
is an AkôFlow import envelope that can be submitted unchanged to
`POST /akoflow-api/schedule-plans/import/`.

## Build and test

```bash
cd other-algorithms
go test ./...
go build ./cmd/memory-plan ./cmd/fifo-plan
```

## Generate from the AkôFlow API

The workflow flag takes the workflow definition ID, while the generated plan
references its current version ID returned by the API.

```bash
export AKOFLOW_API_TOKEN='...'

go run ./cmd/memory-plan \
  -api-url http://localhost:8080/akoflow-api \
  -scope-id fog-hpc-cloud-simulation-scope-v1 \
  -workflow-id WORKFLOW_DEFINITION_ID \
  -topology-id fog-hpc-cloud-network-v1 \
  -plan-id WORKFLOW_MEMORY_PLAN_V1 \
  -alpha 0.5 \
  -format yaml \
  -output memory-plan.yaml

go run ./cmd/fifo-plan \
  -api-url http://localhost:8080/akoflow-api \
  -scope-id fog-hpc-cloud-simulation-scope-v1 \
  -workflow-id WORKFLOW_DEFINITION_ID \
  -topology-id fog-hpc-cloud-network-v1 \
  -plan-id WORKFLOW_FIFO_PLAN_V1 \
  -format yaml \
  -output fifo-plan.yaml
```

## Generate from an offline bundle

The input accepts YAML or JSON using the same planning fields used by AkôFlow:

```yaml
workflow: { ... }
executionScope: { ... }
resources: [ ... ]
networkTopology: { ... }
activityProfiles: [ ... ]
deadlineSeconds: 0
budget: 0
```

Run either generator with `-input planning-input.yaml`. Standard input and
output are supported with `-input -` and `-output -`.

## Import a generated plan

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/yaml' \
  --data-binary @memory-plan.yaml \
  http://localhost:8080/akoflow-api/schedule-plans/import/
```

The generator cannot derive a plan from an execution scope alone: scheduling
also requires a workflow DAG and a network topology. API mode resolves all
resources belonging to the supplied scope automatically.
