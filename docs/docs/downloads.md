---
id: downloads
title: Download AkôFlow Desktop
sidebar_label: Downloads
description: Direct official downloads for AkôFlow Desktop, with platform selection and release verification.
---

## Choose your download

Open the [latest official release](https://github.com/UFFeScience/akoflow/releases/latest)
and choose the asset for your workstation. Check the release notes for supported
platforms and prerequisites, then follow [Installation](/docs/installation).

| Platform                       | Asset to select | What to do next |
| ------------------------------ | --------------- | --------------- |
| macOS, Intel and Apple silicon | Universal `.dmg` | Open it and drag AkôFlow Desktop to Applications |
| Windows x64                    | Windows `.exe` | Follow the release notes for that asset |
| Debian/Ubuntu x64              | Linux `.deb` | Install the downloaded file with `sudo apt install ./downloaded-file.deb` (replace the filename) |
| Linux x64                      | Linux `.AppImage` | Give the downloaded file execute permission and open it |

[View the latest release and all assets](https://github.com/UFFeScience/akoflow/releases/latest).
Use the filenames and requirements attached to the release you choose; do not
mix assets from different tags.

## Which files can I ignore?

For Desktop installation, choose the package for your platform. `.blockmap` and
`latest*.yml` files are updater metadata. Source-code ZIP/TAR downloads are for
development. Daemon/BuildKit `.tar` archives and runtime `.sha256` manifests are
service assets, not separate Desktop installers. Operators use them in the
[self-managed server guide](/docs/guides/operations/server-instance).

Asset availability and installer format can change between releases. Read the
selected release's notes before downloading, especially for Windows and ARM64.

## Download through the GitHub API

For automation, inspect the release metadata first. This is GitHub's public API,
not the AkôFlow daemon API. You need Bash, `curl`, and `jq`.

```bash
AKOFLOW_RELEASE_API=https://api.github.com/repos/UFFeScience/akoflow/releases/latest
curl --fail --location --silent --show-error \
  "$AKOFLOW_RELEASE_API" \
  -o release.json

# Select the exact asset name shown on the release page.
AKOFLOW_ASSET='<desktop-asset-filename>'

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

## Verify your download

Use the digest published for the asset in the release metadata, as shown above.
Download availability and checksum verification are separate from operating-system
installation validation. After opening Desktop, confirm the
[daemon, Docker and BuildKit checkup](/docs/installation#3-first-launch-what-happens).
