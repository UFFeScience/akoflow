#!/bin/sh
set -eu

case "${1:-}" in
  local)
    origin_environment=local-environment
    origin_resource=local-environment-entrypoint
    workflow_id=goal-local-cloud-transfer
    plan_id=goal-local-cloud-plan
    ;;
  kind)
    origin_environment=kind-akoflow
    origin_resource=kind-worker
    workflow_id=goal-kind-cloud-transfer
    plan_id=goal-kind-cloud-plan
    ;;
  *) echo 'usage: submit-run.sh local|kind RUN_ID' >&2; exit 2 ;;
esac

run_id=${2:?run id required}
api_base=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
api_token=${AKOFLOW_API_TOKEN:?API token required}

get() {
  curl --fail-with-body -sS -H "Authorization: Bearer $api_token" "$api_base/$1/"
}

plan_json=$(get "schedule-plans/$plan_id")
definition_json=$(get "workflow-definitions/$workflow_id")
scope_json=$(get execution-scopes/goal-hybrid-scope)
network_json=$(get network-topologies/goal-hybrid-network)
origin_environment_json=$(get "environments/$origin_environment")
cloud_environment_json=$(get environments/goal-gcp-worker)
origin_resource_json=$(get "resources/$origin_resource")
cloud_resource_json=$(get resources/goal-gcp-worker-e2-small)

jq -nc \
  --arg runId "$run_id" \
  --argjson plan "$plan_json" \
  --argjson definition "$definition_json" \
  --argjson scope "$scope_json" \
  --argjson network "$network_json" \
  --argjson originEnvironment "$origin_environment_json" \
  --argjson cloudEnvironment "$cloud_environment_json" \
  --argjson originResource "$origin_resource_json" \
  --argjson cloudResource "$cloud_resource_json" \
  '{run:{id:$runId,schedulePlanId:$plan.id,mode:"real",status:"created"},
    plan:$plan,workflow:$definition.version,executionScope:$scope,networkTopology:$network,
    resources:[$originResource,$cloudResource],
    runtimes:($originEnvironment.runtimes+$cloudEnvironment.runtimes),
    runtimeBindings:($originEnvironment.resourceRuntimeBindings+$cloudEnvironment.resourceRuntimeBindings)}' |
  curl --fail-with-body -sS -H "Authorization: Bearer $api_token" \
    -H 'Content-Type: application/json' --data-binary @- "$api_base/execution-runs/"
