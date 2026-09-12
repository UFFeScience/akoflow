---
id: installation
title: Install AkôFlow
sidebar_label: Installation
description: Install the AkôFlow Desktop application or run the versioned daemon stack.
---

The packaged **AkôFlow Desktop** application is intended to be the primary installation path. It includes the graphical client and starts the version-matched daemon and BuildKit services through Docker Compose.

:::caution Public release status
The public release path is not currently verified end to end. On 2026-09-11, the latest public release was `v1.0.3`, while its Desktop artifacts were named `1.0.0` and the public daemon image manifest at `ghcr.io/uffescience/akoflow-daemon:v1.0.3` returned `401 Unauthorized`. Do not rely on the release assets for a new production installation until a matching Desktop artifact and anonymously pullable runtime images are published.
:::

## Requirements

| Platform | Requirement |
|---|---|
| macOS | Docker Desktop |
| Windows | Docker Desktop using Linux containers |
| Linux | Docker Engine and the Docker Compose v2 plugin |

The first startup checks these requirements and reports anything that is missing. The renderer does not receive the daemon token and does not execute Docker, shell, SSH, Kubernetes, or infrastructure commands.

## Verify public distribution before installing

Run this read-only preflight before downloading a Desktop installer or creating `releases/.env`. It checks two independent publication boundaries: the current GitHub release must contain a Desktop installer with the same semantic version as the tag, and the daemon and BuildKit image manifests must be anonymously readable from GHCR.

```bash
export AKOFLOW_RELEASE_TAG="v1.0.3" # replace with the tag you intend to install

curl --fail-with-body --silent --show-error \
  "https://api.github.com/repos/UFFeScience/akoflow/releases/tags/${AKOFLOW_RELEASE_TAG}" \
  | jq -r '.assets[].name' | sort

for image in akoflow-daemon akoflow-buildkit; do
  printf '%s: ' "$image"
  curl --silent --show-error --output /dev/null --write-out '%{http_code}\n' \
    -H 'Accept: application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json' \
    "https://ghcr.io/v2/uffescience/${image}/manifests/${AKOFLOW_RELEASE_TAG}"
done
```

Continue only when the listed Desktop installer filename contains the release version — for example, `1.0.4` for tag `v1.0.4` — and both image checks print `200`. A `401` or `403` means the image package is not publicly pullable. A missing or mismatched installer filename means the Desktop build was not packaged from the same release version. Neither condition can be repaired from a client machine; wait for a corrected release.

## Install the Desktop application after a verified release

Use these steps only after the release page contains a matching Desktop package and the required GHCR images can be pulled without private organization credentials.

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

Use the release bundle when you want to operate the API stack yourself. This path is also the quickest way to distinguish an image-distribution problem from a Desktop problem. It needs the same publicly accessible daemon and BuildKit images as Desktop.

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

Update and rollback behavior must be verified against the packaged release in use. Until the public release path above is repaired, do not use the updater as a recovery mechanism. Preserve the `akoflow-desktop-data` Docker volume and export the instance before changing containers or versions.

## Verify the installation

After Desktop shows a connected daemon or the API preflight succeeds, complete the [first end-to-end run](./guides/workflows/first-run). That tutorial verifies a real lifecycle boundary: registered infrastructure, workflow, plan, execution, activity records, and data-transfer evidence.
