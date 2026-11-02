---
title: Control AkôFlow through MCP
description: Connect MCP clients to the full AkôFlow daemon API using stdio or authenticated Streamable HTTP.
---

# Control AkôFlow through MCP

AkôFlow provides named MCP tools for every registered daemon API operation.
The current catalog contains 136 domain operations, plus upload, download and
operation preparation tools. The `admin` profile exposes all 142 tools.

MCP uses the same HTTP API and domain validation as Desktop. It can operate
while Desktop is closed, provided the daemon is running. SSH, Docker,
Kubernetes, BuildKit, connection checks and runtime commands remain in the
daemon.

## Connect from AkôFlow Desktop

Open **Instance settings → MCP & AI apps**. MCP starts disabled. Enable it,
then choose **Add app** and select Codex, Claude Desktop, Claude Code or another
MCP client. Give each app its own connection.

![Enable MCP in Desktop](/img/interface/mcp/01-desktop-mcp-disabled.png)

Choose the access level before creating the connection. **Read only** is the
default; **Operate workflows** permits ordinary workflow changes and execution;
**Full administration** exposes all 142 tools, including credentials, cloud
resources and deletion.

![Choose the app and its permissions](/img/interface/mcp/03-desktop-choose-permissions.png)

The setup dialog includes the complete guide inside the platform:

1. Keep AkôFlow Desktop open.
2. Select **Add to Codex**, **Add to Claude Desktop** or **Add to Claude Code**.
   Alternatively, copy the configuration preview into the AI app's MCP settings.
3. Restart the AI app or reconnect its MCP servers. Review and approve requested
   write operations in that app.
4. Select **Test MCP connection**. The daemon runs a real MCP handshake, discovers
   the permitted tools and reads its identity. This verifies the service and
   credential; the AI app must still load its configuration.

![Generated Codex configuration](/img/interface/mcp/04-desktop-codex-configuration.png)

The connector uses the Node runtime already bundled in Electron. Users do not
need a separate Node.js installation or Docker command to configure MCP.
Desktop itself still uses its normal Docker-based runtime. The generated entry
contains an executable, a stable helper path and a **private connection file
reference**. Neither the daemon token nor the per-app token appears in the
preview, clipboard, renderer or client configuration.

![Successful MCP connection test](/img/interface/mcp/06-desktop-mcp-verified.png)

### Client configuration locations

| Client                    | Location and activation                                                              |
| ------------------------- | ------------------------------------------------------------------------------------ |
| Codex Desktop / CLI       | `~/.codex/config.toml`; select **Add to Codex**, then restart/reconnect Codex        |
| Claude Desktop on macOS   | `~/Library/Application Support/Claude/claude_desktop_config.json`                    |
| Claude Desktop on Windows | `%APPDATA%\\Claude\\claude_desktop_config.json`                                      |
| Claude Code               | User-level `~/.claude.json`; select **Add to Claude Code**, then start a new session |
| Other MCP clients         | Copy the generated stdio JSON entry into their MCP server settings                   |

For Claude Desktop, open **Settings → Developer → Edit Config** to check the file
actually used by your installation. If its location differs from the standard
path, copy the generated entry there manually. Add it alongside existing
`mcpServers` entries. Claude Desktop installation is offered on macOS and Windows;
on Linux use Claude Code or another compatible stdio client.

![Claude Desktop configuration guide](/img/interface/mcp/08-desktop-claude-configuration.png)

Automatic installation preserves other servers and settings, and saves a
private backup when replacing an existing configuration. A broken configuration
is reported without exposing its contents. Codex entries have explicit ownership
markers; manually maintained JSON entries are preserved during cleanup.

Try **“List my AkôFlow workflows and available environments.”** With write or
admin access, try **“Create a workflow called MCP demo with one activity.”**
The normal workflow pages show records created through MCP.

![Workflow created by an MCP client](/img/interface/mcp/07-workflow-created-through-mcp.png)

### Pause, rotate and revoke

**Disable MCP** pauses every app without deleting its registration. Re-enable
it to restore access. **Rotate credential** in the setup dialog invalidates the
old credential and refreshes the private desktop reference. Reconnect the AI app
after rotation. **Revoke access** invalidates one app immediately and removes its
managed local configuration and connection file when possible.

![Paused connections](/img/interface/mcp/11-desktop-mcp-paused.png)

![Revoked connection](/img/interface/mcp/12-desktop-access-revoked.png)

