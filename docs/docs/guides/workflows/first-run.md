---
title: Your first workflow run
---

# Your first workflow run

This walkthrough follows the smallest complete AkôFlow lifecycle: register infrastructure, define a workflow, produce a plan, run it, and compare the result with the prediction. Use the simulation example for a deterministic first run that does not dispatch scientific commands to real infrastructure.

The checked-in source files are under `examples/simulation/`:

- `environment.yaml`: a simulation environment, runtime, and resources;
- `scope.yaml`: the environment versions allowed in the run;
- `topology.yaml`: network links between resources;
- `workflow.yaml`: a three-activity dependency chain;
- `plan-request.yaml`: a complete imported/manual planning envelope;
- `execution-request.yaml`: a complete simulation execution envelope.

## Using AkôFlow Desktop

### 1. Register the simulation infrastructure

Open **Infrastructure → Environments**, choose the simulation environment flow, and define the simulation runtime and resources. Then create an execution scope containing that environment version and a network topology for the scope.

<!-- screenshot: Simulation environment form with runtime and two resources annotated -->

<!-- screenshot: Execution scope and matching network topology, with their relationship highlighted -->

### 2. Create the workflow

Open **Workflows → Workflow definitions** and import `examples/simulation/workflow.yaml`, or create the `prepare → analyze → summarize` graph in the form. Open the result and verify the activity DAG.

<!-- screenshot: Imported simulation workflow detail with the three-node DAG annotated -->

See [Workflow definitions](./definitions.md) for the portable authoring contract and normalized activity model.

### 3. Plan it

Start automatic planning for the workflow. Select the simulation scope, its topology, and the algorithms available in your instance. Generate candidates, expand their timelines, and select one candidate to create a schedule plan.

<!-- screenshot: First-run automatic planning configuration -->

<!-- screenshot: First-run candidates with the selected time/cost trade-off highlighted -->

See [Plan a workflow](./planning.md) for sessions, candidates, manual plans, and imported plans.

### 4. Run and inspect it

Open the selected plan, start an execution, and follow the run until it completes. Inspect the activity timeline and transfers, then compare the predicted and observed Gantt views and time/cost metrics.

<!-- screenshot: First-run live execution with current activity highlighted -->

<!-- screenshot: First-run completed planned-versus-observed comparison -->

See [Execute and monitor a workflow](./executions.md) for run modes, states, monitoring, transfers, cost, and interactive sessions.

## Using the API

Set the address and token once:

```bash
export AKOFLOW_API_URL="http://127.0.0.1:<port>/akoflow-api"
export AKOFLOW_API_TOKEN="<token>"
```

Register the example's infrastructure and workflow:

```bash
for document in environment scope topology; do
  case "$document" in
    environment) endpoint="environments" ;;
    scope) endpoint="execution-scopes" ;;
    topology) endpoint="network-topologies" ;;
  esac
  curl --fail-with-body \
    -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
    -H 'Content-Type: application/yaml' \
    --data-binary "@examples/simulation/$document.yaml" \
    "$AKOFLOW_API_URL/$endpoint/"
done

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/simulation/workflow.yaml \
  "$AKOFLOW_API_URL/workflow-definitions/"
```

For the shortest reproducible path, register the checked-in plan envelope and submit its matching execution request:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/simulation/plan-request.yaml \
  "$AKOFLOW_API_URL/schedule-plans/"

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/yaml' \
  --data-binary @examples/simulation/execution-request.yaml \
  "$AKOFLOW_API_URL/execution-runs/"
```

Submission is asynchronous. Poll the run detail until it reaches `completed` or `failed`:

```bash
curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/execution-runs/simulation-example-run-v1/"
```

To exercise automatic planning instead, create a planning session as described in [Plan a workflow](./planning.md), select one of its candidates, and build the execution envelope with that returned plan and the registered catalog snapshots.

## What to verify

A successful first run demonstrates that:

1. the workflow graph and simulation infrastructure were registered;
2. every activity received a valid resource assignment;
3. dependencies controlled the execution order;
4. the run produced activity, transfer, handle, and event records;
5. predicted and observed metrics can be inspected through both the Desktop and API.
