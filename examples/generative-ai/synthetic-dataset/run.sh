#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
image="akoflow-showcase-synthetic-dataset:dev"
rm -rf "$root/outputs" "$root/screenshots"
mkdir -p "$root/outputs" "$root/screenshots"
docker build --tag "$image" --file "$root/docker/Dockerfile" "$root"
docker run --rm --mount "type=bind,src=$root/outputs,dst=/outputs" "$image"
node "$root/scripts/capture_screenshots.mjs"
"$root/validate.sh"

