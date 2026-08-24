# Simulation example

This example executes a frozen schedule plan with the real SimGrid S4U engine.
Akoflow materializes the platform and workload, starts an isolated
`akoflow-simgrid-runner` process, waits for its virtual execution to finish and
persists the resulting task and transfer trace.

Build the runner when developing outside the server image:

```sh
cmake -S simgrid-runner -B build/simgrid-runner -G Ninja -DCMAKE_BUILD_TYPE=Release
cmake --build build/simgrid-runner --parallel
export AKOFLOW_SIMULATION_BACKEND=simgrid
export AKOFLOW_SIMGRID_BINARY="$PWD/build/simgrid-runner/akoflow-simgrid-runner"
```

Run the AkôFlow server and then execute:

```sh
sh examples/simulation/run.sh
```

Inspect the result:

```sh
curl -fsS http://localhost:8080/akoflow-api/execution-runs/simulation-example-run-v1/
```

No Kubernetes Pod or Slurm job is launched. A native SimGrid subprocess is
created, but all compute and network times reported by it are virtual.

Every execution bundle remains under `storage/simgrid/<run-id>-<instance>/`
with `platform.xml`, `simulation.json`, `result.json` and `runner.log`, allowing
the experiment to be inspected and reproduced independently.
