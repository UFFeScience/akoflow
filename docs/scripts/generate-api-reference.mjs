import { mkdir, readFile, readdir, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const docsDirectory = path.resolve(scriptDirectory, "..");
const routerFile = path.resolve(
  docsDirectory,
  "..",
  "internal/api/httpserver/httpserver.go",
);
const handlerFile = path.resolve(
  docsDirectory,
  "..",
  "internal/api/handlers/workflow_engine_api_handler/workflow_engine_api_handler.go",
);
const outputDirectory = path.resolve(docsDirectory, "docs/api/endpoints");
const manifestDirectory = path.resolve(docsDirectory, ".generated");
const manifestFile = path.join(manifestDirectory, "api-manifest.json");
const sourceRoot = path.resolve(docsDirectory, "..", "internal");

const groupRules = [
  [
    "Instance",
    /^\/(instance|instances|instance-activations|factory-reset|user-preferences|preflight|search)/,
  ],
  [
    "Environments",
    /^\/(environments|environment-connections|connection-tests|ssh-keys|kubernetes-tokens)/,
  ],
  ["Cloud", /^\/(cloud-|machine-configuration)/],
  ["Storage", /^\/(storages|storage-downloads)/],
  ["Resources", /^\/(resources|network-topologies|execution-scopes)/],
  ["Workflows", /^\/(workflow-definitions|workflow-definition-actions)/],
  ["Planning", /^\/(schedule-plans|planning-)/],
  ["Executions", /^\/execution-runs/],
  ["Artifacts", /^\/(artifacts|artifact-|build-)/],
  ["Provenance", /^\/provenance/],
  ["Console", /^\/console-/],
  ["Audit", /^\/audit-events/],
];

const methodOrder = { GET: 1, POST: 2, PUT: 3, PATCH: 4, DELETE: 5 };

const groupMetadata = {
  Instance: [
    "/docs/guides/operations/instance-management",
    "instance configuration, archives, preferences, and search",
  ],
  Environments: [
    "/docs/guides/infrastructure/environments",
    "environments and connections",
  ],
  Cloud: [
    "/docs/guides/infrastructure/cloud-capacity",
    "cloud capacity and machine configuration",
  ],
  Storage: [
    "/docs/guides/infrastructure/storage",
    "remote storage content and operations",
  ],
  Resources: [
    "/docs/guides/infrastructure/execution-scopes",
    "resources, network topology, and execution scopes",
  ],
  Workflows: ["/docs/guides/workflows/definitions", "workflow definitions"],
  Planning: ["/docs/guides/workflows/planning", "plans and planning sessions"],
  Executions: [
    "/docs/guides/workflows/executions",
    "workflow and interactive runs",
  ],
  Artifacts: [
    "/docs/guides/data/artifacts",
    "artifacts, materializations, and builds",
  ],
  Provenance: [
    "/docs/guides/data/provenance-and-audit",
    "scientific provenance",
  ],
  Console: [
    "/docs/guides/operations/interactive-console",
    "interactive console sessions and commands",
  ],
  Audit: ["/docs/guides/data/provenance-and-audit", "operational audit events"],
  System: ["/docs/getting-started", "daemon status"],
};

const queryDescriptions = {
  connectionId: "Limit results to this environment connection.",
  cursor: "Opaque cursor returned by the preceding page.",
  depth: "Maximum lineage traversal depth.",
  direction: "Lineage traversal direction.",
  environmentId: "Limit results to this environment.",
  eventType: "Limit results to this audit event type.",
  executionId: "Limit results to this execution run.",
  filterField: "Entity field to filter.",
  filterValue: "Value required in the selected filter field.",
  includeArtifacts:
    "Set to `true` to include artifact bytes in the exported archive.",
  kind: "Limit execution runs to this run kind.",
  limit: "Maximum number of results to return.",
  maxNodes: "Maximum number of nodes in the lineage graph.",
  mode: "Limit execution runs to this execution mode.",
  outcome: "Limit results to this audit outcome.",
  page: "One-based result page.",
  pageSize: "Number of results per page.",
  path: "Path inside the selected storage root.",
  q: "Free-text search query.",
  resourceId: "Limit results to this resource.",
  runId: "Limit results to this run.",
  sessionId: "Limit results to this session.",
  sortField: "Entity field used to order results.",
  sortOrder: "Sort direction for the selected field.",
  status: "Limit execution runs to this status.",
  types: "Comma-separated result types included in global search.",
};

const statusNames = {
  StatusOK: "200 OK",
  StatusCreated: "201 Created",
  StatusAccepted: "202 Accepted",
  StatusNoContent: "204 No Content",
};

// These handlers delegate their responses. The WebSocket upgrade writes 101;
// enqueueCloudOperation writes 202. Shallow handler scans cannot see either.
const delegatedSuccessStatuses = {
  StreamConsoleSession: ["101 Switching Protocols"],
  ProvisionCloudInstance: ["202 Accepted"],
  StartCloudProvisioning: ["202 Accepted"],
  ConfigureCloudInstance: ["202 Accepted"],
  DestroyCloudInstance: ["202 Accepted"],
  StartCloudInstance: ["202 Accepted"],
  StopCloudInstance: ["202 Accepted"],
  ValidateCloudInstance: ["202 Accepted"],
};

// Route-specific wording is reserved for aliases whose handler name cannot
// explain the public operation on its own.
const routeTitles = {
  "GET /": "Check daemon health",
  "GET /akoflow-api/preflight/": "Inspect daemon preflight",
  "POST /akoflow-api/workflow-definitions/import/":
    "Import workflow definition",
  "GET /akoflow-api/storages/{storageId}/entry/": "Inspect storage entry",
  "GET /akoflow-api/storages/{storageId}/stat/":
    "Inspect storage entry (compatibility route)",
};

// These files are submitted together by the versioned SimGrid first-run
// procedure. Each depends on IDs created earlier in that sequence.
const runnableSimulationRequests = {
  "POST /akoflow-api/environments/": "environment.yaml",
  "POST /akoflow-api/execution-scopes/": "scope.yaml",
  "POST /akoflow-api/network-topologies/": "topology.yaml",
  "POST /akoflow-api/workflow-definitions/": "workflow.yaml",
  "POST /akoflow-api/workflow-definitions/import/": "workflow.yaml",
  "POST /akoflow-api/schedule-plans/": "plan-request.yaml",
  "POST /akoflow-api/execution-runs/": "execution-request.yaml",
};

// These notes come from handler calls and the credential/operation services,
// not from JSON tags alone. Keep them scoped to fields the service validates.
const verifiedRequestNotes = {
  "GET /": "This health route is at the daemon root, outside `/akoflow-api/`. With the API URL from the [API access tutorial](/docs/tutorials/api-access), the cURL command removes that prefix and requests `/`. A healthy daemon returns plain text `ok`; this check does not verify Docker, BuildKit, or a workflow runtime. Use [preflight](/docs/api/endpoints/instance/get-preflight) for those local capability checks.",
  "GET /akoflow-api/console-sessions/{sessionId}/stream/": "Open this URL with a WebSocket client after creating a console session. A successful upgrade returns `101 Switching Protocols` and carries terminal input and output over the socket; it is not a JSON response. An unknown session returns `404`. See the [interactive console guide](/docs/guides/operations/interactive-console) for session lifecycle.",
  "POST /akoflow-api/provenance/sql/": "Send a read-only `sql` query using `SELECT` or `WITH`; `parameters` supplies optional named values, and `page`/`pageSize` control results (at most 200 rows per page). Use `GET /provenance/sql/schema/` to see allowed tables and columns. The service enforces a 10-second timeout and rejects writes or restricted fields with `400`; an unavailable explorer returns `503`. See [provenance and audit](/docs/guides/data/provenance-and-audit#query-with-read-only-sql).",
  "POST /akoflow-api/provenance/sql/explain/": "Send the same `sql` and optional named `parameters` as the read-only SQL route. This runs `EXPLAIN QUERY PLAN` for a permitted `SELECT` or `WITH` statement and returns plan rows, not the query's data rows. It uses the same read-only table/column restrictions and 10-second timeout; invalid SQL returns `400` and an unavailable explorer returns `503`.",
  "PUT /akoflow-api/instance/": "Send the complete current instance object with non-empty `id` and `name`; this route saves the supplied object, so preserve existing identity and metadata when changing one field. `transferBufferBytes` accepts 5–64 MiB; `0` selects the 8 MiB default. The [instance guide](/docs/guides/operations/instance-management#inspect-the-active-identity) reads the current object before updating it.",
  "PUT /akoflow-api/environments/{environmentId}/": "Read `GET /environments/{environmentId}/` before editing and send a complete environment definition; `environment.id` must match the path ID. Replacement returns `404` when the environment does not exist and can return `422` when references prevent replacing its inventory. Use new environment and version IDs for revised inventory already used by scopes or plans; see the [environment YAML reference](/docs/reference/environment-yaml#compatibility-and-common-failures).",
  "POST /akoflow-api/instance-activations/{instanceId}/": "Use `default` to return to the writable instance or an ID returned by `POST /instances/import/` to open a read-only snapshot. The response is `202 Accepted` with `instance` and `restarting`; when `restarting` is false, restart the server manually. See [instance management](/docs/guides/operations/instance-management#import-and-open-a-read-only-snapshot).",
  "POST /akoflow-api/instances/import/": "Send a ZIP archive as the request body with `Content-Type: application/zip`, not JSON. The archive must be a compatible AkôFlow export with its redaction marker, valid database checksum and schema, no symbolic links, at most 100,000 entries, at most 8 GiB compressed and 64 GiB expanded. Success returns `201 Created` with the new read-only snapshot `id`; import does not replace the active instance. See [instance management](/docs/guides/operations/instance-management#import-and-open-a-read-only-snapshot).",
  "PUT /akoflow-api/user-preferences/{clientId}/": "The path `clientId` must be 8–128 characters and overrides any body `clientId`. Send `theme` as `light` or `dark`; `animationsEnabled` is optional and defaults to false when omitted. The route saves preferences for this client profile and returns the stored record with `200 OK`; invalid ID or theme returns `422`. See [instance preferences](/docs/guides/operations/instance-management#personal-preferences).",
  "POST /akoflow-api/factory-reset/": "No JSON body is required. This permanently clears user catalog/database records while retaining the schema and system instance identity; the server also removes managed Kubernetes token files. Success is `204 No Content`. The database reset runs before token-directory cleanup, so a `422` from that cleanup can arrive after the catalog has already been cleared. Export a snapshot first; see [instance management](/docs/guides/operations/instance-management#factory-reset).",
  "PUT /akoflow-api/environment-connections/{connectionId}/": "Read the current connection in `GET /environments/{environmentId}/`, then send its complete object with the changed fields. Its `id` must match the path and `environmentId` must be non-empty. The route inserts a new connection or replaces saved fields, and returns the submitted object without running a health check. See the [HPC connection guide](/docs/guides/infrastructure/hpc-slurm#1-create-the-ssh-credential-and-proxy-aware-connection).",
  "POST /akoflow-api/environment-connections/{connectionId}/health/": "Checks a saved connection and records a health result. Inspect the returned `status` and `message`: an unreachable target returns `200 OK` with `status: offline` when the probe itself completed. The environment status is updated to `connected` or `unreachable`.",
  "POST /akoflow-api/environment-connections/{connectionId}/discover/": "Discovers through a saved connection and returns a `snapshots` array. The connection needs a configured discovery driver and bound resources; missing prerequisites return `422`. A successful request records observed inventory but does not reserve a scheduler allocation or guarantee that every site resource was found.",
  "POST /akoflow-api/workflow-definitions/import/": "This compatibility route uses the same portable YAML/JSON importer as `POST /workflow-definitions/`. Supply `name`, `spec.namespace`, and activities with valid dependencies and execution or simulation fields. The [workflow specification](/docs/internal/workflow-spec) defines the authoring format; a duplicate normalized workflow ID returns `422`.",
  "POST /akoflow-api/workflow-definition-actions/duplicate/{workflowId}/": "Send a non-empty new `name`; `namespace` is optional and otherwise inherited. The source workflow must exist. AkôFlow generates new workflow and activity IDs from the new name and returns the independent definition with `201 Created`; an ID collision or invalid name returns `422`.",
  "POST /akoflow-api/planning-sessions/{sessionId}/candidates/{candidateId}/select/": "The candidate must belong to this session and be feasible. No JSON body is required. A successful selection returns the saved schedule plan with `201 Created` and records the selected IDs on the session. The API permits selecting before the session completes; wait for final ranking unless choosing an early candidate intentionally.",
  "POST /akoflow-api/planning-sessions/{sessionId}/cancel/": "No JSON body is required. Cancelling a queued or running session marks its algorithm runs and session as cancelled and requests cancellation of active work; success returns `204 No Content`. Cancelling an already-cancelled session also returns `204`. A missing, completed, or failed session returns `409 Conflict`, not `404`.",
  "POST /akoflow-api/environments/{environmentId}/cloud-catalog/refresh/": "Discovers the catalog using the saved cloud connection for this environment and returns the discovered catalog with `200 OK`. This is a live provider call, not a VM provisioning request. A discovery error returns `422`; `GET /environments/{environmentId}/cloud-catalog/` returns `404` until a catalog has been synchronized.",
  "POST /akoflow-api/environments/{environmentId}/cloud-capacity-targets/": "Use an existing cloud environment with a cloud runtime. Send a target definition with `name`, `providerMachineType`, and `imageReference`; the server supplies an omitted `id`, sets `environmentId` from the path, and enables the target. Provider region, image, machine type, and network policy are account-specific; use the [GCP guide](/docs/guides/infrastructure/gcp) before creating one. The route creates a schedulable capacity resource but no VM. Set approved `configuration.sshSourceRanges` before provisioning: the current Terraform target otherwise defaults SSH ingress to `0.0.0.0/0`. If creation returns `422`, inspect the target list before retrying because target persistence precedes resource binding.",
  "DELETE /akoflow-api/cloud-capacity-targets/{targetId}/": "No body is required. An active provisioned instance using this target blocks removal with `409 Conflict`; destroy it first. Success returns `204 No Content`, disables and renames the target record, and marks its capacity resource unschedulable. A missing or already-disabled target returns `404`. This does not itself destroy a VM.",
  "POST /akoflow-api/cloud-instances/{instanceId}/configure/": "No JSON body is required. The instance ID must exist (`404` otherwise). This returns a queued cloud operation with `202 Accepted`; inspect its status and events before treating configuration as complete. The worker needs a public address and the target's machine-configuration versions; missing prerequisites can fail asynchronously. Repeating the same active action returns its existing operation; a different active action on the instance returns `409`.",
  "POST /akoflow-api/cloud-instances/{instanceId}/destroy/": "No JSON body is required. The instance ID must exist (`404` otherwise). `202 Accepted` queues destruction; inspect the operation until it completes or fails before assuming the VM is gone. A lifecycle safety check can block destruction after acceptance. The same active action returns its existing operation; another active action on this instance returns `409`.",
  "POST /akoflow-api/cloud-instances/{instanceId}/start/": "No JSON body is required. The instance ID must exist (`404` otherwise). `202 Accepted` queues a start operation; the worker accepts a stopped instance and treats an already-ready instance as a no-op. Other states can fail asynchronously. A repeated active start returns the existing operation; a different active action returns `409`.",
  "POST /akoflow-api/cloud-instances/{instanceId}/stop/": "No JSON body is required. The instance ID must exist (`404` otherwise). `202 Accepted` queues a stop operation; the worker accepts a ready instance and treats an already-stopped instance as a no-op. Other states or lifecycle safety checks can fail after acceptance. A repeated active stop returns the existing operation; a different active action returns `409`.",
  "POST /akoflow-api/cloud-instances/{instanceId}/validate/": "No JSON body is required. The instance ID must exist (`404` otherwise). `202 Accepted` queues validation against the target's configured checks; it does not by itself provision, configure, start, or stop the instance. Inspect the operation status and events for the result. A repeated active validation returns the existing operation; a different active action returns `409`.",
  "DELETE /akoflow-api/environments/{environmentId}/": "No body is required. The environment must exist (`404` otherwise) and must not be referenced by an execution scope, network topology, schedule plan, execution data or transfers, console activity, or simulation scenario (`422` if referenced). Success returns `204 No Content` and removes its saved versions, inventory, and connections. Inspect dependencies before deleting; see [environments](/docs/guides/infrastructure/environments).",
  "DELETE /akoflow-api/storages/{storageId}/entries/": "Pass the entry path as a URL query parameter, for example `?path=/approved/root/file.txt`; there is no JSON body. The storage must be registered, writable, and usable, and the path must be allowed by its adapter. Success returns `204 No Content`; adapter or policy errors return `422`. This removes the entry rather than merely deleting its catalog record.",
  "POST /akoflow-api/resources/": "The example adds an inventory-only resource to the SimGrid environment from the [first-run tutorial](/docs/guides/workflows/first-run); that environment version must already exist. Use a unique `id`: reusing one updates the resource and still returns `201 Created`. This route does not create a runtime binding, so the example is not a runnable planning resource. Add an enabled binding through a complete environment definition before selecting it in a plan.",
  "DELETE /akoflow-api/execution-scopes/{scopeId}/": "No body is required. A missing scope returns `404`; a scope referenced by a saved schedule plan cannot be deleted and returns `422`. Success returns `204 No Content`. Inspect plans before deleting a scope used by planning or execution.",
  "DELETE /akoflow-api/console-sessions/{sessionId}/": "No body is required. Close a session when its terminal work is finished; the route returns `204 No Content` on success and `404` for an unknown session. Closing the Desktop detail page alone does not close a daemon-owned session. See the [interactive console guide](/docs/guides/operations/interactive-console).",
  "POST /akoflow-api/ssh-keys/": "The tested example creates a new key. Choose an unused `id`: 1–64 ASCII letters, digits, `_`, or `-`, starting with a letter or digit. `comment` is optional. The daemon needs `ssh-keygen`.",
  "POST /akoflow-api/ssh-keys/import/": "Send a unique `id` and your own `privateKey` as a JSON string. The ID follows the same SSH key ID rule; the key must be a non-empty OpenSSH private key that `ssh-keygen -y` can read. The response returns public metadata, not the private key. No example key is supplied because its bytes must come from your credential store.",
  "POST /akoflow-api/kubernetes-tokens/": "Send an `id` and your own non-empty `token` as JSON strings. The ID must be 1–63 lowercase letters, digits, or hyphens, starting with a letter or digit. The response returns a `credentialRef`; it does not echo the token. Supply the token from your cluster's credential process.",
  "POST /akoflow-api/cloud-credentials/": "Send `id`, `provider`, and `credential` with your actual provider credential. The ID follows the Kubernetes credential ID rule; `provider` must be `gcp`, `aws`, or `azure`, and `credential` must be valid JSON. Saving a credential does not validate provider access or make every provider operation available. The response returns a `credentialRef`.",
  "POST /akoflow-api/environments/{environmentId}/cloud-instances/": "Send a `capacityTargetId` for an existing target in this environment. `name` and `sshUsername` are optional; the server generates an `instanceId` when omitted. `202 Accepted` returns a queued operation, not a ready VM. Inspect its status and events; an active provision for the same target returns the existing operation. Follow the [GCP guide](/docs/guides/infrastructure/gcp) for account-specific setup.",
  "POST /akoflow-api/environments/{environmentId}/cloud-provisioning/": "This compatibility route accepts the same `capacityTargetId` request as [Provision Cloud Instance](/docs/api/endpoints/environments/post-environments-environmentid-cloud-instances). It queues the same operation and returns `202 Accepted`; inspect status and events before treating the VM as ready.",
  "POST /akoflow-api/planning-sessions/": "Required: `id`, an existing `workflowVersionId`, `executionScopeId`, and `networkTopologyId`, plus at least one `algorithms` entry. Each algorithm ID must appear in `GET /planning-algorithms/`; duplicate IDs are rejected. The server sets status and timestamps. The example IDs require the SimGrid environment, scope, topology, and workflow to be registered first.",
  "POST /akoflow-api/schedule-plans/import/": "Send a JSON object with a complete `plan`. Use a unique plan `id` and matching assignment `planId` values. Its workflow version, execution scope, topology, and resources must already exist. The server sets `source` to `imported`, checks the schedule against those saved records, and returns `422` if it is invalid. The response shape below shows the plan fields; a body filled with placeholder IDs would not pass validation.",
  "POST /akoflow-api/storages/{storageId}/promote-data/": "Replace the example `path` with an existing file within the selected storage's approved root. `id` is optional; the server generates one when omitted. `workflowVersionId`, `runId`, and `activityId` are optional associations and should identify real records when supplied. The [storage guide](/docs/guides/infrastructure/storage) shows the browsing step.",
  "POST /akoflow-api/storages/{storageId}/promote-artifact/": "Replace the example `path` with an existing `.sif` file within the selected storage's approved root. `id`, `name`, `version`, and `scope` have server defaults. The [storage guide](/docs/guides/infrastructure/storage) shows the browsing step; no file is uploaded or moved.",
  "POST /akoflow-api/storages/{storageId}/downloads/": "Replace the example `path` with an existing file within the selected storage's approved browse root; directories require the archive route. `id` is optional. The response is a ready download record: use its ID with `GET /storage-downloads/{downloadId}/content/` to stream the file.",
  "POST /akoflow-api/storages/{storageId}/checksum/": "Replace the example `path` with an existing file within the selected storage's approved browse root. The server reads the file and returns a `sha256:`-prefixed checksum; it does not queue a background job.",
  "POST /akoflow-api/storages/{storageId}/copies/": "Replace the example `path` with a file in the source storage and `destinationStorageId` with a registered, writable storage ID. `id` is optional. The copy runs in the background at the same path in the destination. Inspect `GET /storage-downloads/{downloadId}/` until its status is `completed` or `failed`; `202 Accepted` does not mean the bytes have arrived.",
  "POST /akoflow-api/storages/{storageId}/archives/": "Replace the example `path` with an existing directory in a writable storage. `id` is optional. AkôFlow writes a `.tar.gz` beside that directory and returns a queued record. Inspect `GET /storage-downloads/{downloadId}/` until its status is `ready` or `failed`; only a ready archive can be streamed through the download content route.",
  "POST /akoflow-api/storages/{storageId}/index-runs/": "Indexing must be enabled for this registered storage. Use a unique `id` in place of the example or omit it for a generated ID. The current service completes the bounded scan before responding; the returned record is `completed` or the request fails. The `202 Accepted` status does not mean this scan continues in the background.",
  "POST /akoflow-api/build-contexts/": "To upload bytes, send multipart form data with file field `context`. The JSON form only records metadata for bytes already in the artifact store; it requires `digest`, `storageUri`, and positive `sizeBytes`. A browser-local path is not a server build context.",
  "POST /akoflow-api/artifact-builds/": "Upload the build context first and use its returned digest as `contextDigest`; `artifactVersionId` must identify a saved artifact version. Required fields are `id`, `artifactVersionId`, `contextDigest`, `recipeDigest`, and `cacheKey`. An existing cache key returns that build with `200 OK`; a new specification returns `201 Created`. Creating a specification does not start a build run. See [Build an executable](/docs/guides/data/artifacts#build-an-executable-from-a-docker-image) for the simpler Docker-image path.",
  "POST /akoflow-api/artifacts/docker/": "Required: `artifactId`, `version`, and a Docker image reference without whitespace. `architecture` defaults to `amd64`. The response contains `artifact` and `build` objects. This registers a version and build specification; the registry pull and SIF conversion begin only after `POST /artifact-builds/{buildId}/runs/`.",
  "POST /akoflow-api/artifact-builds/{buildId}/runs/": "`buildId` must identify an existing build specification. This request starts a build run and returns its record with `202 Accepted`; inspect `GET /build-runs/{runId}/` for its outcome. The request has no JSON body.",
  "POST /akoflow-api/artifact-materializations/": "This endpoint stores a caller-supplied record; it does not copy or verify artifact bytes. Supply a distinct `id`, existing `variantId` and `resourceId`, the variant's `sha256:` digest, `destinationPath`, and a truthful `status`. The database checks references and digest format but does not prove the bytes exist at the destination. If supplied, `environmentId` must be the environment **version** ID, despite the field name. Use the [Artifacts guide](/docs/guides/data/artifacts#locations-and-materializations) to inspect evidence recorded by execution.",
  "POST /akoflow-api/console-sessions/": "Replace the example `resourceId` with a saved resource that has a connected interactive runtime; `actorId` is optional. A successful request starts the terminal and returns a `connected` session. Resolution or startup failure returns `422`, not a successful session record. Use the [console guide](/docs/guides/operations/interactive-console) for streaming and closure.",
  "POST /akoflow-api/console-commands/": "Replace the example `resourceId` with a saved resource that supports one-shot commands. `resourceId` and `command` are required; `timeoutSeconds` defaults to 30 and cannot exceed 3600. The request waits for the runner and returns a command record. A runner failure can return `201 Created` with `status: failed`; inspect `status`, `exitCode`, and `failure` instead of treating HTTP status as command success.",
  "POST /akoflow-api/connection-tests/": "The example tests the local server. For SSH, agent, or Kubernetes, send the connection fields and credentials required by that type instead. This route tests the supplied connection without saving it. It returns `200 OK` with `healthy` and `message`; `healthy: false` means the probe failed even though the HTTP request succeeded. Cloud connections are not handled here.",
  "POST /akoflow-api/machine-configuration-validations/": "Send `playbookYaml` containing the Ansible playbook text. Valid input returns `200 OK` with `valid: true` and a content hash; invalid input returns `422` with `valid: false` and errors. This validates structure only and does not run a playbook or provision a machine.",
  "POST /akoflow-api/machine-configurations/": "`name` is required. `id` is optional and generated when omitted. The server sets `ownership: user` and `enabled: true`. Create a version separately; this request does not validate or execute a playbook.",
  "POST /akoflow-api/machine-configurations/{configurationId}/versions/": "Create the configuration first, then use its returned ID as `configurationId` (the preceding example uses `example-machine-setup`). The path ID overrides any body `machineConfigurationId`. Send a positive `version` and valid `playbookYaml`; `id` is optional and `status` defaults to `draft`. The response is the stored configuration with its versions, not just the new version.",
  "POST /akoflow-api/cloud-credentials/validate/": "Send `provider`, your actual provider `credential` JSON, and the project/region fields required by that provider. This route performs a live catalog discovery and does not save the credential. The current server registers only a GCP catalog adapter; AWS or Azure validation returns `422` as unsupported even though those credential records can be stored. No generic credential body can establish access; use the [GCP guide](/docs/guides/infrastructure/gcp) for the current provider path.",
};

const verifiedRequestExamples = {
  "POST /akoflow-api/ssh-keys/": {
    id: "generated",
    comment: "test",
  },
  "POST /akoflow-api/resources/": {
    id: "inventory-only-node",
    environmentVersionId: "simulation-example-v1",
    type: "fog_device",
    name: "Inventory-only node",
    providerId: "inventory-only-node",
    cpuCores: 2,
    cpuCapacity: 2,
    memoryBytes: 2147483648,
    computeSpeedup: 1,
    schedulable: false,
  },
  "POST /akoflow-api/workflow-definition-actions/duplicate/{workflowId}/": {
    name: "Copied workflow",
    namespace: "copied",
  },
  "PUT /akoflow-api/user-preferences/{clientId}/": {
    theme: "dark",
    animationsEnabled: false,
  },
  "POST /akoflow-api/connection-tests/": {
    type: "local",
  },
  "POST /akoflow-api/console-sessions/": {
    resourceId: "my-interactive-resource",
  },
  "POST /akoflow-api/console-commands/": {
    resourceId: "my-interactive-resource",
    command: "hostname",
  },
  "POST /akoflow-api/storages/{storageId}/downloads/": {
    path: "/shared/project/result.csv",
  },
  "POST /akoflow-api/storages/{storageId}/checksum/": {
    path: "/shared/project/result.csv",
  },
  "POST /akoflow-api/storages/{storageId}/copies/": {
    path: "/shared/project/result.csv",
    destinationStorageId: "storage-archive",
  },
  "POST /akoflow-api/storages/{storageId}/archives/": {
    path: "/shared/project/experiment",
  },
  "POST /akoflow-api/storages/{storageId}/index-runs/": {
    id: "example-index-run-1",
  },
  "POST /akoflow-api/storages/{storageId}/promote-data/": {
    path: "/shared/project/result.csv",
  },
  "POST /akoflow-api/storages/{storageId}/promote-artifact/": {
    path: "/shared/bin/model.sif",
    name: "model",
    version: "1.0.0",
  },
  "POST /akoflow-api/artifacts/docker/": {
    artifactId: "busybox",
    version: "1.36",
    image: "docker.io/library/busybox:1.36",
    architecture: "amd64",
  },
  "POST /akoflow-api/machine-configuration-validations/": {
    playbookYaml: "- hosts: all\n  tasks:\n    - ansible.builtin.debug:\n        msg: ready\n",
  },
  "POST /akoflow-api/machine-configurations/": {
    id: "example-machine-setup",
    name: "Example machine setup",
  },
  "POST /akoflow-api/machine-configurations/{configurationId}/versions/": {
    version: 1,
    playbookYaml: "- hosts: all\n  tasks:\n    - ansible.builtin.debug:\n        msg: ready\n",
  },
  "POST /akoflow-api/provenance/sql/": {
    sql: "SELECT id, status FROM execution_runs WHERE status = :status",
    parameters: { status: "completed" },
    page: 1,
    pageSize: 50,
  },
  "POST /akoflow-api/provenance/sql/explain/": {
    sql: "SELECT id, status FROM execution_runs WHERE status = :status",
    parameters: { status: "completed" },
  },
  "POST /akoflow-api/planning-sessions/": {
    id: "planning-simulation-example",
    workflowVersionId: "simulation-example-workflow-v1",
    executionScopeId: "simulation-example-v1-scope",
    networkTopologyId: "simulation-network-v1",
    algorithms: [{ id: "heft", configuration: {} }],
  },
};

const requestWithoutStandaloneExample = new Set([
  "POST /akoflow-api/schedule-plans/import/",
  "POST /akoflow-api/environments/{environmentId}/cloud-capacity-targets/",
  "POST /akoflow-api/environments/{environmentId}/cloud-instances/",
  "POST /akoflow-api/environments/{environmentId}/cloud-provisioning/",
  "POST /akoflow-api/ssh-keys/import/",
  "POST /akoflow-api/kubernetes-tokens/",
  "POST /akoflow-api/cloud-credentials/",
  "POST /akoflow-api/cloud-credentials/validate/",
  "PUT /akoflow-api/instance/",
  "PUT /akoflow-api/environments/{environmentId}/",
  "PUT /akoflow-api/environment-connections/{connectionId}/",
  "POST /akoflow-api/artifact-builds/",
  "POST /akoflow-api/artifact-materializations/",
]);

function humanizeHandler(handler) {
  const phrase = handler
    .replace(/([A-Z]+)([A-Z][a-z])/g, "$1 $2")
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .trim();
  return phrase.replace(/^Get /, "Retrieve ");
}

function endpointPath(routePath) {
  return routePath.replace(/^\/akoflow-api/, "") || "/";
}

function groupFor(routePath) {
  const relative = endpointPath(routePath);
  return (
    groupRules.find(([, pattern]) => pattern.test(relative))?.[0] || "Instance"
  );
}

function slugFor(method, routePath) {
  const pathPart =
    endpointPath(routePath)
      .replace(/[{}]/g, "")
      .replace(/[^a-zA-Z0-9]+/g, "-")
      .replace(/^-|-$/g, "")
      .toLowerCase() || "health";
  return `${method.toLowerCase()}-${pathPart}`;
}

function extractParameters(routePath) {
  return [...routePath.matchAll(/\{([^}]+)\}/g)].map((match) => match[1]);
}

