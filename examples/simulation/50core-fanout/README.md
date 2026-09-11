# SimGrid 100 activities on 50 cores

This scenario places 100 independent, 10-second activities on M2, which has
50 schedulable cores. Every activity requests one core. The manual plan has
two waves: workers 001–050 use cores 0–49 from 0 to 10 seconds, and workers
051–100 reuse those cores from 10 to 20 seconds.

M1 and M3 remain in the environment to keep the same three-machine scope used
by the other SimGrid examples, but this workload intentionally has no data
dependencies. It isolates core capacity and makes a 20-second makespan easy
to verify.

Run the complete bundle against a fresh AkôFlow instance:

```sh
export AKOFLOW_API_URL="http://127.0.0.1:8080/akoflow-api"
export AKOFLOW_API_TOKEN="<token>"
bash examples/simulation/50core-fanout/run.sh
```

The script creates persistent objects and rejects duplicate IDs. A completed
run has ID `simgrid-50core-workers-run-v2`, 100 settled activities, a 20-second
makespan, and no data transfers.
