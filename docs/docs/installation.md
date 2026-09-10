---
id: installation
title: Install AkôFlow
sidebar_label: Installation
description: Install the AkôFlow Desktop application or run the versioned daemon stack.
---

The recommended way to use AkôFlow is the packaged **AkôFlow Desktop** application. It includes the graphical client and starts the version-matched daemon and BuildKit services through Docker Compose.

## Requirements

| Platform | Requirement |
|---|---|
| macOS | Docker Desktop |
| Windows | Docker Desktop using Linux containers |
| Linux | Docker Engine and the Docker Compose v2 plugin |

The first startup checks these requirements and reports anything that is missing. The renderer does not receive the daemon token and does not execute Docker, shell, SSH, Kubernetes, or infrastructure commands.

## Install the Desktop application

1. Open the [latest AkôFlow release](https://github.com/UFFeScience/akoflow/releases/latest).
2. Download the installer for your operating system and architecture.
3. Install and open AkôFlow Desktop.
4. Allow the application to start Docker when prompted.
5. Wait for the instance screen or Overview to appear.

<!-- screenshot: installation/desktop-first-start.png — first startup requirement and daemon health screen -->

Docker chooses an available API port bound to `127.0.0.1`; a fixed host port is not required. BuildKit remains inside the private Compose network.

:::note Persistent data
Database records, credentials, artifacts, and simulation data live in the `akoflow-desktop-data` Docker volume. Updating or recreating containers preserves this volume. Data removal is reserved for the explicit factory-reset flow.
:::

## Run the daemon stack without the Desktop bootstrap

Use the release bundle when you want to operate the API stack yourself.

1. Copy `releases/.env.example` to `releases/.env`.
2. Replace `AKOFLOW_API_TOKEN` with a random secret.
3. Pull and start the pinned images:

```bash
docker compose --env-file releases/.env -f releases/compose.yaml pull
docker compose --env-file releases/.env -f releases/compose.yaml up -d
docker compose --env-file releases/.env -f releases/compose.yaml ps
docker compose --env-file releases/.env -f releases/compose.yaml port daemon 8080
```

The last command prints the loopback port selected by Docker. Use it as the API origin:

```bash
export AKOFLOW_URL="http://127.0.0.1:<port>/akoflow-api"
export AKOFLOW_TOKEN="<the token from releases/.env>"

curl --fail --silent \
  -H "Authorization: Bearer ${AKOFLOW_TOKEN}" \
  "${AKOFLOW_URL}/environments/"
```

Do not commit `releases/.env` or paste its token into screenshots.

## Develop the graphical client

The Desktop source is maintained in the sibling `akoflow-admin` project. Start the daemon, then run:

```bash
npm install --legacy-peer-deps
npm run dev
```

Use `npm run desktop` to run the Electron development shell. The Vite proxy reads the daemon token on the development server; it is not bundled into React or stored by the production Desktop bootstrap.

## Updates and rollback

The packaged application checks releases from the main AkôFlow repository. After confirmation, it installs the update on restart, pulls matching daemon and BuildKit images, and runs a health check. If the new runtime does not become healthy, AkôFlow restores the last healthy container version without deleting persistent volumes.

## Verify the installation

Continue to the [interface tour](./guides/interface-tour), then complete the [first end-to-end run](./guides/workflows/first-run).