function replaceParameters(routePath) {
  return routePath.replace(/\{([^}]+)\}/g, (_, name) => `<${name}>`);
}

function extractFunctionBody(source, handler) {
  const match = new RegExp(
    `func (?:\\(h \\*Handler\\) )?${handler}\\([^\\n]*\\) \\{`,
  ).exec(source);
  if (!match) return "";
  const start = match.index + match[0].length;
  let depth = 1;
  for (let index = start; index < source.length; index += 1) {
    if (source[index] === "{") depth += 1;
    if (source[index] === "}" && --depth === 0)
      return source.slice(start, index);
  }
  return "";
}

function extractQueryParameters(body) {
  return [
    ...new Set(
      [...body.matchAll(/r\.URL\.Query\(\)\.(?:Get|Has)\("([^"]+)"\)/g)].map(
        (match) => match[1],
      ),
    ),
  ].sort();
}

function extractSuccessStatuses(body, handler) {
  const statuses = new Set();
  for (const match of body.matchAll(
    /http\.(Status(?:OK|Created|Accepted|NoContent))/g,
  )) {
    if (statusNames[match[1]]) statuses.add(statusNames[match[1]]);
  }
  if (/\b(?:writeList|writeItem)\(/.test(body)) statuses.add("200 OK");
  // These handlers write response bytes without WriteHeader: Go uses 200.
  if (
    statuses.size === 0 &&
    [
      "ExportArchiveInstance",
      "ExportConsoleSessionLog",
      "Preflight",
      "StreamBuildOutput",
      "StreamDownload",
    ].includes(handler)
  )
    statuses.add("200 OK");
  if (statuses.size === 0)
    throw new Error(`Cannot infer a success status for ${handler}`);
  return [...statuses];
}

async function collectGoSources(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const sources = [];
  for (const entry of entries) {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) sources.push(...(await collectGoSources(target)));
    else if (entry.name.endsWith(".go") && !entry.name.endsWith("_test.go")) {
      sources.push(await readFile(target, "utf8"));
    }
  }
  return sources;
}