Connections are installation settings and survive archive switches and daemon
restarts. A factory reset revokes all saved app connections and disables MCP
before the restarted daemon accepts requests. Domain writes still respect read-only archives. The desktop Compose
stack persists MCP state in a dedicated volume, separate from instance exports.
The daemon stores metadata and credential references in `desktop-settings.json`;
credentials and operation receipts use separate private files. Native Desktop
refreshes local endpoint references at startup when the daemon port changes.

### Troubleshooting

- **MCP management unavailable:** update both Desktop and its matching daemon
  runtime. An older daemon cannot provide these management endpoints.
- **Server absent in the AI app:** restart it, check its MCP settings and use
  **Open config** or **Copy configuration**. Check custom client configuration
  directories and your installation's actual file location.
- **Connection unavailable:** keep Desktop running and test again. Local stdio
  connects to the daemon on this computer; a cloud-hosted AI client cannot reach
  this loopback endpoint.
- **Insufficient access:** revoke and recreate a connection with the required
  profile. Clients cannot promote their own permissions.
- **Moved application:** open **Configure app** and add the configuration again
  so it uses the current executable path.
- **Manually edited configuration:** access revocation still takes effect;
  remove any retained entry manually in the AI app.

Desktop management endpoints require the operator credential and are deliberately
excluded from the MCP catalog. An MCP client cannot enable itself, mint another
credential or promote its own role. The dedicated credential endpoint is
accessible only to native main-process code through Desktop; renderer proxies
reject it.

The screenshot and protocol evidence is documented in
[the homeserver validation record](/img/interface/mcp/validation.md).
For advanced standalone deployments and remote HTTPS/OAuth clients, use the
following sections.

## Build and run

Build either executable with Go 1.25 and a C compiler (SQLite uses CGO):

```bash
go build -o ./bin/akoflow ./cmd/akoflow
go build -o ./bin/akoflow-server ./cmd/server
```

Both executables accept the same MCP subcommand:

```bash
./bin/akoflow mcp --api-url http://127.0.0.1:8080/akoflow-api --permissions admin
# Equivalent, including inside the production daemon image:
./bin/akoflow-server mcp --permissions admin
```

The default transport is stdio. The client starts this process; the process
connects to an already running daemon. It does not start another engine.
Only MCP protocol messages go to stdout; operational logs go to stderr.

If the daemon is already running in its production container, clients can use
the bundled executable without installing a local Go toolchain:

```toml
[mcp_servers.akoflow]
command = "docker"
args = ["exec", "-i", "YOUR_DAEMON_CONTAINER", "akoflow-server", "mcp", "--permissions", "admin"]
tool_timeout_sec = 150
default_tools_approval_mode = "writes"
```

Replace the container name with the running daemon container. Use `-i`
without a TTY. The subprocess inherits the container's API credential;
no daemon token needs to be copied into client configuration.

Set `AKOFLOW_API_TOKEN` in the process environment through your existing
credential management. The token is never a command-line argument.
`AKOFLOW_API_URL` can supply the API URL instead of `--api-url`.

## Permission profiles

| Profile          | Capabilities                                                                                                  |
| ---------------- | ------------------------------------------------------------------------------------------------------------- |
| `read` (default) | Read domain records, results and provenance, and download outputs                                             |
| `write`          | Read access plus workflow creation, planning, execution and other ordinary mutations                          |
| `admin`          | Every operation, including credentials, console access, deletion, cloud lifecycle and instance administration |

The server filters the advertised catalog and enforces its configured profile.
The daemon continues to enforce its own capabilities, validation and read-only
archive restrictions. Environment creation, replacement and connection updates
first validate the exact connection definitions through the daemon's dedicated
connection-test API. Failed checks prevent registration. Simulated definitions
without connections do not run connectivity checks. Embedded secrets are
rejected; connection definitions must use credential references. An unavailable optional backend capability returns its
normal API error, such as `503`; MCP does not simulate success.

MCP credentials grant access under the daemon credential configured for this
endpoint. They do not create tenant or resource isolation inside the daemon.
Use separate endpoints and daemon credentials for different trust boundaries.

## Connect Codex

For stdio, configure an absolute executable path in the client's
`config.toml`. Forward credentials from the environment rather than copying
their values into the configuration:

```toml
[mcp_servers.akoflow]
command = "/absolute/path/to/akoflow"
args = ["mcp", "--permissions", "admin"]
env_vars = ["AKOFLOW_API_URL", "AKOFLOW_API_TOKEN"]
tool_timeout_sec = 150
default_tools_approval_mode = "writes"
```

For an authenticated HTTP endpoint:

```toml
[mcp_servers.akoflow]
url = "http://127.0.0.1:8080/mcp"
bearer_token_env_var = "AKOFLOW_MCP_TOKEN"
tool_timeout_sec = 150
default_tools_approval_mode = "writes"
```

