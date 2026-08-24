#!/bin/sh
set -eu

api=${AKOFLOW_API_URL:-http://localhost:8080/akoflow-api}
directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

post() {
  endpoint=$1
  file=$2
  curl -fsS -H 'Content-Type: application/yaml' --data-binary "@$directory/$file" "$api/$endpoint/"
  printf '\n'
}

post environments environment.yaml
post execution-scopes scope.yaml
post network-topologies topology.yaml
post workflow-definitions workflow.yaml
post schedule-plans plan-request.yaml
post execution-runs execution-request.yaml

printf 'Result: %s/execution-runs/simulation-example-run-v1/\n' "$api"
