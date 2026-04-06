---
id: downloads
title: Downloads & Releases
sidebar_label: Downloads
---

All AkôFlow components are released automatically when a version tag (`v0.x.y`) is pushed to the repository.  
Find every release at [github.com/UFFeScience/akoflow/releases](https://github.com/UFFeScience/akoflow/releases).

---

## Desktop App

The desktop app provides a graphical interface to manage and monitor workflows.

| Platform | Architecture | File |
|---|---|---|
| macOS | Apple Silicon (arm64) | `AkôFlow Desktop-*-arm64.dmg` |
| macOS | Intel (x64) | `AkôFlow Desktop-*.dmg` |
| Windows | x64 | `AkôFlow Desktop Setup *.exe` |
| Linux | x64 | `AkôFlow Desktop-*.AppImage` |

[**→ Download latest release**](https://github.com/UFFeScience/akoflow/releases/latest)

:::note macOS Gatekeeper
If macOS blocks the app on first launch, run:
```bash
xattr -cr /Applications/AkôFlow\ Desktop.app
```
:::