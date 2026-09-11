---
id: downloads
title: Downloads & Releases
sidebar_label: Downloads
---

Pushing a version tag (`v0.x.y`) triggers the repository release workflow. A version is available to users only after that workflow completes successfully; find published artifacts at [github.com/UFFeScience/akoflow/releases](https://github.com/UFFeScience/akoflow/releases).

---

## Desktop App

The Desktop application is the graphical client. A usable package needs a matching daemon/BuildKit image pair that Docker can pull.

| Platform | Architecture | Release asset family |
|---|---|---|
| macOS | universal build | `.dmg` and `.zip` |
| Windows | x64 | `.exe` |
| Linux | x64 | `.AppImage` and `.deb` |

[**→ Download latest release**](https://github.com/UFFeScience/akoflow/releases/latest)

:::caution Verify the release before installing
The latest public release may contain packages that cannot yet start their matching runtime images. In particular, the public `v1.0.3` release was checked on 2026-09-11: its Desktop asset names still carried `1.0.0`, and anonymous access to `ghcr.io/uffescience/akoflow-daemon:v1.0.3` returned `401 Unauthorized`. Treat a release as installable only after its package version, image tag, and public pull access agree.
:::

Before installing, review the platform requirements in [Installation](installation). After launch, confirm Docker starts the matching runtime images and that Desktop reaches **Overview**.

:::note macOS Gatekeeper
After verifying that the bundle came from the official release, if macOS blocks it on first launch, run:
```bash
xattr -cr /Applications/AkôFlow\ Desktop.app
```
:::
