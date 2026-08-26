# SimGrid stress scenarios

Generate and run workflows by the number of intermediate activities on M2.
By default, M2 receives one core per worker, so every worker has a dedicated
core.

```sh
AKOFLOW_API_TOKEN=... ./generate.py 100 1000
```

For example, `100` creates 100 workers on M2 plus one source on M1 and one sink
on M3, totaling 102 activities. Every dependency transfers 10 GB over a shared
80 Gbit/s link. Use `--cores N` to explicitly test a smaller shared core pool.
