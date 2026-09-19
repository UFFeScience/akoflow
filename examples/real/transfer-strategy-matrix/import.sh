#!/bin/sh
set -eu

base=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
api_base=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
api_token=${AKOFLOW_API_TOKEN:?API token required}

post_json() {
  endpoint=$1
  file=$2
  curl --fail-with-body -sS \
    -H "Authorization: Bearer $api_token" \
    -H 'Content-Type: application/json' \
    --data-binary "@$file" \
    "$api_base/$endpoint/"
  printf '\n'
}

post_json execution-scopes "$base/scope.json"
post_json network-topologies "$base/topology.json"

for workflow in "$base"/workflow-*.json; do
  post_json workflow-definitions "$workflow"
done

for plan in "$base"/plan-*.json; do
  post_json schedule-plans/import "$plan"
done
