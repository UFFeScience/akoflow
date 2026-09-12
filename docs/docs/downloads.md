---
id: downloads
title: Downloads & Releases
sidebar_label: Downloads
---

Pushing a version tag (`v0.x.y`) triggers the repository release workflow. A version is available to users only after that workflow completes successfully; find published artifacts at [github.com/UFFeScience/akoflow/releases](https://github.com/UFFeScience/akoflow/releases).

---

## Desktop App

The Desktop application is the graphical client. A GitHub Release contains the
installers and runtime archives; its semantic tag identifies the corresponding
source revision.

| Platform | Architecture | Release asset family |
|---|---|---|
| macOS | universal build | `.dmg` and `.zip` |
| Windows | x64 | `.exe` |
| Linux | x64 | `.AppImage` and `.deb` |

[**→ Download latest release**](https://github.com/UFFeScience/akoflow/releases/latest)

:::caution Verify the release before installing
Use an installer whose filename contains the same semantic version as the
GitHub Release tag. Desktop loads the matching daemon and BuildKit archives
from that release into local Docker; no container-registry package is required.
:::

Before installing, review the platform requirements in [Installation](installation). After launch, confirm that Desktop reaches **Overview**.

Need a control plane on a Linux instance instead of a workstation installation?
Use the separate [self-managed server guide](./guides/operations/server-instance).
It is an operator procedure, not a Desktop download option.

:::note macOS Gatekeeper
After verifying that the bundle came from the official release, if macOS blocks it on first launch, run:
```bash
xattr -cr /Applications/AkôFlow\ Desktop.app
```
:::
