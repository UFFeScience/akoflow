#!/bin/sh
# Start an isolated daemon that resolves sbatch and singularity to this fixture.
set -eu

repository=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
fixture=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
state="$fixture/.runtime"
mkdir -p "$state"
chmod +x "$fixture/bin/sbatch" "$fixture/bin/singularity"

# The daemon must run from the fixture directory: the Slurm adapter stores its
# sentinel and log files relative to the submission directory. Build from the
# repository first, then execute the isolated binary from that directory.
(
  cd "$repository"
  go build -o "$state/akoflow-server" ./cmd/server
)
cd "$fixture"

exec env \
  PATH="$fixture/bin:$PATH" \
  AKOFLOW_SLURM_FIXTURE_ROOT="$fixture" \
  AKOFLOW_SLURM_FIXTURE_BIN="$fixture/bin" \
  AKOFLOW_DATABASE_PATH="$state/akoflow.sqlite" \
  AKOFLOW_INSTANCE_ARCHIVE_ROOT="$state/instances" \
  AKOFLOW_ARTIFACT_STORE_ROOT="$state/artifacts" \
  AKOFLOW_SLURM_SCRIPT_DIRECTORY="$fixture/scripts" \
  AKOFLOW_HTTP_ADDRESS="${AKOFLOW_HTTP_ADDRESS:-127.0.0.1:18082}" \
  "$state/akoflow-server"
