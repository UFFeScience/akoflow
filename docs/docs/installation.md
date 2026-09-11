---
id: installation
title: Install AkôFlow
sidebar_label: Installation
description: Install the AkôFlow Desktop application or run the versioned daemon stack.
---

The recommended path is the packaged **AkôFlow Desktop** application. It includes the graphical client and starts the version-matched daemon and BuildKit services through Docker Compose. Use the manual stack when you need to inspect Docker output or automate the API directly.

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
5. Wait for **Overview** to appear and for the connection indicator to report that the daemon is connected.

If startup does not reach Overview, open the failure details before retrying. Typical causes are Docker not running, an unavailable image pull, or a local port conflict. The Desktop does not need you to copy the daemon token into the application.

Docker chooses an available API port bound to `127.0.0.1`; a fixed host port is not required. BuildKit remains inside the private Compose network.

:::note Persistent data
Database records, credentials, artifacts, and simulation data live in the `akoflow-desktop-data` Docker volume. Updating or recreating containers preserves this volume. Data removal is reserved for the explicit factory-reset flow.
:::

## Run the daemon stack without the Desktop bootstrap

Use the release bundle when you want to operate the API stack yourself. This path is also the quickest way to distinguish an image-distribution problem from a Desktop problem.

1. Verify Docker and Compose are available:

```bash
docker info >/dev/null
docker compose version
```

2. Copy `releases/.env.example` to `releases/.env`.
3. Set `AKOFLOW_VERSION` to the release tag and replace `AKOFLOW_API_TOKEN` with a secret. For example:

```bash
AKOFLOW_TOKEN_VALUE="$(openssl rand -hex 32)"
sed -i.bak "s/^AKOFLOW_API_TOKEN=.*/AKOFLOW_API_TOKEN=$AKOFLOW_TOKEN_VALUE/" releases/.env
rm releases/.env.bak
```

Keep `releases/.env` private. It is intentionally ignored by Git.

4. Pull and start the pinned images:

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

curl --fail-with-body \
  -H "Authorization: Bearer ${AKOFLOW_TOKEN}" \
  "${AKOFLOW_URL}/preflight/" | jq
```

The check is successful when `server.available`, `docker.available`, and `buildkit.available` are all `true`. Then verify authentication and an empty/new catalog:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer ${AKOFLOW_TOKEN}" \
  "${AKOFLOW_URL}/environments/" | jq
```

### If image pull is denied

The Compose stack pulls `ghcr.io/uffescience/akoflow-daemon` and `ghcr.io/uffescience/akoflow-buildkit`. If `docker compose pull` reports `unauthorized`, the registry package is not publicly reachable to your Docker client. Do not continue with a partial stack.

For an organization account that has package access, authenticate with a GitHub token that has `read:packages`:

```bash
printf '%s' "$GHCR_TOKEN" | docker login ghcr.io --username "$GITHUB_USERNAME" --password-stdin
docker compose --env-file releases/.env -f releases/compose.yaml pull
```

For an open-source public installation, the package owner must instead make the package public. A GitHub release being public does not by itself make its GHCR container packages public.

Do not commit `releases/.env`, `GHCR_TOKEN`, or any daemon token. Do not paste them into screenshots.

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

After Desktop shows a connected daemon or the API preflight succeeds, complete the [first end-to-end run](./guides/workflows/first-run). That tutorial verifies a real lifecycle boundary: registered infrastructure, workflow, plan, execution, activity records, and data-transfer evidence.
