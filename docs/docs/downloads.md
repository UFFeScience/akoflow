---
id: downloads
title: Download AkôFlow Desktop
sidebar_label: Downloads
description: Direct official downloads for AkôFlow Desktop, with platform selection and release verification.
---

## Choose your download

These links point to the published **v1.0.8** release, checked on **2026-09-12**.
Choose one package for your workstation. The browser saves it in your configured
download folder; open the completed download and follow [Installation](./installation).

| Platform                       | Download                                                                                                                          | What to do next                                                          |
| ------------------------------ | --------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| macOS, Intel and Apple silicon | [Download universal DMG](https://github.com/UFFeScience/akoflow/releases/download/v1.0.8/Akoflow-Desktop-1.0.8-mac-universal.dmg) | Open it and drag AkôFlow Desktop to Applications                         |
| Windows x64                    | [Download Windows EXE](https://github.com/UFFeScience/akoflow/releases/download/v1.0.8/Akoflow-Desktop-1.0.8-win-x64.exe)         | Open the executable and follow its prompts                               |
| Debian/Ubuntu x64              | [Download DEB](https://github.com/UFFeScience/akoflow/releases/download/v1.0.8/Akoflow-Desktop-1.0.8-linux-amd64.deb)             | Install using `sudo apt install ./Akoflow-Desktop-1.0.8-linux-amd64.deb` |
| Linux x64                      | [Download AppImage](https://github.com/UFFeScience/akoflow/releases/download/v1.0.8/Akoflow-Desktop-1.0.8-linux-x86_64.AppImage)  | Give it execute permission and open it                                   |

[View this release and all assets](https://github.com/UFFeScience/akoflow/releases/tag/v1.0.8)
· [Check for a newer release](https://github.com/UFFeScience/akoflow/releases/latest)

The direct links above remain pinned to the documented version. If you choose a
newer release, use the filenames and requirements attached to that release.
Do not rename an older installer to match a newer tag.

## Which files can I ignore?

For Desktop installation, choose the package in the table. `.blockmap` and
`latest*.yml` files are updater metadata. Source-code ZIP/TAR downloads are for
development. Daemon/BuildKit `.tar` archives and runtime `.sha256` manifests are
service assets, not separate Desktop installers. Operators use them in the
[self-managed server guide](./guides/operations/server-instance).

v1.0.8 has a single Windows `.exe`, not separate named installer/portable files.
It has no Linux ARM64 Desktop package.

## Download through the GitHub API

For automation, inspect the release metadata first. This is GitHub's public API,
not the AkôFlow daemon API. You need Bash, `curl`, and `jq`.

```bash
AKOFLOW_RELEASE_TAG=v1.0.8
AKOFLOW_ASSET=Akoflow-Desktop-1.0.8-linux-amd64.deb
curl --fail --location --silent --show-error \
  "https://api.github.com/repos/UFFeScience/akoflow/releases/tags/$AKOFLOW_RELEASE_TAG" \
  -o release.json

AKOFLOW_DOWNLOAD_URL=$(jq -er --arg name "$AKOFLOW_ASSET" \
  '.assets[] | select(.name == $name) | .browser_download_url' release.json) || exit 1
curl --fail --location --show-error \
  "$AKOFLOW_DOWNLOAD_URL" -o "$AKOFLOW_ASSET"
```

On Linux, verify the downloaded bytes against the release asset digest:

```bash
AKOFLOW_DIGEST=$(jq -er --arg name "$AKOFLOW_ASSET" \
  '.assets[] | select(.name == $name) | .digest | select(startswith("sha256:"))' \
  release.json) || exit 1
printf '%s  %s\n' "${AKOFLOW_DIGEST#sha256:}" "$AKOFLOW_ASSET" | sha256sum --check -
```

Expect `OK`. If metadata or download requests fail, stop before installation.
If the checksum differs, discard that download and retrieve it again.

## Download verification result

The Linux `.deb` was downloaded in full and verified on 2026-09-12:

```text
Package: akoflow-desktop
Version: 1.0.8
Architecture: amd64
SHA-256: db73f5efd75789ff82db7aa6338e10ffad80b6c178f8650730c58b3ebb023315
```

The macOS, Windows, AppImage and runtime checksum URLs were checked with
redirect-following range requests. Download availability is separate from
operating-system installation validation.

The extracted Linux application was also opened with a fresh profile and reached
the successful [daemon, Docker and BuildKit checkup](./installation#3-first-launch-what-happens).
