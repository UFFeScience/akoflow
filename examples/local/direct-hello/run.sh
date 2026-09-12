#!/bin/sh
set -eu

api=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

authorization_header=
if [ -n "${AKOFLOW_API_TOKEN:-}" ]; then
  authorization_header="Authorization: Bearer $AKOFLOW_API_TOKEN"
fi

post() {
  endpoint=$1
  file=$2
  if [ -n "$authorization_header" ]; then
    curl --fail-with-body -sS -H "$authorization_header" \
      -H 'Content-Type: application/yaml' \
      --data-binary "@$directory/$file" "$api/$endpoint/"
  else
    curl --fail-with-body -sS -H 'Content-Type: application/yaml' \
      --data-binary "@$directory/$file" "$api/$endpoint/"
  fi
  printf '\n'
}

post environments environment.yaml
post execution-scopes scope.yaml
post network-topologies topology.yaml
post workflow-definitions workflow.yaml
post schedule-plans/import plan.yaml
post execution-runs execution-request.yaml

printf 'Result: %s/execution-runs/local-direct-hello-run-v1/\n' "$api"
