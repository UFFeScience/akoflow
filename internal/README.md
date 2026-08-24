# AkoFlow internal architecture

The server and worker follow dependency-oriented boundaries:

- `domain`: workflow, environment, resource, planning and execution concepts.
- `application`: use-case services and provider-independent ports.
- `infrastructure`: SQLite persistence, planning plugins and Kubernetes, Slurm and SSH adapters.
- `execution`: orchestration, lifecycle, real-runtime and simulation behavior.
- `api`: HTTP handlers, transport requests and responses.

`cmd/server` and `cmd/worker` are composition roots. Domain packages must not
import application, infrastructure, execution or API packages. New external
integrations implement a port and stay under `infrastructure`.
