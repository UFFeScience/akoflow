# SimGrid 50-core fan-out

This scenario uses three machines. M1 runs `source`, M2 runs 50 workers in
parallel (one worker on each core), and M3 runs `sink`.

Each edge transfers 10 GB over a shared 80 Gbit/s link. Fifty concurrent
transfers take 50 seconds per network stage. The predicted makespan is 130
seconds: 10 + 50 + 10 + 50 + 10.
