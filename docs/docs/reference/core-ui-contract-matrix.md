---
title: Core and Desktop contract matrix
description: Audited HTTP contracts, asynchronous behavior, ownership boundaries, and known UI integration gaps.
---

# Core and Desktop contract matrix

This page maps the user-visible flows to the current daemon contract. It is an
integration checklist, not a second API specification. The registered method
and path in `internal/api/httpserver/httpserver.go`, the handler and domain type
named in each row, and the generated endpoint catalog are the sources of truth.

All paths below are relative to `/akoflow-api`. Unless a row says otherwise,
requests and responses are JSON, errors use `{ "error": "message" }`, and the
Desktop renderer calls through Electron main. Electron main owns daemon
bootstrap, token injection and privileged host operations; the renderer owns
forms, navigation and polling; the daemon owns validation, persistence,
planning, execution and provider access. The renderer must never execute
Docker, shell, SSH, Kubernetes or cloud commands.

## Flow matrix

| Flow | Endpoint and contract | State and client update | Ownership | Code source and audited gap |
|---|---|---|---|---|
| Install and bootstrap | `GET /instance/` returns installation identity; `PUT /instance/` saves it. `GET /preflight/` returns `server`, `docker` and `buildkit` availability objects. | Bootstrap calls are synchronous. Electron main starts and health-checks the packaged daemon; the renderer displays readiness. | Electron main: release assets, containers and secret token. Daemon: readiness checks. Renderer: status only. | Router and `httpserver.Preflight`; instance handlers. **Gap:** Core cannot prove that a Desktop release bundled matching archives; installation must verify Release assets. |
| Workflow definition | `POST /workflow-definitions/` accepts the versioned workflow document and returns the created definition (`201`). List/detail/export and duplicate routes provide retrieval and reuse. | No background state. Validation failures are `422`; missing detail is `404`. | Renderer authors/imports; daemon validates IDs, DAG and command/data contracts and persists immutable versions. | `requests.Workflow.Domain`, `Handler.CreateWorkflow`, workflow repository. No Core contract gap. |
| Environments | `POST /environments/` accepts `EnvironmentDefinition` and returns it (`201`); list/detail/replace/delete manage it. Connection test, health, discovery and history are separate routes. | Creation/replacement is synchronous. Discovery and health results are read from their response/history; there is no environment event stream. | Renderer collects non-secret configuration. Electron main brokers local credentials where required. Daemon stores credential references, probes and persists observations. | Environment domain, handlers and repository. **Gap:** clients must not infer resource readiness from an environment existing; run connection health/discovery explicitly. |
| Planning | `POST /planning-sessions/` accepts `PlanningSession` input and returns the created session (`202`). Detail returns `{session, algorithmRuns}`; candidates are separate; selecting a feasible candidate returns a `SchedulePlan` (`201`). | Poll session detail while `queued` or `running`; stop on `completed`, `failed` or `cancelled`. There is no planning SSE/WebSocket contract. | Renderer selects inputs/algorithms, polls and chooses a candidate. Daemon freezes inputs, runs algorithms, ranks candidates and persists the selected plan. | Planning domain, planning service, session handler and API handlers. **Gap:** selection is currently allowed before session completion; UI should normally wait for final ranking. |
| Manual/imported plan | `POST /schedule-plans/` accepts `{plan, workflow, resources, executionScope, networkTopology}`; import accepts `{plan}`. Both return the stored plan (`201`). | A schedule plan has no status. Validation is synchronous and failures are `422`. | Renderer/API supplies a complete plan; daemon validates assignments/topology and enriches cloud lifecycle actions. | `Handler.CreatePlan`, `ImportPlan`, plan validator. No Core contract gap. |
| Execution run | `POST /execution-runs/` accepts `ports.ExecutionRequest` and returns a durable queue job (`202`), not the run. List returns `{items,page,pageSize,total,hasNext}`; detail returns run evidence. | The daemon creates the run only after consuming the queue job. Clients retry/list until the requested run ID appears, then poll while `running`; terminal states are `completed` and `failed`. No run event stream or cancel endpoint exists. | Renderer requests and polls. Daemon queue/event loop and supervisor exclusively own dispatch, provider polling, transfers and evidence. | `Handler.CreateExecution`, queue execution handler, `ExecutionSupervisor`, execution domain. **Gap:** accepted does not mean started; UI must retain the submitted run ID and represent queue acceptance separately. |
| Artifacts and builds | Artifact registration/build-context/build-spec routes create immutable inputs. `POST /artifact-builds/{buildId}/runs/` returns a build run (`202`); status and binary output have separate GET routes. Materializations can be listed/recorded. | Poll `GET /build-runs/{runId}/`; fetch `/output/` only after success. Output is `application/vnd.sylabs.sif`, not JSON. | Renderer chooses source and shows logs. Electron main handles any host file chooser/upload bridge. Daemon/BuildKit own builds, digests and materializations. | Data domain, build service and artifact handlers. **Gap:** availability depends on configured build service; `503` is a capability result, not a retryable validation error. |
| Storage, downloads and transfers | Browse/stat are synchronous. `POST /storages/{id}/downloads/` returns a download record (`201`), then status and content are separate. Copy/archive return accepted work (`202`). Promotion routes create data/artifact records. | Poll download status and index-run lists; binary content is not JSON. Copy/archive have no public job-detail or event route. Execution transfers are observed in run detail, not controlled by the UI. | Renderer selects logical paths. Electron main owns the save dialog and filesystem destination. Daemon enforces roots, invokes storage/transfer adapters and records checksums/evidence. | Storage handlers/service; transfer service; execution trace domain. **Gap:** copy/archive acceptance cannot currently be followed through a dedicated public status endpoint; refresh browse/audit evidence and do not display acceptance as completion. |
| Provenance and audit | Entity catalog/query, safe read-only SQL, explain and lineage are synchronous JSON. `GET /audit-events/` accepts documented filters. | Query on demand; no live subscription. Pagination/filter shape is endpoint-specific. | Renderer builds filters and visualizations. Daemon enforces the read-only SQL allowlist and returns persisted evidence. | Provenance handlers/repository/SQL guard and audit repository. No Core contract gap. |
| Cloud lifecycle | Capacity/catalog/configuration routes prepare inputs. Provision/configure/start/stop/destroy/validate return operation/instance records; operation detail and `/events/` expose progress. | Poll `GET /cloud-operations/{id}/` and fetch ordered events until the operation is terminal. These are HTTP events-as-records, not SSE. | Renderer requests actions and polls. Daemon queues operations; Terraform/Ansible/provider adapters own external calls and logs. | Cloud handlers, cloud operation event-loop handler, cloud allocator. **Gap:** provider capabilities differ; consult the provider matrix and treat `503`/validation errors as unsupported or unavailable, not success. |
| Interactive console | Recorded commands use `POST /console-commands/` (`201`). Sessions use `POST /console-sessions/`; stream, log and close have dedicated routes. | The session stream is a streaming response and must not be decoded as JSON. Session list/log provide durable inspection; close is explicit. | Renderer renders terminal I/O. Electron main proxies authenticated streaming and lifecycle. Daemon terminal adapter owns the remote process and audit record. | Console/terminal handlers and provider terminal adapters. **Gap:** generic browser polling is not a substitute for the stream route. |

## Common client rules

1. Use the exact trailing-slash route registered by the mux and send the bearer
   token on every non-bootstrap API request.
2. Treat `201` as a created domain record and `202` as accepted asynchronous
   work. Never synthesize a completed state from either status code.
3. Stop polling only on the terminal states documented for that record. A queue
   job, planning session, build run, cloud operation and execution run are
   different state machines.
4. Handle `400` as malformed input, `401` as missing/invalid authentication,
   `403` as network/origin policy, `404` as an absent record, `409` as a state
   conflict, `422` as domain validation, `423` as an activated read-only
   snapshot and `503` as an unavailable capability.
5. Do not expose the daemon token to renderer code or persist credentials in
   ordinary environment/resource payloads.

## How to keep this map current

When a route or payload changes, update its handler/domain type first, add a
directed Go test, regenerate the endpoint catalog with
`npm run generate:api --prefix docs`, and update this matrix and the relevant task guide in the same
pull request. Contract changes also require explicit compatibility notes for
Desktop maintainers. A generated endpoint page proves registration; it does
not replace lifecycle and ownership documentation here.
