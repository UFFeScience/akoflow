---
id: downloads
title: Downloads & Releases
sidebar_label: Downloads
---

All AkôFlow components are released automatically when a version tag (`v0.x.y`) is pushed to the repository.  
Find every release at [github.com/UFFeScience/akoflow/releases](https://github.com/UFFeScience/akoflow/releases).

---

## Desktop App

The desktop app is the recommended graphical client. It starts the version-matched AkôFlow daemon and BuildKit stack through Docker and preserves application data in Docker volumes.

| Platform | Architecture | File |
|---|---|---|
| macOS | Apple Silicon (arm64) | `AkôFlow Desktop-*-arm64.dmg` |
| macOS | Intel (x64) | `AkôFlow Desktop-*.dmg` |
| Windows | x64 | `AkôFlow Desktop Setup *.exe` |
| Linux | x64 | `AkôFlow Desktop-*.AppImage` |

[**→ Download latest release**](https://github.com/UFFeScience/akoflow/releases/latest)

Before installing, review the platform requirements in [Installation](installation). The packaged application checks for updates, health-checks the matching runtime images, and returns to the last healthy version if startup fails.

:::note macOS Gatekeeper
If macOS blocks the app on first launch, run:
```bash
xattr -cr /Applications/AkôFlow\ Desktop.app
```
:::
