# SimGrid stress scenarios

Generate and run workflows with an exact total number of activities over a
three-machine environment. By default, M2 receives as many cores as the total
activity count, so every intermediate worker has a dedicated core.

```sh
AKOFLOW_API_TOKEN=... ./generate.py 100 1000
```

Each workflow contains one source activity on M1, `N - 2` workers on M2, and
one sink activity on M3. Every dependency transfers 10 GB over a shared
80 Gbit/s link. Use `--cores N` to explicitly test a smaller shared core pool.
