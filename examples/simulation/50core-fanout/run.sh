#!/bin/sh
set -eu
api=${AKOFLOW_API_URL:-http://localhost:8080/akoflow-api}
token=${AKOFLOW_API_TOKEN:-akoflow-development-token}
directory=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
post() {
  curl -fsS -H "Authorization: Bearer $token" -H 'Content-Type: application/yaml' --data-binary "@$directory/$2" "$api/$1/"
  printf '\n'
}
post environments environment.yaml
post execution-scopes scope.yaml
post network-topologies topology.yaml
post workflow-definitions workflow.yaml
post schedule-plans plan-request.yaml
{
  cat "$directory/execution-request.yaml"
  cat "$directory/plan-request.yaml"
} | curl -fsS -H "Authorization: Bearer $token" -H 'Content-Type: application/yaml' \
  --data-binary @- "$api/execution-runs/"
printf '\nRun: %s/execution-runs/simgrid-50core-fanout-run-v3/\n' "$api"
