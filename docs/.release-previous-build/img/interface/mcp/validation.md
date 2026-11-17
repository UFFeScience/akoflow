# Desktop MCP validation — homeserver

Captured on 2026-10-31 in an isolated homeserver worktree and QA container.
All records and credentials are disposable test data. Screenshots are direct,
unedited captures of the actual Electron renderer, not generated mockups.

## Environment

- Homeserver Linux; Debian 12 QA container, Xvfb display 1440 × 1000.
- Actual Electron 43.4.1 Linux x64, verified against official SHA-256 sums.
- Built React production bundle served by the real authenticated desktop server.
- Real AkôFlow daemon, isolated SQLite database and persistent MCP state.
- Native preload/IPC, configuration installer and bundled-runtime connector.
- Official MCP Go SDK v1.7.0 as an independent client.

No authenticated AI conversation inside Codex/Claude and no native macOS/Windows
UI execution is claimed. The Linux screenshots show Claude Desktop's honest
copy-only fallback; automatic installation paths for supported operating systems
are covered by configuration tests.

## Observed results

| Check                                   | Result                                                                                        |
| --------------------------------------- | --------------------------------------------------------------------------------------------- |
| Enable MCP in Desktop                   | Disabled → enabled, persisted by daemon                                                       |
| Add Codex with Full administration      | 142 tools advertised                                                                          |
| Install generated configuration         | Entry written to isolated ~/.codex/config.toml                                                |
| In-platform test                        | Credential accepted, handshake, discovery, daemon identity                                    |
| Installed stdio connector               | Official SDK starts the exact Electron executable/helper/private reference written by Desktop |
| Protocol interoperability               | SDK offers 2026-07-28; relay uses supported stateless HTTP protocol 2025-11-25                |
| Create workflow via installed connector | HTTP 201; durable receipt; readback HTTP 200                                                  |
| Workflow visible in Desktop             | Created through desktop MCP, namespace mcp-demo, one activity                                 |
| Claude Desktop Read only                | 73 tools, real connection test passed                                                         |
| Renderer credential access              | HTTP 403 for both test clients; no MCP/token localStorage entries; no token in DOM            |
| Rotate credential                       | Old token HTTP 401; new token HTTP 200                                                        |
| Disable / re-enable MCP                 | Authenticated request HTTP 503 / HTTP 200                                                     |
| Revoke Codex                            | Token HTTP 401; private connection file removed; owned Codex entry removed                    |

The full nonsecret create/readback result is in
[installed-connector-proof.json](installed-connector-proof.json).
Neither it nor the images contain a token.

## Screenshots

1. [Disabled service](01-desktop-mcp-disabled.png)
2. [Enabled service](02-desktop-mcp-enabled.png)
3. [Client and permission selection](03-desktop-choose-permissions.png)
4. [Codex configuration preview](04-desktop-codex-configuration.png)
5. [Codex configuration installed](05-desktop-codex-installed.png)
6. [Real MCP verification](06-desktop-mcp-verified.png)
7. [Workflow created through MCP](07-workflow-created-through-mcp.png)
8. [Claude Desktop configuration](08-desktop-claude-configuration.png)
9. [Separate app profiles and verification](09-desktop-app-connections.png)
10. [Credential rotation](10-desktop-credential-rotated.png)
11. [Paused connections](11-desktop-mcp-paused.png)
12. [Revoked access](12-desktop-access-revoked.png)

## Automated checks

Backend: go test ./... and targeted race tests cover the bridge and managed MCP
lifecycle, real HTTP and stdio clients, all registered domain tools, permissions,
authentication, rotation/revocation/restart, mutation receipts, archive rules,
connection preflight, secret redaction and transfers.

Desktop: npm run test:mcp (10 tests), npm run test:packaging-contract (12 tests, including the MCP suite),
npm run build, changed-file Prettier and git diff --check. Tests cover preserved
client configuration, private backups, dotfile symlinks, concurrent installation,
credential proxy isolation, runtime packaging, lossless relay and rotation.

The existing public documentation CI audit blocker remains separate: unchanged
main-branch dependencies have published advisories. See the PR for its current
CI links. A packaged production release still needs matching daemon images and
the corresponding pinned desktop runtime.
