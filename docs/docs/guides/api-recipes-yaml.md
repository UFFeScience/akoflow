---
id: api-recipes-yaml
title: API recipes: importing fixtures as YAML
sidebar_label: YAML import recipes
slug: /guides/api-recipes-yaml
description: Tested curl recipes that import an example fixture as YAML and walk the run to completion.
---

# API recipes: importing fixtures as YAML

The AkôFlow API accepts YAML on every collection endpoint, which lets
operators reproduce a fixture without first translating it to JSON. The
recipes below are the same sequence the validation harness exercises in
`internal/validation/example_http_test.go` and that the example `run.sh`
scripts run against a live daemon.

## Authentication

Set the base URL and token in your shell. The token is optional when the
daemon runs loopback-only; sending it anyway mirrors what the Desktop does.

```sh
export AKOFLOW_API_URL="http://127.0.0.1:8080/akoflow-api"
export AKOFLOW_API_TOKEN="${AKOFLOW_API_TOKEN:-}"

authorization_header=()
if [ -n "$AKOFLOW_API_TOKEN" ]; then
  authorization_header=(-H "Authorization: Bearer $AKOFLOW_API_TOKEN")
fi
```

## Import an environment, scope, topology, and workflow

These four POSTs are the minimum required to attach a workflow to a daemon.
Each returns the created record (`201 Created`) with the assigned IDs echoed
back. Use the IDs from the response when composing the plan and run requests.

```sh
curl --fail-with-body -sS "${authorization_header[@]}" \
  -H 'Content-Type: application/yaml' \
  --data-binary @environment.yaml \
  "$AKOFLOW_API_URL/environments/"

curl --fail-with-body -sS "${authorization_header[@]}" \
  -H 'Content-Type: application/yaml' \
  --data-binary @scope.yaml \
  "$AKOFLOW_API_URL/execution-scopes/"

curl --fail-with-body -sS "${authorization_header[@]}" \
  -H 'Content-Type: application/yaml' \
  --data-binary @topology.yaml \
  "$AKOFLOW_API_URL/network-topologies/"

curl --fail-with-body -sS "${authorization_header[@]}" \
  -H 'Content-Type: application/yaml' \
  --data-binary @workflow.yaml \
  "$AKOFLOW_API_URL/workflow-definitions/"
```

## Import a plan

Use `/schedule-plans/import/` when the plan is shipped as a bare `{ plan: ... }`
document (the common case for hand-written local fixtures). Use
`/schedule-plans/` when the plan must travel with its supporting artefacts
(the simulation envelope in `examples/simulation/plan-request.yaml`):

```sh
# bare plan
curl --fail-with-body -sS "${authorization_header[@]}" \
  -H 'Content-Type: application/yaml' \
  --data-binary @plan.yaml \
  "$AKOFLOW_API_URL/schedule-plans/import/"

# envelope plan (plan + workflow + resources + runtimes + bindings + scope)
curl --fail-with-body -sS "${authorization_header[@]}" \
  -H 'Content-Type: application/yaml' \
  --data-binary @plan-request.yaml \
  "$AKOFLOW_API_URL/schedule-plans/"
```

The two endpoints accept disjoint shapes. The bare plan must NOT carry
`workflow`, `resources`, `executionScope`, or `networkTopology`; the envelope
plan must carry all of them or the request fails with `400 unknown field`.

## Trigger and poll a run

`POST /execution-runs/` queues the request onto the event loop and returns
`202 Accepted`. The body is the queued job; the run is created asynchronously.
Poll `GET /execution-runs/{runId}/` with a short backoff:

```sh
post_runs() {
  curl --fail-with-body -sS "${authorization_header[@]}" \
    -H 'Content-Type: application/yaml' \
    --data-binary @execution-request.yaml \
    "$AKOFLOW_API_URL/execution-runs/"
}
post_runs

run_id="local-direct-hello-run-v1"
for attempt in $(seq 1 50); do
  body=$(curl --fail-with-body -sS "${authorization_header[@]}" \
    "$AKOFLOW_API_URL/execution-runs/$run_id/")
  status=$(printf '%s' "$body" | jq -r '.run.status // "missing"')
  case "$status" in
    completed|failed) printf 'final run status: %s\n' "$status"; break ;;
    *) sleep 0.2 ;;
  esac
done
```

The run lifecycle is documented in
[reference/planning-and-execution-states](../reference/planning-and-execution-states).
The harness asserts the status is one of `created`, `running`, `completed`,
or `failed` before declaring the journey validated.

## Inspect activity, outputs, and provenance

Once the run reaches `completed`, fetch the activity detail and the
provenance records:

```sh
curl --fail-with-body -sS "${authorization_header[@]}" \
  "$AKOFLOW_API_URL/execution-runs/$run_id/activities/"

curl --fail-with-body -sS "${authorization_header[@]}" \
  "$AKOFLOW_API_URL/artifacts/?runId=$run_id"

curl --fail-with-body -sS "${authorization_header[@]}" \
  "$AKOFLOW_API_URL/provenance/?runId=$run_id"
```

The validation harness captures every one of these GETs against a freshly
booted daemon in `internal/validation/example_http_test.go`.
