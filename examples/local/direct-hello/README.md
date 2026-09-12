# Local direct execution

This is a complete, single-activity real execution for the `local` runtime.
The AkôFlow daemon starts `sh` directly on its own host and records the file
that the activity creates. It is intended for a safe first real execution,
small host-local tools, and runtime integration checks.

The activity writes `result.txt` containing `Akoflow local execution`.

## Run it

Start an AkôFlow daemon on the same host that is permitted to execute the
command, then submit the files in order:

```sh
export AKOFLOW_API_URL="http://127.0.0.1:8080/akoflow-api"
./run.sh
curl --fail-with-body "$AKOFLOW_API_URL/execution-runs/local-direct-hello-run-v1/"
```

The environment has one direct resource, `local-host`, and no network links.
`workflow.yaml` uses an OCI reference because portable real workflow imports
require an executable reference. The current local adapter does not pull or
run that image: it invokes `command.entrypoint` and `command.arguments` on the
daemon host.

Use a constrained daemon host and only commands trusted by its operator. This
runtime is not a container sandbox or a replacement for Kubernetes or SLURM.
