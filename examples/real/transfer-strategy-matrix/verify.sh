#!/bin/sh
set -eu

base=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

"$base/generate.sh"
jq empty "$base"/*.json

for workflow in "$base"/workflow-*.json; do
  case "$workflow" in
    *roundtrip*) continue ;;
  esac
  workspace=$(mktemp -d)
  producer=$(jq -er '.spec.activities[0].run' "$workflow")
  consumer=$(jq -er '.spec.activities[1].run' "$workflow")
  (
    cd "$workspace"
    sh -c "$producer"
    sh -c "$consumer"
    test -s payload.txt
    test -s payload.sha256
    test -s receipt.txt
  )
  printf 'validated %s\n' "$(basename "$workflow")"
done

workspace=$(mktemp -d)
index=0
while [ "$index" -lt 4 ]; do
  command=$(jq -er ".spec.activities[$index].run" "$base/workflow-roundtrip.json")
  (cd "$workspace" && sh -c "$command")
  index=$((index + 1))
done
test -s "$workspace/journey.txt"
test -s "$workspace/journey.sha256"
test -s "$workspace/final.receipt"
printf 'validated %s\n' workflow-roundtrip.json