function parseJSONFields(structBody) {
  const fields = [];
  for (const line of structBody.split("\n")) {
    const tagged =
      /^\s*([A-Za-z][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z][A-Za-z0-9_]*)*)\s+([^`]+?)\s+`json:"([^",]+)[^`]*`/.exec(
        line,
      );
    if (tagged) {
      if (tagged[3] !== "-")
        fields.push({ name: tagged[3], type: tagged[2].trim() });
      continue;
    }
    const untagged =
      /^\s*([A-Z][A-Za-z0-9_]*(?:\s*,\s*[A-Z][A-Za-z0-9_]*)*)\s+([^\s/]+)\s*$/.exec(
        line,
      );
    if (untagged) {
      for (const name of untagged[1].split(",").map((value) => value.trim())) {
        fields.push({ name, type: untagged[2] });
      }
    }
  }
  return fields;
}

function buildStructIndex(sources) {
  const index = new Map();
  const qualified = new Map();
  const aliases = new Map();
  for (const sourceText of sources) {
    const packageName = /^package\s+([A-Za-z0-9_]+)/m.exec(sourceText)?.[1];
    for (const match of sourceText.matchAll(
      /type\s+([A-Za-z0-9_]+)\s+struct\s*\{([\s\S]*?)\n\}/g,
    )) {
      const fields = parseJSONFields(match[2]);
      if (fields.length > 0) {
        const existing = index.get(match[1]);
        // Distinct packages can use the same short type name. Never borrow
        // one package's fields for an unrelated request.
        if (existing === null || (existing && JSON.stringify(existing) !== JSON.stringify(fields))) {
          index.set(match[1], null);
        } else if (!existing) {
          index.set(match[1], fields);
        }
        if (packageName) {
          const key = `${packageName}.${match[1]}`;
          const known = qualified.get(key);
          qualified.set(key, known && JSON.stringify(known) !== JSON.stringify(fields) ? null : fields);
        }
      }
    }
    for (const match of sourceText.matchAll(
      /type\s+([A-Za-z0-9_]+)\s*=\s*((?:[A-Za-z0-9_]+\.)?[A-Za-z0-9_]+)/g,
    )) {
      aliases.set(match[1], match[2]);
    }
  }
  for (const [alias, target] of aliases) {
    // A domain alias must not replace a concrete type with the same name in
    // another package (for example console.Request vs simgrid.Request).
    if (index.has(alias)) continue;
    const fields = target.includes(".") ? qualified.get(target) : index.get(target);
    if (fields !== undefined) index.set(alias, fields);
  }
  return index;
}

