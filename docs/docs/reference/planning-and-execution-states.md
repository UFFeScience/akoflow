---

title: Planning and execution states
description: Authoritative lifecycle states for planning sessions, algorithm runs, candidates, schedule plans, and execution runs.
---

import useBaseUrl from '@docusaurus/useBaseUrl';

# Planning and execution states

This reference distinguishes the state-bearing records used by AkôFlow planning and execution. It is intended for API clients, integrations, and readers interpreting the Desktop status badges. A queued durable job, a planning session, and an execution run are separate records; their status values must not be compared as if they were one state machine.

## Lifecycle overview

<img src={useBaseUrl('/img/architecture/planning-execution-states.svg')} alt="Planning flow from request to selected plan and separate execution flow from request through durable queue job to terminal evidence." />

A planning request is persisted as a session and publishes a queue job. An execution request first publishes a queue job; a workflow `ExecutionRun` is created only when the daemon starts processing that job. Consequently, an accepted execution request may not yet appear in `GET /execution-runs/`.

## Planning session

A `PlanningSession` freezes a workflow version, execution scope, network topology, algorithm selections, optional deadline/budget, and configuration input for one comparison. Read it with `GET /planning-sessions/{sessionId}/`; the response also includes its algorithm runs.

| State | How it is reached | Meaning | Terminal? |
| --- | --- | --- | --- |
| `queued` | Initial creation; a running session can be reset to queued when the coordinator begins work again. | The session is waiting for the daemon's serialized planning coordinator. | No |
| `running` | Coordinator acquires the planning lock and starts the session. | The session is resolving its frozen input and executing algorithm runs. | No |
| `completed` | At least one valid candidate exists, candidate ranks are persisted, and session progress becomes `1`. | Candidate comparison is complete. Selection is still a separate action. | Yes |
| `failed` | Input cannot be built, or no algorithm produces a valid candidate. `failureReason` records the cause. | No usable comparison was produced. | Yes |
| `cancelled` | `POST /planning-sessions/{sessionId}/cancel/` cancels a queued or running session. | The coordinator cancellation was requested and current queued/running algorithm runs are marked cancelled. | Yes |

The cancellation endpoint returns a conflict for a completed or failed session, and is idempotent for an already cancelled session. It does not delete candidates or plans created before cancellation.

### Algorithm runs

Each selected algorithm creates an `AlgorithmRun` within the session. It uses the same status values: `queued`, `running`, `completed`, `failed`, and `cancelled`.

Algorithm runs execute sequentially. A session can be `running` while one algorithm run is `running` and later runs remain `queued`. If one algorithm fails, the others continue. The session fails only when none produces a valid candidate.

`progress` is a fraction based on completed algorithm runs, not a count of evaluated schedules. `estimate` carries a predicted planning duration, search-space metadata, and confidence for one algorithm run.

## Candidates are not state machines

A `PlanCandidate` has no `status` field. It is an immutable generated option associated with one algorithm run and carries these disposition fields instead:

| Field | Meaning |
| --- | --- |
| `feasible` | The candidate passed plan validation and may be selected. Infeasible candidates cannot be promoted. |
| `rank` | Comparison order after session completion. Lower rank is preferred by the common ranking step. |
| `paretoOptimal` | The candidate is on the non-dominated time/cost frontier. |
| `dominated` | Another feasible candidate is no worse in both predicted metrics and better in at least one. |
| `fingerprint` | Deduplication identity for equivalent generated plans. |

Candidates may appear while an algorithm run and its parent session are still `running`. Their final rank/Pareto fields are assigned when the session completes. Use `GET /planning-sessions/{sessionId}/candidates/` for summaries and `GET /planning-sessions/{sessionId}/candidates/{candidateId}/` for the full embedded plan.

## Selecting a candidate and schedule plans

A `SchedulePlan` does **not** have a lifecycle status. It is the saved plan containing assignments, predicted metrics, algorithm/source information, and lifecycle actions. It can originate from a manual plan, an import, or a selected candidate.

`POST /planning-sessions/{sessionId}/candidates/{candidateId}/select/` promotes a feasible candidate. The API verifies that the candidate belongs to the session and is feasible, stores its embedded plan if it is new, and records `selectedCandidateId` and `selectedPlanId` on the session. It currently does not require the session to be `completed`, so clients should normally wait for completed ranking before selecting unless they deliberately choose an early candidate.

Selecting another feasible candidate updates the session's selected IDs; it does not invalidate the previously persisted plan. A schedule plan becomes execution evidence only after an execution run references its ID.

## Workflow execution runs

Workflow execution uses a distinct `ExecutionRun` lifecycle. Submit `POST /execution-runs/`; it returns an accepted queue job. Once the daemon consumes that job, the supervisor creates the run and begins execution.

| State | How it is reached | Meaning | Terminal? |
| --- | --- | --- | --- |
| `created` | Defined in the domain for a not-yet-started run. | Reserved state; the current workflow supervisor sets a workflow run to `running` before persisting it. | No |
| `running` | Daemon starts the workflow run. | The supervisor is simulating or dispatching activities. | No |
| `completed` | The complete execution trace, tasks, transfers, makespan, and cost are persisted. | Observed evidence is available. | Yes |
| `failed` | Preparation, provisioning, dispatch, polling, or persistence fails. `failureReason` records the cause. | The run did not complete successfully. | Yes |

There is no `cancelled` workflow-run status or workflow-run cancellation endpoint in the current API. Do not report a cancelled planning session as a cancelled execution, and do not infer that stopping a provider-side job updates a workflow run unless the daemon records the resulting failure.

### Task and handle states

Task records offer finer-grained evidence than the workflow-run badge:

| Record | States | Notes |
| --- | --- | --- |
| `TaskExecution` | `blocked`, `ready`, `preparing`, `running`, `completed`, `failed`, `cancelled` | The domain supports all values. The current supervisor persists running and completed task records for normal execution; failure/cancellation can be recorded from runtime outcomes. |
| `ActivityHandle` | `starting`, `running`, `completed`, `failed`, `stopped` | Runtime adapter identity, such as a Kubernetes Job, Slurm Job, PID, or simulation event. `stopped` is surfaced as a failed task with the handle failure reason in task reads. |

Task-stage totals such as `queueSeconds`, `transferSeconds`, and `runtimeSeconds` are accumulated over all tasks. They are diagnostic totals, not wall-clock makespan. Use the run's `makespanSeconds` and task timestamps to understand elapsed time.

## What to poll and what to verify

| Goal | Endpoint / evidence | Completion condition |
| --- | --- | --- |
| Wait for planning | `GET /planning-sessions/{sessionId}/` | Session is `completed`, `failed`, or `cancelled`. |
| Inspect planning progress | Algorithm-run statuses and estimates in the session response | Each intended algorithm is terminal. |
| Choose a generated plan | Candidate detail then select endpoint | Session records `selectedCandidateId` and `selectedPlanId`; `GET /schedule-plans/{planId}/` returns the plan. |
| Wait for execution | `GET /execution-runs/{runId}/` after the daemon creates the run | Run is `completed` or `failed`. |
| Explain an observed result | Run projection tasks, transfers, events, and Plan vs execution view | Task/transfer records account for the observed critical path. |

Related material: [planning a workflow](../guides/workflows/planning), [execution evidence](../guides/workflows/executions), [execution scopes and topologies](./execution-scopes-and-topologies), and [provenance and audit](../guides/data/provenance-and-audit).
