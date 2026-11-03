---
title: Connect your AI apps with MCP
sidebar_label: MCP & AI apps
description: Enable AkôFlow MCP, connect Codex or Claude, choose permissions and verify your first AI app connection.
---

# Connect your AI apps with MCP

Use your AI app to inspect workflows, plan work and operate AkôFlow through the same daemon that powers Desktop. AkôFlow **1.0.12** exposes **150 tools** with Full administration, covering 144 domain operations and six protocol helpers.

Start with [the matching Desktop and runtime](/docs/downloads). Open Desktop and confirm the sidebar shows **Connected**. Keep Desktop open while its bundled MCP connector is in use.

## 1. Enable MCP

Open **Settings → MCP & AI apps**. Select **Enable MCP**. The service starts disabled on a fresh installation.

![AkôFlow 1.0.12 MCP settings before activation](/img/releases/v1.0.12/01-mcp-disabled.png)

Each AI app gets its own connection. You can pause all connections or revoke one app later.

## 2. Choose the app and its access

Select **Add app**, choose Codex, Claude Desktop, Claude Code or another MCP client, and enter a recognizable connection name.

| Access level                      | What the app can do                                                                     | Start here when…                                                          |
| --------------------------------- | --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| **Read only** (`read`)            | Inspect domain records, results and provenance; download outputs                        | You want explanations, status checks or help interpreting a run           |
| **Operate workflows** (`write`)   | Read data and perform ordinary workflow, planning and execution mutations               | You want the app to create definitions or operate workflows               |
| **Full administration** (`admin`) | Access every tool, including credentials, cloud operations, console access and deletion | The app must administer this installation and you will review its actions |

Read only is the default. Grant the access the task needs and use a separate connection for each app.

![Choose an AI app, connection name and permission profile](/img/releases/v1.0.12/02-mcp-permissions.png)

Full administration is selected in this demonstration so the complete catalog can be verified. Permissions are chosen when the connection is created; revoke and recreate a connection to change its profile.

## 3. Add the connection to your AI app

Select **Create connection** to open the setup guide. Expand **Configuration preview · credential references only** if you want to inspect the generated entry.

![Generated Codex configuration uses private connection references](/img/releases/v1.0.12/03-mcp-configuration.png)

- **Codex:** select **Add to Codex**, then restart or reconnect its MCP servers. Desktop and CLI share the generated `config.toml` entry.
- **Claude Code:** use its setup button, then start a new session or reconnect the server.
- **Claude Desktop on macOS or Windows:** use the setup button or copy the configuration into the client configuration file shown in the guide. Restart Claude Desktop.
- **Claude Desktop on Linux or another MCP app:** use **Copy configuration** and follow that client's MCP settings. The guide shows the supported installation method for your platform.

The connector and its Node runtime are bundled with Desktop. The preview contains executable and helper paths plus a private connection-file reference. Secret values remain in private storage. Paths in the screenshot belong to the isolated documentation installation; your generated entry uses your own application location.

Automatic setup preserves other configured servers and saves a backup when replacing a managed entry. Use **Open config** to check the actual file, or copy the entry manually into a custom configuration location.

## 4. Verify the connection

Select **Test MCP connection**. A real MCP client authenticates, completes the handshake, discovers the allowed tools and reads daemon identity.

![Successful AkôFlow 1.0.12 MCP handshake and 150-tool discovery](/img/releases/v1.0.12/04-mcp-verified.png)

After the test succeeds, reconnect your AI app to finish loading its configuration. The connection card shows **Verified** and the last successful connection time.

Try these prompts in your AI app:

> List my AkôFlow workflows and available environments.

> Summarize the most recent failed run and point me to its activity evidence.

With Operate workflows or Full administration, try:

> Create an AkôFlow workflow called “MCP demo” with one activity that prints hello. Show me the resulting definition before executing it.

Records created through MCP appear in the normal Desktop pages. Review requested writes in your AI app. Administrative operations bind their confirmation to the exact proposed arguments; cloud provisioning, credentials and deletion deserve the same review as their Desktop equivalents.

## Pause, rotate or remove access

| Control               | Result                                                                                                    |
| --------------------- | --------------------------------------------------------------------------------------------------------- |
| **Disable MCP**       | Pauses all saved connections. Re-enabling restores them.                                                  |
| **Rotate credential** | Invalidates the old credential and updates the private Desktop reference. Reconnect the AI app afterward. |
| **Revoke access**     | Invalidates one app immediately and removes the locally managed entry and connection file where possible. |

Connections survive daemon restarts and instance/archive switches. A factory reset revokes saved connections and disables MCP. Read-only archives still restrict domain writes.

## Troubleshooting

| What you see                                         | What to check                                                                                            |
| ---------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| The AI app does not show AkôFlow                     | Restart or reconnect the AI app, inspect its active MCP configuration and use the generated setup entry. |
| The test cannot connect                              | Keep Desktop open, confirm **Connected**, enable MCP and test again.                                     |
| A credential is rejected after rotation              | Reconnect the AI app so it reads the refreshed private reference.                                        |
| The app can read but cannot perform an operation     | Check the chosen profile. Recreate the connection with the required access.                              |
| Setup points to an old application path              | Open **Configure app** and add the connection again after moving or reinstalling Desktop.                |
| MCP management is unavailable                        | Update Desktop and its matching daemon runtime together.                                                 |
| A manually maintained entry remains after revocation | Remove that entry in the AI app. Revocation has already invalidated its credential.                      |

The Desktop connector reaches a daemon on the same computer. For a separately hosted daemon, authenticated HTTP, OAuth, protocol arguments and tool semantics, use the [MCP technical reference](/docs/reference/mcp).

## Validated in 1.0.12

These screenshots were captured from the published Linux Desktop package with its matching daemon and BuildKit runtime in an isolated installation. The native setup wrote a separate documentation-only Codex configuration, and the official MCP SDK verified all 150 administration tools. The documented installation also compiled the MOHEFT plugin through the daemon.

The [capture record](/img/releases/v1.0.12/capture.json) lists the published source revisions and the verified operations.

Explore [the project changelog](/docs/changelog) for every release and its screenshots, changes and downloads.
