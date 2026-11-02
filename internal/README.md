# AkôFlow internal architecture

The daemon follows dependency-oriented boundaries:

- `domain`: workflow, environment, resource, planning and execution concepts.
- `application`: use-case services and provider-independent ports.
- `controlplane`: execution supervision, event processing and coordination.
- `planning`: scheduling algorithms, scheduler library contracts and reviewed plugins.
- `provider`: local, cloud, Kubernetes, Slurm, transfer, storage and simulation adapters.
- `infrastructure`: persistence, configuration, credentials and plugin infrastructure.
- `api`: HTTP handlers, transport requests and responses.
- `mcpbridge` and `mcpcontrol`: MCP transport, tool catalog and connection management.
- `cli`: command-line API integration.

`cmd/server` composes the daemon. `cmd/akoflow` is the API client entry point.
Domain packages must not import application services, infrastructure, providers,
control-plane orchestration or transport code. External adapters implement the
ports consumed by application services.