For an OAuth endpoint, use its HTTPS URL without a static bearer token and run
`codex mcp login akoflow`. Configure a pre-registered client with
`codex mcp add akoflow --url https://akoflow.example.com/mcp --oauth-client-id CLIENT_ID`
when required by your authorization provider.

See the [official Codex MCP configuration](https://developers.openai.com/codex/mcp)
for client registration, approval policies and available configuration fields.

## Connect Claude Code or another MCP client

Claude Code can start the same stdio executable:

```bash
claude mcp add --transport stdio akoflow -- /absolute/path/to/akoflow mcp --permissions admin
```

For an OAuth-enabled remote endpoint:

```bash
claude mcp add --transport http akoflow https://akoflow.example.com/mcp
```

Use the client's MCP connection controls to complete authentication. A generic
stdio client configuration uses the executable and arguments below:

```json
{
  "mcpServers": {
    "akoflow": {
      "command": "/absolute/path/to/akoflow",
      "args": ["mcp", "--permissions", "admin"]
    }
  }
}
```

Provide `AKOFLOW_API_TOKEN` through the client's supported secret/environment
mechanism. Configuration formats, remote-server support and OAuth registration
requirements vary by client. The automated compatibility tests exercise the
official MCP Go SDK over real stdio and HTTP connections; they do not certify
every client application.

## Enable HTTP on the daemon

Configure the daemon, then restart it:

```dotenv
AKOFLOW_MCP_ENABLED=true
AKOFLOW_MCP_PERMISSIONS=admin
AKOFLOW_MCP_STATE_DIRECTORY=/var/lib/akoflow/mcp
```

Provide a separate, high-entropy `AKOFLOW_MCP_TOKEN` through the server's secret
management. The endpoint is `/mcp` on the daemon's configured HTTP address.
The API token and MCP token serve different connections and can be different.

The development Compose file forwards these settings. MCP remains disabled
unless explicitly enabled. For another listener, run a dedicated adapter:

```bash
akoflow mcp --transport http --listen 127.0.0.1:8081 --permissions admin
```

Use HTTPS for remote access. Set `AKOFLOW_MCP_PUBLIC_URL` to the public `/mcp`
URL when a reverse proxy preserves its public Host header. The adapter allows
that configured host and loopback hosts while rejecting other Host values.
Browser origins are denied unless explicitly
listed in `AKOFLOW_MCP_ALLOWED_ORIGINS`; non-browser clients do not need an
Origin header. Redirects from the daemon are not followed.

## OAuth for remote clients

AkôFlow can act as an OAuth protected resource backed by an existing
authorization server. Configure these values in the daemon or adapter:

```dotenv
AKOFLOW_MCP_PUBLIC_URL=https://akoflow.example.com/mcp
AKOFLOW_MCP_OAUTH_ISSUER=https://identity.example.com/realms/akoflow
AKOFLOW_MCP_OAUTH_INTROSPECTION_URL=https://identity.example.com/realms/akoflow/protocol/openid-connect/token/introspect
AKOFLOW_MCP_OAUTH_CLIENT_ID=akoflow-introspection
```

Provide `AKOFLOW_MCP_OAUTH_CLIENT_SECRET` through secret management. When OAuth
is configured, it replaces static-token authentication for this endpoint.

The authorization provider must support authorization-code login with PKCE and
either pre-registered clients or the registration mechanism required by your
MCP client. Configure it to issue access tokens for the exact public MCP URL
with the endpoint's scope: `akoflow:read`, `akoflow:write` or `akoflow:admin`.

AkôFlow publishes protected resource metadata at
`/.well-known/oauth-protected-resource/mcp` and its root fallback. It verifies
tokens through authenticated RFC 7662 introspection and requires:

- `active: true` and a future `exp`;
- `iss` matching the configured issuer;
- `aud` containing the exact public MCP URL;
- a nonempty `sub` and the endpoint's advertised permission scope.

Discovery and `WWW-Authenticate` challenges use the configured public URL.
The daemon API token remains server-side; client OAuth tokens are never passed
through to the API. AkôFlow does not provide its own identity provider or client
registration service. HTTP OAuth URLs are accepted only on loopback for tests
and local development.

## Tool arguments and results

Domain tools have stable snake_case names derived from their HTTP handlers.
Each tool exposes its own path parameters and request-body schema. Optional
query parameters retain the API's string representation.

For example, call `get_execution` with:

```json
{
  "path": { "runId": "simulation-example-run-v1" }
}
```

Call `list_executions` with:

```json
{
  "query": { "page": "1", "pageSize": "20" }
}
```

Mutations require a unique `requestId`, for example:

```json
{
  "requestId": "create-workflow-2026-10-30-001",
  "body": {
    "name": "MCP example",
    "spec": {
      "namespace": "science",
      "image": "alpine:3.20",
      "activities": [
        {
          "name": "prepare",
          "command": { "entrypoint": "sh", "arguments": ["-c", "echo ready"] }
        }
      ]
    }
  }
}
```

Use that payload with `create_workflow`, then retrieve it with
`get_workflow` using the returned ID. The `_compat` tools expose existing
compatibility API routes; prefer the canonical tool when both are available.

Small results contain `status`, `data` and `truncated: false`. API errors set
MCP `isError` and preserve their HTTP status and sanitized error payload.
Asynchronous operations retain the API's operation/run IDs and queued status.
Poll the appropriate detail tool instead of submitting another operation.
Daemon requests have a two-minute timeout; after a mutation times out, inspect
its daemon state before taking further action.

For admin operations, call `prepare_operation` with `tool` and the exact
`arguments`, review the operation with the user, then pass the returned
`confirmation` digest alongside those unchanged arguments. Changing an
argument invalidates the digest. This binds intent to arguments; it is not
cryptographic proof of human approval. Configure the client's approval policy
to require user review of mutations.

## Durable retry behavior

Every domain mutation reserves its `requestId` in a private SQLite receipt
ledger before contacting the daemon. Completed calls return the saved result
when retried with identical arguments. A different payload under the same ID
is rejected. OAuth receipts and transfers are scoped to the authenticated
subject; shared-token and stdio connections use the local shared principal.

A concurrent or interrupted request with an unknown outcome is not dispatched
again. Inspect daemon records before deciding whether to perform a new
operation. This prevents duplicate dispatch; it does not make the daemon
transaction and the receipt transaction atomic.

Receipts survive adapter restarts and are retained until an operator removes
them. Keep `AKOFLOW_MCP_STATE_DIRECTORY` persistent and private. Changing or
deleting it loses retry protection.

## Files, logs and console sessions

Use `begin_upload`, `upload_chunk` and `seal_upload` to stage immutable input.
Chunks are base64-encoded, have explicit offsets, and are limited to 512 KiB
decoded. Identical chunk retransmissions are accepted.

Pass the resulting `uploadId` instead of `body`:

- `import_archive_instance` sends the staged ZIP to the dedicated archive API.
- `save_build_context` sends the staged tar archive as multipart field `context`.
- Other body-based operations accept staged JSON or YAML; select the appropriate
  `mediaType` for YAML or non-JSON input.

Binary responses and sanitized text/JSON larger than 1 MiB return a
`downloadId`. Retrieve the complete bytes through `read_transfer`, then use
`release_transfer` to free storage. There is no silent truncation of these
responses. Each transfer is limited to 8 GiB, with a 16 GiB staging quota,
32 active references and a one-hour lifetime. Text sanitization accepts lines
up to 4 MiB; deeper-than-100-level JSON is rejected. References do not survive
a process restart. Adapter shutdown removes its temporary staging directory;
after a crash, operators can remove abandoned `transfers-*` directories once
the associated process is no longer running.

For terminal access, create a session through `open_console_session`, then
use `stream_console_session` with its `sessionId`, optional `input` and
`waitMillis` (1–5000). The adapter exchanges bounded input/output over the
daemon's WebSocket. Output can include prior session history; timeout does
not mean a terminal command has finished. Use `export_console_session_log`
for persisted logs and `close_console_session` to close the session.

Raw secret values belong only in dedicated credential operations, preferably
through a trusted client upload. Environment definitions contain credential
references. JSON secret fields and known credentials are redacted from MCP
responses, receipts and text logs. Arbitrary binary artifacts remain data
downloads; inspect their contents according to the same access policy as the
daemon's storage API.

## Maintaining coverage

The catalog is generated from the daemon's Go route registrations and request
types:

```bash
go generate ./internal/mcpbridge
git diff --exit-code -- internal/mcpbridge/catalog.json
go test ./...
go test -race ./internal/mcpbridge ./internal/api/httpserver ./cmd/server
```

A Go test regenerates the catalog in a temporary source tree and checks for
drift, so the existing Go CI job covers this automatically. Tests invoke every
registered domain tool
with the official MCP client, exercise real stdio/HTTP transports, and compare
workflow operations with the actual daemon API backed by a temporary database.

See the [API overview](/docs/reference/api-overview) for endpoint semantics and
the [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) for supported
protocol versions.