function buildReturnTypeIndex(sources) {
  const index = new Map();
  const add = (name, signature) => {
    const first = signature.split(",")[0].trim();
    if (!first || first === "error") return;
    const values = index.get(name) || new Set();
    values.add(first);
    index.set(name, values);
  };
  for (const sourceText of sources) {
    for (const match of sourceText.matchAll(
      /^\s*([A-Z][A-Za-z0-9_]*)\([^\n]*\)\s+\(([^\n)]+)\)/gm,
    ))
      add(match[1], match[2]);
    for (const match of sourceText.matchAll(
      /^func\s+(?:\([^)]*\)\s*)?([A-Z][A-Za-z0-9_]*)\([^\n]*\)\s+\(([^\n)]+)\)/gm,
    ))
      add(match[1], match[2]);
  }
  return index;
}

function buildOwnerReturnTypeIndex(sources) {
  const index = new Map();
  for (const sourceText of sources) {
    for (const interfaceMatch of sourceText.matchAll(
      /type\s+([A-Za-z0-9_]+)\s+interface\s*\{([\s\S]*?)\n\}/g,
    )) {
      for (const method of interfaceMatch[2].matchAll(
        /^\s*([A-Z][A-Za-z0-9_]*)\([^\n]*\)\s+\(([^\n)]+)\)/gm,
      ))
        index.set(
          `${interfaceMatch[1]}.${method[1]}`,
          method[2].split(",")[0].trim(),
        );
    }
    for (const method of sourceText.matchAll(
      /^func\s+\([^)]*\*?([A-Za-z0-9_]+)\)\s+([A-Z][A-Za-z0-9_]*)\([^\n]*\)\s+\(([^\n)]+)\)/gm,
    ))
      index.set(`${method[1]}.${method[2]}`, method[3].split(",")[0].trim());
  }
  return index;
}

