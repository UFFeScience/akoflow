---
title: Complete task flows
description: Ten end-to-end paths from Desktop bootstrap to execution evidence and operations.
---

# Complete task flows

This page connects ten user goals and makes their hand-offs explicit. Follow
the flows in order for a new installation, or jump to one whose prerequisites
already exist. Detailed guides linked from every flow contain complete payloads.

For API alternatives, set the operator-controlled daemon address and token.
Desktop obtains its connection during bootstrap; never paste a token into
browser storage.

```bash
export AKOFLOW_URL='http://127.0.0.1:<daemon-port>/akoflow-api'
export AKOFLOW_TOKEN='<daemon-token>'
```

## 1. Install and bootstrap Desktop

**Prerequisites.** Use a supported workstation with Docker running. Choose one
GitHub Release whose Desktop installer, daemon archive, BuildKit archive, and
checksums all have the same version and architecture.

**Desktop steps.** Install and open Desktop. On first launch, wait while the
Electron main process verifies Docker, loads the version-matched runtime
archives, starts the local stack, and checks the daemon. Continue only when the
welcome flow reports a connected Core and **Overview** opens.

![AkôFlow Desktop connected to the local daemon during first-run setup.](/img/interface/desktop/installation-welcome.png)

**API alternative.** Bootstrap is intentionally owned by Electron's main
process, so there is no renderer or public API equivalent. Operators running a
separate Linux instance should use the [self-managed server procedure](./operations/server-instance).

**Expected result.** `GET /instance/` returns HTTP 200 and Desktop displays the
same instance as connected.

**Asynchronous states and errors.** Archive download, image loading, container
startup, and readiness checks can take time. A missing asset, checksum mismatch,
unavailable Docker daemon, or readiness failure is not a partial success. Read
the failure detail before retrying.

