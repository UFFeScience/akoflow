---
id: installation
title: Install AkôFlow
sidebar_label: Installation
description: Install the AkôFlow Desktop application from a versioned GitHub Release.
---

The packaged **AkôFlow Desktop** application is the end-user installation path.
GitHub Releases contain the Desktop installers and runtime archives, and a
semantic Git tag identifies the source revision used to create each release.
Desktop loads the matching daemon and BuildKit archives into local Docker; the
project does not publish them to a container registry.

## Requirements

| Platform | Requirement |
|---|---|
| macOS | Docker Desktop and a matching Desktop installer from GitHub Releases |
| Windows | Docker Desktop using Linux containers and a matching Desktop installer |
| Linux | Docker Engine, the Docker Compose v2 plugin, and a matching Desktop installer |

The renderer does not receive the daemon token and does not execute Docker,
shell, SSH, Kubernetes, or infrastructure commands.

## Verify a release before installing

Run this read-only preflight before downloading a Desktop installer. It lists
the assets attached to the exact GitHub Release tag.

```bash
export AKOFLOW_RELEASE_TAG="v1.0.3" # replace with the tag you intend to install

curl --fail-with-body --silent --show-error \
  "https://api.github.com/repos/UFFeScience/akoflow/releases/tags/${AKOFLOW_RELEASE_TAG}" \
  | jq -r '.assets[].name' | sort

```

Continue only when a listed Desktop installer filename contains the release
version — for example, `1.0.4` for tag `v1.0.4` — and the release also includes
`akoflow-daemon-${AKOFLOW_RELEASE_TAG}-linux-<arch>.tar`,
`akoflow-buildkit-${AKOFLOW_RELEASE_TAG}-linux-<arch>.tar`, and the matching
`.sha256` file for your architecture. Missing or mismatched assets mean the
release is incomplete; wait for a corrected release.

## Install the Desktop application

1. Open the [latest AkôFlow release](https://github.com/UFFeScience/akoflow/releases/latest).
2. Download the installer for your operating system and architecture.
3. Install and open AkôFlow Desktop.
4. Wait for **Overview** to appear and for the connection indicator to report that the daemon is connected.

If startup does not reach Overview, open the failure details before retrying.
The first launch downloads the matching runtime archives from the same release
and loads them into Docker. The Desktop does not need you to copy a daemon
token into the application.

## Run from a source tag

The GitHub Release assets are the supported end-user distribution. For local
development or a custom deployment, check out the desired Git tag and follow
the repository's development Compose instructions. This is a source-based
workflow, not a package installation, and it is maintained separately from the
Desktop release path.

## Develop the graphical client

The Desktop source is maintained in the sibling `akoflow-admin` project. Start the daemon, then run:

```bash
npm install --legacy-peer-deps
npm run dev
```

Use `npm run desktop` to run the Electron development shell. The Vite proxy reads the daemon token on the development server; it is not bundled into React or stored by the production Desktop bootstrap.

## Updates and rollback

Update and rollback should use a named GitHub Release. Export the instance
before changing versions so it can be restored if needed.

## Verify the installation

After Desktop shows a connected daemon or the API preflight succeeds, complete the [first end-to-end run](./guides/workflows/first-run). That tutorial verifies a real lifecycle boundary: registered infrastructure, workflow, plan, execution, activity records, and data-transfer evidence.
