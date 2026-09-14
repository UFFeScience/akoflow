# Local SLURM batch fixture

This is a complete local adapter fixture, not a connection test for a production
SLURM cluster. It starts an isolated AkôFlow daemon and provides lightweight
`sbatch` and `singularity` shims so the submitted workflow creates `result.txt`.

## Bundle contents

- `environment.yaml`: SLURM-backed execution environment and runtime binding.
- `scope.yaml` and `topology.yaml`: execution scope and network model.
- `workflow.yaml`: one batch activity that writes `result.txt`.
- `plan.yaml` and `execution-request.yaml`: imported manual plan and run request.
- `start-fixture.sh`: starts the disposable local daemon.
- `run.sh`: imports every model artifact and submits the execution.

## Run and verify

From a repository checkout with Go, Docker, `curl`, and POSIX `sh` available:

```sh
sh examples/slurm/local-fixture/start-fixture.sh
sh examples/slurm/local-fixture/run.sh
curl --fail-with-body \
  http://127.0.0.1:18082/akoflow-api/execution-runs/slurm-fixture-run-v1/
```

Wait for `status` to become `completed`; the artifact manifest must contain
`result.txt` with the text `slurm fixture completed`. Stop the fixture using
the cleanup instructions on the accompanying documentation page.