**Next.** [Register an environment](#2-register-and-validate-an-environment).
The [installation guide](../installation) has release preflight and rollback.

## 2. Register and validate an environment

**Prerequisites.** Decide whether the target is a SimGrid model or connected
infrastructure. For a real target, prepare a daemon-owned credential reference
and endpoint or cluster configuration; never place secrets in the environment.

**Desktop steps.** Open **Infrastructure → Environments**. Choose **Create
simulation** for a model or **Connect environment** for real infrastructure.
Complete the runtime-specific form. For a connected target, run the connection
test before saving, then open the environment and run discovery.

![Environment catalog separating real targets from simulation models.](/img/interface/infrastructure/environments.png)

**API alternative.** Submit a complete definition to `POST /environments/`.
For a connection-backed version, call its `health-checks` endpoint and then its
`discoveries` endpoint. [Environments](./infrastructure/environments) contains
the exact routes and bodies.

**Expected result.** The detail shows an immutable version, healthy connection
when applicable, and discovered resources, runtimes, bindings, and storage.

**Asynchronous states and errors.** Health checks and discovery are queued.
Poll their records to a terminal state. A stored credential does not prove
connectivity; a healthy endpoint does not prove complete discovery. Resolve
authentication, host-key, namespace, permission, or binding errors before
planning.

**Next.** [Define a workflow](#3-define-or-import-a-workflow), then create an
[execution scope and topology](./infrastructure/execution-scopes).

## 3. Define or import a workflow

**Prerequisites.** Identify activities, control and data dependencies, resource
requirements, and real/simulation capabilities. Register executable artifacts
before selecting them in a real-execution command.

**Desktop steps.** Open **Workflows → Workflow definitions**. Use **Import
YAML** or **Create workflow**, review the DAG preview, save, then open the row
and verify the immutable version and dependencies.

![Workflow definition catalog with import and create paths.](/img/interface/workflows/definitions.png)

**API alternative.** Send portable YAML to `POST /workflow-definitions/` and
read the normalized response. See [Workflow definitions](./workflows/definitions).

**Expected result.** The definition has a stable version ID for planning.
Exporting produces portable YAML without generated IDs or resolved runtime paths.

**Asynchronous states and errors.** Creation is synchronous. A validation error
means the graph, command, dependency, capability, or requirement was rejected;
correct it before planning.

**Next.** [Plan the immutable version](#4-generate-and-select-a-plan).

## 4. Generate and select a plan

**Prerequisites.** A workflow version, execution scope, compatible resources and
bindings, and network topology must exist. Query installed algorithms rather
than assuming PRISM or HEFT is enabled.

**Desktop steps.** Open the workflow and choose **Generate plan**. Select scope,
topology, algorithms, and optional constraints. Generate candidates, follow
progress, compare time, cost, feasibility and Pareto status, then select one.

![Planning form with target, scope, PRISM objectives, and HEFT.](/img/interface/planning/create-execution-plan.png)

**API alternative.** Query `GET /planning-algorithms/`, create a session with
`POST /planning-sessions/`, poll it, list candidates, and call the selected
candidate's `/select/` route. See [Plan a workflow](./workflows/planning).

**Expected result.** Selection returns a canonical plan whose assignments cover
every activity and refer to frozen workflow and infrastructure inputs.

**Asynchronous states and errors.** Creation returns `202 Accepted`; sessions
can be `queued`, `running`, `completed`, `failed`, or `cancelled`. Do not select
a candidate until it is complete, feasible, and its assignments are reviewed.

**Next.** [Start the selected plan](#5-start-and-monitor-a-run).

## 5. Start and monitor a run

**Prerequisites.** Use a selected or manually validated plan. Confirm its scope
selects the intended mode and connected resources remain healthy for a real run.

**Desktop steps.** Open the plan, choose its run action, inspect the run ID and
predicted allocation, and start execution. Open **Runs**, filter by workflow and
mode, then inspect activities, logs, placement, transfers, and observed timing.

![Workflow run history with mode, status, target, and progress.](/img/interface/runs/workflow-history.png)

**API alternative.** Submit a complete envelope to `POST /execution-runs/` and
poll `GET /execution-runs/{runId}/`. See [Execute and monitor](./workflows/executions).

**Expected result.** The run and every activity reach `completed`; detail
contains handles, events, timings, costs, and transfer evidence.

**Asynchronous states and errors.** Submission returns `202 Accepted`. Runs move
from `created` to `running`, then `completed` or `failed`. Inspect the first
failed activity, handle, exit code, log, and ordered events. Preparation failures
usually point to artifact, workspace, or transfer setup.

**Next.** [Inspect artifacts](#6-register-build-and-materialize-artifacts) and
[verify transfers](#7-transfer-and-retrieve-data).

## 6. Register, build, and materialize artifacts

**Prerequisites.** Choose an immutable OCI reference, uploaded build context, or
existing file on registered storage. Use a credential reference for a private
registry; never send the secret in an artifact payload.

**Desktop steps.** Open **Artifacts**. Register an OCI artifact or start a build,
then inspect immutable versions, locations, build records, and materializations.
For an existing `.sif`, browse it and choose **Register executable artifact**.

**API alternative.** Use `POST /artifacts/`; upload through
`POST /build-contexts/` and create `POST /artifact-builds/`, or promote an
existing storage path. See [Artifacts, storage, and builds](./data/artifacts).

**Expected result.** The artifact has an immutable version and digest. A
materialization is committed only when its verified digest matches that digest.

**Asynchronous states and errors.** Builds and materializations are asynchronous.
Materialization moves through `planned`, `reconciling`, `transferring`,
`verifying`, `committed`, or `failed`. A browser-local path is not a server build
context. Diagnose failures from build events, transfers, and digest evidence.

**Next.** Attach the version to a workflow command, or
[transfer data](#7-transfer-and-retrieve-data).

## 7. Transfer and retrieve data

**Prerequisites.** Source and destination storage must be registered, healthy,
and capable of the operation. Use only roots returned by the daemon. For
run-generated data, record the run and activity IDs before promotion.

**Desktop steps.** Open an environment's **Storage**, choose an approved root,
and browse to the entry. Download a file, archive a directory, copy, checksum,
or register it as a data object. In run detail, inspect observed route and bytes.

![Run detail showing transferred data and time decomposition.](/img/interface/runs/simgrid-run-decomposition.png)

**API alternative.** Use storage-owned `downloads`, `archives`, `copies`, and
`checksum` endpoints and poll the operation. Use `promote-data` to register an
existing file without moving it. See [Browse and manage storage](./infrastructure/storage).

**Expected result.** A download becomes ready before content is fetched; copy
or archive succeeds; promoted data links to workflow/run/activity context.
Execution transfers report strategy, route, duration, cost, and bytes.

**Asynchronous states and errors.** These operations may be queued. Preserve
pagination cursors and poll the operation ID, not the path. Disabled actions
mean unhealthy storage or a missing capability. Resolve path policy, connector,
checksum, or destination failure before treating bytes as retrieved.

**Next.** [Follow lineage and audit](#8-investigate-provenance-and-audit).

## 8. Investigate provenance and audit

**Prerequisites.** Start with a persisted run, plan, activity, transfer, data,
or operational target ID. Provenance must be configured for entity, SQL, and
lineage queries.

**Desktop steps.** In **Provenance → Explore**, choose a server-defined entity
and find the record. Open bounded lineage, use read-only SQL for cross-entity
questions, and open **Audit** to correlate operational actions.

![Lineage graph rooted at a completed run.](/img/interface/provenance/lineage-fanout.png)

![Operational audit events with targets and outcomes.](/img/interface/operations/audit-events.png)

**API alternative.** Discover `/provenance/entities/`, query an entity or
lineage route, submit read-only SQL, and filter `/audit-events/`. See
[Provenance and audit](./data/provenance-and-audit).

**Expected result.** Scientific lineage connects upstream/downstream evidence
without conflating it with chronological audit. Exports preserve root,
direction, depth, query, or filters.

**Asynchronous states and errors.** Reads are synchronous but can describe
running operations. The API returns `503` when provenance is unavailable and
rejects invalid entities, unsafe SQL, and invalid lineage. A truncated graph or
paged result is incomplete; preserve and adjust its bounds deliberately.

**Next.** Return to the identified run/target, or manage cloud and console access.

## 9. Manage cloud capacity lifecycle

**Prerequisites.** Use a supported compute provider. In v1.0 GCP supports
catalog discovery and Terraform provisioning; AWS is limited to S3-compatible
data movement. Store provider credentials in the daemon.

**Desktop steps.** Open a GCP environment's **Cloud capacity**. Refresh its
catalog, create a compatible capacity target, and optionally attach a validated
machine-configuration version. Start provisioning from the resource, follow the
operation, events, and logs, then validate the instance. Stop or destroy only
after confirming the resource and active operation.

**API alternative.** Refresh/read the cloud catalog, create a target, validate
and version machine configuration, queue provisioning, then poll the operation
and events. See [Cloud capacity](./infrastructure/cloud-capacity).

**Expected result.** The resource is linked to its environment and target,
configuration is recorded, and final validation confirms it is usable.

**Asynchronous states and errors.** Every catalog or lifecycle action is
asynchronous; a queued response is not capacity. Inspect Terraform,
configuration, and validation separately. IAM, quota, compatibility, network,
or playbook errors need operator correction. Failed cleanup may leave billable
resources; verify it in the provider console.

**Next.** Add the ready resource to a scope or open an interactive console.

## 10. Open and close an interactive console

**Prerequisites.** Select a resource with an interactive-capable runtime binding
and usable connection. For SSH, authorize the Engine key on every hop. Imported
read-only instances cannot be used.

**Desktop steps.** Open resource detail and its terminal action. Use the fixed
bottom panel and session tabs. Export logs only to an approved location and
explicitly close the session when finished.

**API alternative.** Create `POST /console-sessions/`, connect to its `/stream/`
WebSocket, then `DELETE` it. Use `POST /console-commands/` for repeatable
one-shot diagnostics. See [Interactive console](./operations/interactive-console).

**Expected result.** The session reaches `connected`, streams through the
daemon, closes with `204`, and produces audit evidence. A one-shot record keeps
status, stdout, stderr, and exit code.

**Asynchronous states and errors.** Sessions move through `starting`,
`connected`, `closed`, or `failed`. Creation returns `422` on resolution/startup
failure and `503` when unavailable. Changing tabs does not close the remote
session. Treat exported logs as sensitive.

**Next.** Correlate the session with [Audit](#8-investigate-provenance-and-audit)
and close every session no longer needed.

## Visual evidence policy

Images appear only where layout or visual relationships add information:
bootstrap readiness, real-versus-simulation catalogs, DAG entry points,
planning choices, run monitoring/decomposition, and evidence graphs. Artifact,
storage, cloud, and terminal procedures remain executable from text and detailed
payloads; machine-specific images would add unstable IDs, paths, hosts, or logs.

The canonical Electron capture size is **1280 × 860** at device scale factor 1.
The Xvfb screen may be larger so the window frame fits, but does not define the
output image. Browser-only documentation captures use their separate scripted
1440 × 900 viewport. Reviewed legacy captures retain their original pixels
until their screen is recaptured; they are not templates for new Electron work.
