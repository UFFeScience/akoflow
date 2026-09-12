#!/bin/sh
set -eu

api=${AKOFLOW_API_URL:-http://127.0.0.1:18082/akoflow-api}
directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
post() {
  if [ -n "${AKOFLOW_API_TOKEN:-}" ]; then
    curl --fail-with-body -sS -H 'Content-Type: application/yaml' \
      -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
      --data-binary "@$directory/$2" "$api/$1/"
  else
    curl --fail-with-body -sS -H 'Content-Type: application/yaml' \
      --data-binary "@$directory/$2" "$api/$1/"
  fi
  printf '\n'
}

post environments environment.yaml
post execution-scopes scope.yaml
post network-topologies topology.yaml
post workflow-definitions workflow.yaml
post schedule-plans/import plan.yaml
post execution-runs execution-request.yaml

printf 'Result: %s/execution-runs/slurm-fixture-run-v1/\n' "$api"
