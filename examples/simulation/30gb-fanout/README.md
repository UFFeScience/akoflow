# SimGrid 30 GB fan-out

This example models three machines connected at 10 GB/s. M2 has three cores;
M1 and M3 have one core each:

```text
M1: t1 (10 s)
       |-- 10 GB --> t2 -- 10 GB --|
M2:    |-- 10 GB --> t3 -- 10 GB --|--> M3: t5 (10 s)
       |-- 10 GB --> t4 -- 10 GB --|
```

`t2`, `t3`, and `t4` use separate M2 cores and therefore execute in parallel. The three
transfers on each network stage share an 80 Gbit/s link, so a concurrent batch
of three 10 GB objects takes 3 seconds. The manual plan records a 36-second
makespan: 10 s for `t1`, 3 s for fan-out, 10 s in parallel on M2, 3 s for fan-in, and 10 s
for `t5`. `execution-request.yaml` is the portable execution envelope; use
`run.sh` against a fresh AkôFlow instance to submit the entire bundle.