function extractHandlerFieldTypes(handlerSource) {
  const body =
    /type\s+Handler\s+struct\s*\{([\s\S]*?)\n\}/.exec(handlerSource)?.[1] || "";
  return new Map(
    [...body.matchAll(/^\s*([a-z][A-Za-z0-9_]*)\s+([^\s]+)\s*$/gm)].map(
      (match) => [match[1], match[2].replace(/^\*/, "").split(".").at(-1)],
    ),
  );
}

function sampleValue(typeName, structIndex, depth = 0) {
  const clean = typeName.replace(/^\*+/, "").trim();
  if (/^\[\]/.test(clean))
    return [sampleValue(clean.replace(/^\[\]/, ""), structIndex, depth + 1)];
  if (/^map\[/.test(clean) || /^(?:any|interface\{\})$/.test(clean)) return {};
  if (/bool$/.test(clean)) return true;
  if (/(?:^|\.)(?:Time|Duration)$/.test(clean)) return "2026-01-01T00:00:00Z";
  if (/(?:^|\b)(?:u?int(?:8|16|32|64)?|float(?:32|64))$/.test(clean)) return 0;
  const shortName = clean.split(".").at(-1);
  const fields = structIndex.get(shortName);
  if (fields === null) return null;
  if (fields && depth < 3) {
    return Object.fromEntries(
      fields.map((field) => [
        field.name,
        sampleValue(field.type, structIndex, depth + 1),
      ]),
    );
  }
  return "string";
}

function extractRequestContract(body, structIndex) {
  const decoded = /decode\(w, r, &([A-Za-z0-9_]+)\)/.exec(body)?.[1];
  if (!decoded) return null;
  const inline = new RegExp(
    `var\\s+${decoded}\\s+struct\\s*\\{([\\s\\S]*?)\\}`,
  ).exec(body);
  if (inline)
    return {
      type: "inline request",
      example: Object.fromEntries(
        parseJSONFields(inline[1]).map((field) => [
          field.name,
          sampleValue(field.type, structIndex),
        ]),
      ),
    };
  const named = new RegExp(`var\\s+${decoded}\\s+([^\\s]+)`).exec(body)?.[1];
  return {
    type: named || "JSON request",
    example: sampleValue(named || "", structIndex),
  };
}

function extractResponseContract(
  endpoint,
  body,
  structIndex,
  returnTypeIndex,
  ownerReturnTypeIndex,
  handlerFieldTypes,
) {
  const responseTypeOverrides = {
    ConfigureCloudInstance: "domain.CloudOperationRun",
    DestroyCloudInstance: "domain.CloudOperationRun",
    StartCloudInstance: "domain.CloudOperationRun",
    StopCloudInstance: "domain.CloudOperationRun",
    ValidateCloudInstance: "domain.CloudOperationRun",
    ProvisionCloudInstance: "domain.CloudOperationRun",
    StartCloudProvisioning: "domain.CloudOperationRun",
    ValidateMachineConfiguration: "domain.MachineConfigurationValidation",
    ImportSSHKey: "sshkey.Key",
    ListSSHKeys: "[]sshkey.Key",
    ListPlanningAlgorithms: "[]ports.SchedulerDescriptor",
    ImportPlan: "domain.SchedulePlan",
  };
  if (endpoint.path === "/")
    return { mediaType: "text/plain", type: "plain text", example: "ok" };
  if (endpoint.path === "/akoflow-api/preflight/") {
    const check = { available: true, message: "service is available" };
    return {
      mediaType: "application/json",
      type: "preflight checks",
      example: { server: check, docker: check, buildkit: check },
    };
  }
  if (endpoint.handler === "StreamConsoleSession")
    return {
      mediaType: "application/websocket",
      type: "bidirectional terminal byte stream",
      example: null,
    };
  if (endpoint.handler === "ExportArchiveInstance")
    return { mediaType: "application/zip", type: "ZIP archive", example: null };
  const streamedContentType = /Content-Type",\s*"([^"]+)"/.exec(body)?.[1];
  if (streamedContentType && /(?:io\.Copy|w\.Write)\(/.test(body))
    return {
      mediaType: streamedContentType,
      type: streamedContentType.includes("text/")
        ? "text stream"
        : "binary stream",
      example: streamedContentType.includes("text/")
        ? "streamed content"
        : null,
    };
  if (/Content-Type",\s*"application\/octet-stream"/.test(body))
    return {
      mediaType: "application/octet-stream",
      type: "binary stream",
      example: null,
    };
  if (/Content-Type",\s*"text\/event-stream"/.test(body))
    return {
      mediaType: "text/event-stream",
      type: "event stream",
      example: "data: {...}",
    };
  if (endpoint.successStatuses.some((status) => status.startsWith("204")))
    return { mediaType: null, type: "empty response", example: null };
  const explicitMap = [
    ...body.matchAll(
      /writeJSON\(w,\s*http\.Status[A-Za-z]+,\s*map\[string\](?:any|string)\s*\{([^}]*)\}/g,
    ),
  ].at(-1);
  if (explicitMap) {
    const keys = [...explicitMap[1].matchAll(/"([^"]+)"\s*:/g)].map(
      (match) => match[1],
    );
    return {
      mediaType: "application/json",
      type: "JSON object",
      example: Object.fromEntries(keys.map((key) => [key, "string"])),
    };
  }

  const responseVariable =
    /(?:writeItem|writeList|writeJSON)\(w,\s*(?:http\.Status[A-Za-z]+,\s*)?([A-Za-z0-9_]+)/g;
  const variable = [...body.matchAll(responseVariable)].at(-1)?.[1];
  if (variable) {
    const assignedMap = new RegExp(
      `${variable}\\s*:=\\s*map\\[string\\]any\\s*\\{([\\s\\S]*?)\\n\\s*\\}`,
    ).exec(body);
    if (assignedMap) {
      const keys = [...assignedMap[1].matchAll(/"([^"]+)"\s*:/g)].map(
        (match) => match[1],
      );
      return {
        mediaType: "application/json",
        type: "JSON object",
        example: Object.fromEntries(
          keys.map((key) => [
            key,
            /(?:s|activities|handles|events)$/i.test(key) ? [] : {},
          ]),
        ),
      };
    }
  }
  const declaredType = variable
    ? new RegExp(`var\\s+${variable}\\s+([^\\s]+)`).exec(body)?.[1]
    : null;
  const producer = variable
    ? new RegExp(
        `${variable}\\s*(?:,[^:=\\n]+)?\\s*:=\\s*(?:[A-Za-z0-9_]+\\.)*([A-Z][A-Za-z0-9_]*)\\(`,
      ).exec(body)?.[1]
    : null;
  const ownedProducer = variable
    ? new RegExp(
        `${variable}\\s*(?:,[^:=\\n]+)?\\s*:=\\s*h\\.([a-z][A-Za-z0-9_]*)\\.([A-Z][A-Za-z0-9_]*)\\(`,
      ).exec(body)
    : null;
  const owner = ownedProducer ? handlerFieldTypes.get(ownedProducer[1]) : null;
  const ownerType = owner
    ? ownerReturnTypeIndex.get(`${owner}.${ownedProducer[2]}`)
    : null;
  const candidates = producer ? [...(returnTypeIndex.get(producer) || [])] : [];
  const inferredType =
    responseTypeOverrides[endpoint.handler] ||
    declaredType ||
    ownerType ||
    (candidates.length === 1 ? candidates[0] : null);
  return {
    mediaType: "application/json",
    type: inferredType || "JSON object",
    example: inferredType ? sampleValue(inferredType, structIndex) : {},
  };
}

function endpointDescription(endpoint) {
  return `AkôFlow ${endpoint.group} API: ${endpoint.title}.`;
}

function endpointDocument(endpoint, position) {
  const relativePath = endpointPath(endpoint.path);
  const title = endpoint.title;
  const params = extractParameters(endpoint.path);
  const body = Boolean(endpoint.request);
  const runnableFile =
    runnableSimulationRequests[`${endpoint.method} ${endpoint.path}`];
  const runnableSection = runnableFile
    ? endpoint.path === "/akoflow-api/workflow-definitions/import/"
      ? `## Runnable SimGrid request\n\nUse [\`examples/simulation/workflow.yaml\`](https://github.com/UFFeScience/akoflow/blob/v1.0.8/examples/simulation/workflow.yaml) **instead of** the create-workflow request in the [first-run tutorial](/docs/guides/workflows/first-run). Keep the other five requests in order so their referenced IDs exist. Do not submit the same workflow through both routes.\n\n`
      : `## Runnable SimGrid request\n\nThe [first-run tutorial](/docs/guides/workflows/first-run) submits [\`examples/simulation/${runnableFile}\`](https://github.com/UFFeScience/akoflow/blob/v1.0.8/examples/simulation/${runnableFile}) in a six-request sequence. Follow that order so referenced IDs exist.\n\n`
    : "";
  const verifiedNote = verifiedRequestNotes[`${endpoint.method} ${endpoint.path}`];
  const verifiedSection = verifiedNote
    ? `## Handler-checked request notes\n\n${verifiedNote}\n\n`
    : "";
  const requestExample =
    runnableFile || requestWithoutStandaloneExample.has(`${endpoint.method} ${endpoint.path}`)
      ? null
      : verifiedRequestExamples[`${endpoint.method} ${endpoint.path}`] ??
        endpoint.request?.example;
  return `---
title: ${JSON.stringify(title)}
sidebar_label: ${JSON.stringify(`${endpoint.method} ${relativePath}`)}
sidebar_position: ${position}
hide_title: true
hide_table_of_contents: true
custom_edit_url: null
description: ${JSON.stringify(endpoint.description)}
---

import ApiEndpoint from '@site/src/components/ApiEndpoint';

<ApiEndpoint
  method=${JSON.stringify(endpoint.method)}
  path=${JSON.stringify(endpoint.path)}
  handler=${JSON.stringify(endpoint.handler)}
  group=${JSON.stringify(endpoint.group)}
  pathParams={${JSON.stringify(params)}}
  queryParams={${JSON.stringify(endpoint.queryParameters)}}
  successStatuses={${JSON.stringify(endpoint.successStatuses)}}
  requestExample={${JSON.stringify(requestExample == null ? null : JSON.stringify(requestExample, null, 2))}}
  requestExampleVerified={${Boolean(verifiedRequestExamples[`${endpoint.method} ${endpoint.path}`])}}
  requestMediaType={${JSON.stringify(endpoint.request?.mediaType || (body ? "application/json" : null))}}
  requestFileName={${JSON.stringify(endpoint.request?.fileName || null)}}
  requestMultipartField={${JSON.stringify(endpoint.request?.multipartField || null)}}
  responseExample={${JSON.stringify(endpoint.response.example === null ? null : typeof endpoint.response.example === "string" ? endpoint.response.example : JSON.stringify(endpoint.response.example, null, 2))}}
  responseType=${JSON.stringify(endpoint.response.type)}
  responseMediaType={${JSON.stringify(endpoint.response.mediaType)}}
  hasRequestBody={${body}}
/>

${runnableSection}${verifiedSection}## Related guide

See the [${endpoint.group} guide](${groupMetadata[endpoint.group][0]}) for related tasks and context.
`;
}

const source = await readFile(routerFile, "utf8");
const handlerSource = await readFile(handlerFile, "utf8");
const goSources = await collectGoSources(sourceRoot);
const structIndex = buildStructIndex(goSources);
const returnTypeIndex = buildReturnTypeIndex(goSources);
const ownerReturnTypeIndex = buildOwnerReturnTypeIndex(goSources);
const handlerFieldTypes = extractHandlerFieldTypes(handlerSource);
const routePattern =
  /mux\.HandleFunc\("(GET|POST|PUT|PATCH|DELETE) ([^" ]+)",\s*(?:http_config\.KernelHandler\()?(?:workflowEngine\.)?([A-Za-z0-9_]+)/g;
const endpoints = [];
for (const match of source.matchAll(routePattern)) {
  const [, method, routePath, handler] = match;
  const group = groupFor(routePath);
  const handlerBody = extractFunctionBody(
    `${handlerSource}\n${source}`,
    handler,
  );
  const title =
    routeTitles[`${method} ${routePath}`] || humanizeHandler(handler);
  const endpoint = { method, path: routePath, handler, group, title };
  const baseEndpoint = {
    ...endpoint,
    description: endpointDescription(endpoint),
    queryParameters: extractQueryParameters(handlerBody),
    successStatuses:
      delegatedSuccessStatuses[handler] ?? extractSuccessStatuses(handlerBody, handler),
    guide: groupMetadata[group][0],
  };
  endpoints.push({
    ...baseEndpoint,
    request: handler === "SaveBuildContext"
      ? { type: "uploaded build context", example: null, mediaType: "multipart/form-data", fileName: "context.tar.gz", multipartField: "context" }
      : handler === "ImportArchiveInstance"
      ? { type: "ZIP instance archive (maximum 8 GiB)", example: null, mediaType: "application/zip", fileName: "instance.zip" }
      : extractRequestContract(handlerBody, structIndex),
    response: extractResponseContract(
      baseEndpoint,
      handlerBody,
      structIndex,
      returnTypeIndex,
      ownerReturnTypeIndex,
      handlerFieldTypes,
    ),
  });
}

if (endpoints.length === 0) {
  throw new Error(`No API routes found in ${routerFile}`);
}

const undocumentedMutations = endpoints
  .filter((endpoint) => endpoint.method !== "GET")
  .map((endpoint) => `${endpoint.method} ${endpoint.path}`)
  .filter((route) => !verifiedRequestNotes[route] && !runnableSimulationRequests[route]);
if (undocumentedMutations.length > 0) {
  throw new Error(
    `Mutating API routes need handler-checked notes or a runnable request: ${undocumentedMutations.join(", ")}`,
  );
}

endpoints.sort(
  (left, right) =>
    left.group.localeCompare(right.group) ||
    left.path.localeCompare(right.path) ||
    methodOrder[left.method] - methodOrder[right.method],
);

await rm(outputDirectory, { recursive: true, force: true });
await mkdir(outputDirectory, { recursive: true });
await mkdir(manifestDirectory, { recursive: true });

const groupPositions = new Map();
for (const [index, group] of [
  ...new Set(endpoints.map((endpoint) => endpoint.group)),
].entries()) {
  groupPositions.set(group, index + 1);
  const groupDirectory = path.join(
    outputDirectory,
    group.toLowerCase().replaceAll(" ", "-"),
  );
  await mkdir(groupDirectory, { recursive: true });
  await writeFile(
    path.join(groupDirectory, "_category_.json"),
    `${JSON.stringify({ label: group, position: index + 1, collapsed: true }, null, 2)}\n`,
  );
}

const positions = new Map();
for (const endpoint of endpoints) {
  const currentPosition = (positions.get(endpoint.group) || 0) + 1;
  positions.set(endpoint.group, currentPosition);
  const groupDirectory = path.join(
    outputDirectory,
    endpoint.group.toLowerCase().replaceAll(" ", "-"),
  );
  await writeFile(
    path.join(groupDirectory, `${slugFor(endpoint.method, endpoint.path)}.mdx`),
    endpointDocument(endpoint, currentPosition),
  );
}

await writeFile(
  manifestFile,
  `${JSON.stringify({ source: path.relative(docsDirectory, routerFile), generatedAt: new Date().toISOString(), endpoints }, null, 2)}\n`,
);

console.log(
  `Generated ${endpoints.length} API endpoint pages from ${path.relative(docsDirectory, routerFile)}`,
);
