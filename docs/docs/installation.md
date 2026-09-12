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

## Running a server on an instance

Desktop is the installation path for an individual workstation. If an operator
needs a control plane on a Linux instance, follow [Run the AkôFlow server on a
Linux instance](./guides/operations/server-instance). That separate how-to
uses the daemon and BuildKit runtime archives from a named Release and keeps
the server API off the public network by default.

## Updates and rollback

Update and rollback should use a named GitHub Release. Export the instance
before changing versions so it can be restored if needed.

## Verify the installation

After Desktop shows a connected daemon, complete the [first end-to-end run](./guides/workflows/first-run). That tutorial verifies a real lifecycle boundary: registered infrastructure, workflow, plan, execution, activity records, and data-transfer evidence.
