# SimGrid stress scenarios

Generate and run workflows with an exact total number of activities over a
three-machine environment. M2 has 50 cores; the intermediate workers are
distributed across those cores in consecutive waves.

```sh
AKOFLOW_API_TOKEN=... ./generate.py 100 1000
```

Each workflow contains one source activity on M1, `N - 2` workers on M2, and
one sink activity on M3. Every dependency transfers 10 GB over a shared
80 Gbit/s link.
