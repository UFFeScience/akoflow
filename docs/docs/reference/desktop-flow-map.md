---
title: Desktop flow map
description: Renderer routes, Core contracts, ownership boundaries, and verification points for the AkôFlow Desktop.
---

# Desktop flow map

This map inventories the routes declared by the current Desktop in `src/App.jsx`
and connects each user-facing flow to its Core contract. The Desktop repository
is the source of truth for routes and presentation. The Core router, handlers,
and domain types remain the source of truth for HTTP behavior. See the
[Core and Desktop contract matrix](./core-ui-contract-matrix) for status codes,
state machines, polling, and known contract gaps.

The packaged application uses `HashRouter`; the development renderer uses
`BrowserRouter`. Both renderers call the daemon through HTTP. Electron main owns
local bootstrap and authenticated proxying. The renderer must not run Docker,
SSH, kubectl, BuildKit, or shell commands.

## Routes and flows

| Flow                       | Canonical Desktop routes                                                                                                                                                     | Core boundary                                                                                                | What to verify                                                                                                 |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------- |
| Installation and bootstrap | pre-shell loading/onboarding; `/`; `/settings`                                                                                                                               | `/instance/`, `/instances/`, `/instance-activations/:id/`, `/preflight/`, import/export/reset                | The daemon is reachable before accepting a capture; distinguish package bootstrap from renderer readiness.     |
| Workflow definitions       | `/workflows`, `/workflows/new`, `/workflows/:id`                                                                                                                             | `/workflow-definitions/`, import, export, duplicate                                                          | Validation, DAG, immutable version, and empty/error states.                                                    |
| Environments               | `/environments`, `/environments/new`, `/environments/:id`, `/environments/:id/edit`, `/environments/:id/inventory`, `/environments/:id/storages`                             | environments, connection tests, health, discovery, credential references, storage actions                    | Connected targets validate before registration; simulations do not imply a real connection.                    |
| Infrastructure models      | `/resources`, `/resources/:id`, `/machine-configurations`, `/execution-scopes`, `/execution-scopes/new`, `/execution-scopes/:id`, `/network`, `/network/new`, `/network/:id` | resources/snapshots, machine configurations/versions, execution scopes, network topologies                   | Ownership, compatibility lookup, validation failures, and immutable versions.                                  |
| Planning                   | `/planning-sessions`, `/planning-sessions/:id`, `/workflows/:id/plans/new`, `/workflows/:workflowId/plans/:id`, `/plans`                                                     | planning algorithms/sessions/candidates/select/cancel, schedule plans                                        | Poll queued/running sessions, select only feasible candidates, and preserve the workflow owner in navigation.  |
| Runs                       | `/executions`, `/executions/new`, `/workflows/:workflowId/plans/:planId/executions/:id`, child `/activities/:activityId`                                                     | execution runs and activity reads                                                                            | A `202` queue acceptance is not a completed or even persisted run; show live and terminal evidence separately. |
| Artifacts and transfers    | `/artifacts`, `/artifacts/new`, `/artifacts/:id`, `/artifact-locations`, `/materializations`, environment storage browser                                                    | artifacts, builds/build runs/output, materializations, storage browse/copy/download/promotion/checksum/index | Binary responses are not JSON; accepted copy/archive work has no dedicated public status route.                |
| Provenance and audit       | `/provenance`, `/data`, `/audit`, execution/activity evidence sections                                                                                                       | provenance catalog/query/SQL/explain/lineage and audit events                                                | Scientific provenance and operational audit are separate histories; query errors must remain visible.          |
| Cloud lifecycle            | `/environments/:id/cloud-capacity`, `/environments/:id/provisioning`, `/resources/:resourceId/provisioning/:instanceId`                                                      | capacity/catalog/configuration, provision/configure/start/stop/destroy/validate, operation events            | Poll operation records/events, preserve resource ownership, and treat provider capability errors as failures.  |
| Console                    | `/console` and the global terminal                                                                                                                                           | console commands/sessions/stream/log/close                                                                   | Electron proxies the authenticated stream; closing and reconnecting are explicit lifecycle actions.            |

## Navigation ownership

- A plan belongs to a workflow: `/workflows/:workflowId/plans/:id`.
- An execution belongs to a workflow plan: `/workflows/:workflowId/plans/:planId/executions/:id`.
- An activity belongs to that execution and appends `/activities/:activityId`.
- A provisioning operation belongs to a resource:
  `/resources/:resourceId/provisioning/:instanceId`.

The shorter `/plans/:id`, `/executions/:id`, activity routes below executions,
and provisioning detail below environments are compatibility routes. New links
should preserve the complete owner chain. Aggregate pages may list children
from several owners, but both owner and child must remain independently
navigable.

## HTTP and credential boundary

Renderer requests are implemented in `src/providers/akoflow.js` and
`src/providers/http.js`. Connection secrets go only to dedicated credential
endpoints; ordinary environment definitions and browser preferences store
credential references. The Electron main process may bootstrap the pinned local
stack, but it must not expose the daemon token to renderer storage.

When a route or contract changes, compare this map with `src/App.jsx`, the two
Desktop providers above, `internal/api/httpserver/httpserver.go`, and the
relevant Core handler/domain type. Update both contract maps in the same pull
request and record the Desktop commit used for verification.
