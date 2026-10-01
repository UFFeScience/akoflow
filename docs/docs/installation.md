---
id: installation
title: Install AkôFlow
sidebar_label: Installation
description: Install the AkôFlow Desktop application from a versioned GitHub Release.
---

Install **AkôFlow Desktop** to run AkôFlow on your workstation. The installer
and matching runtime files are available from GitHub Releases. Desktop loads
the runtime into local Docker on first launch.

## Requirements

| Platform | Requirement |
|---|---|
| macOS | Docker Desktop and a matching Desktop installer from GitHub Releases |
| Windows | Docker Desktop using Linux containers and a matching Desktop installer |
| Linux | Docker Engine, the Docker Compose v2 plugin, and a matching Desktop installer |

Docker must be running before you open AkôFlow. Choose an installer that
matches your operating system and architecture.

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

After Desktop shows a connected service, use the [interface tour](./guides/interface-tour).
The [SimGrid example](./guides/workflows/first-run) verifies a full workflow,
but currently requires the development stack, repository files, and API access.
