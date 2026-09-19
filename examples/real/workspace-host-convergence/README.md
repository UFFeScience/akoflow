# Workspace Host Convergence

Real execution workflow that alternates between Kind/Kubernetes and Plafrim.
Every activity records its host, user, UTC timestamp, working directory and
kernel information as one JSON Lines record.

The final Plafrim activity validates `k1.txt` through `k6.txt` and writes
`k7.txt` with its own record followed by the complete contents of every input.

Placement:

```text
k1 Kubernetes ─┐
               ├─> k3 Plafrim ─┬─> k4 Kubernetes ─┐
k2 Plafrim ────┘                └─> k5 Plafrim ────┼─> k6 Kubernetes ─> k7 Plafrim
```
