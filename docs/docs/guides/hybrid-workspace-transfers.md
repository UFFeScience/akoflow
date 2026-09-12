# Hybrid workspace transfers

AkôFlow can execute one frozen plan across local, Slurm, and cloud runtimes.
Every assignment selects a stable `resourceId` and `runtimeId`; the selected
runtime advertises how its workspace is addressed. The execution supervisor
does not branch on provider names.

## Runtime configuration

Set `capabilities.workspace.destinationUri` on every runtime that participates in a data
dependency. `sourceUri` is optional and defaults to `destinationUri`.

```yaml
runtimes:
  - id: local-runtime
    mode: execution
    configuration: {connectionId: local}
    capabilities:
      workspace:
        destinationUri: file:///var/lib/akoflow/workspaces/{runId}/{activityId}
  - id: slurm-a-runtime
    mode: execution
    configuration: {connectionId: slurm-a}
    capabilities:
      workspace:
        destinationUri: file:///scratch/akoflow/{runId}/{activityId}?connectionId={connectionId}
  - id: slurm-b-runtime
    mode: execution
    configuration: {connectionId: slurm-b}
    capabilities:
      workspace:
        destinationUri: file:///work/akoflow/{runId}/{activityId}?connectionId={connectionId}
  - id: cloud-runtime
    mode: execution
    configuration: {connectionId: cloud}
    capabilities:
      workspace:
        destinationUri: file:///akoflow/workspace/runs/{runId}/{activityId}?connectionId={connectionId}
```

Templates accept `{runId}`, `{activityId}`, `{connectionId}`, `{resourceId}`,
`{environmentId}`, `{runtimeId}`, `{cloudInstanceId}`, and `{bytes}`. Connection
capabilities and network domains resolve direct/shared routes; otherwise the
bounded AkôFlow gateway is used. A failed direct route records its fallback.

## Execution and integrity gate

Submit a normal execution request whose plan assignments select the four
runtime IDs. For every dependency AkôFlow constructs a content-addressed plan,
copies only missing bytes, validates the exact byte length and SHA-256 digest,
then atomically promotes the partial object. The consumer starts only after its
workspace materialization is `committed`.

`GET /executions/{runId}` returns `activities` and `dataTransfers`. Activities
include stable environment/resource/runtime identities. Transfers include both
endpoint identities, route and fallback, logical/network bytes, digests,
integrity status, and start/finish timestamps. Clients should render these
records and must not infer provider behavior.

## Recovery

Transfer-run and chunk-run state is persisted. After a process or lease retry,
AkôFlow verifies an already committed destination and skips it. Otherwise it
continues from the `.partial` length and persisted chunk attempts. Digest or
size failures remain `failed` and auditable; correct the endpoint/connectivity
problem and retry the same execution command so the stable transfer ID resumes
instead of creating a duplicate transfer.
