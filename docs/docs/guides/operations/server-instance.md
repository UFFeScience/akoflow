---
id: server-instance
title: Run the AkôFlow server on a Linux instance
sidebar_label: Server on an instance
description: Install the AkôFlow server from a versioned Release on a trusted Linux instance.
---

# Run the AkôFlow server on a Linux instance

Use this how-to when an operator needs an AkôFlow server on a trusted
Linux instance, separate from the local server installed by AkôFlow
Desktop. It loads the server and BuildKit images directly from a versioned
GitHub Release. It does not use a container registry and it does not install
the Desktop application.

Use [Install AkôFlow](/docs/installation) for a personal workstation. Do not
use this procedure for an untrusted or multi-tenant host: the supplied Compose
configuration gives the server access to the host Docker socket and runs
BuildKit with Docker privileges.

## Before you begin

You need:

- a Linux `amd64` or `arm64` instance with Docker Engine and the Docker Compose
  v2 plugin installed;
- an account allowed to download the public GitHub Release assets;
- Bash, `curl`, `jq`, and `sha256sum` on the instance;
- shell access to the instance and enough disk space for the two image archives,
  their loaded images, BuildKit state, SQLite data, and workflow artifacts;
- a firewall or private network policy that keeps port 8080 reachable only from
  the operator or a reverse proxy you manage.

This guide keeps the API bound to `127.0.0.1` on the instance. Reach it through
an SSH tunnel or terminate TLS at a separately managed reverse proxy. Do not
change the port mapping to `0.0.0.0` merely to make it convenient: bearer-token
authentication protects operations, but the server has
access to Docker, workflow credentials, and execution targets.

The release must contain matching daemon and BuildKit archives for the instance
architecture. A missing archive means that release cannot be used for this
deployment.

## 1. Select the Release and architecture

On the instance, choose the exact release tag and map the kernel architecture
to the name used by the Release assets.

```bash
export AKOFLOW_RELEASE_TAG="v1.0.8" # version documented in Downloads

case "$(uname -m)" in
  x86_64) export AKOFLOW_ARCH="amd64" ;;
  aarch64|arm64) export AKOFLOW_ARCH="arm64" ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac
```

Confirm that the exact release lists these three files before continuing:

```text
akoflow-daemon-<tag>-linux-<arch>.tar
akoflow-buildkit-<tag>-linux-<arch>.tar
akoflow-runtime-<tag>-linux-<arch>.sha256
```

The [Downloads and Releases](/docs/downloads) page explains the relationship
between the Git tag and published artifacts. This procedure deliberately uses
the two runtime archives; it does not look for a package or a registry image.

## 2. Download and verify the runtime images

Create a directory owned by the service operator. The checksum manifest and
both image archives must stay together while `sha256sum` verifies them.

```bash
mkdir -p ~/akoflow-server/releases
cd ~/akoflow-server/releases

export AKOFLOW_RELEASE_URL="https://github.com/UFFeScience/akoflow/releases/download/${AKOFLOW_RELEASE_TAG}"

curl --fail-with-body --location --remote-name \
  "${AKOFLOW_RELEASE_URL}/akoflow-daemon-${AKOFLOW_RELEASE_TAG}-linux-${AKOFLOW_ARCH}.tar"
curl --fail-with-body --location --remote-name \
  "${AKOFLOW_RELEASE_URL}/akoflow-buildkit-${AKOFLOW_RELEASE_TAG}-linux-${AKOFLOW_ARCH}.tar"
curl --fail-with-body --location --remote-name \
  "${AKOFLOW_RELEASE_URL}/akoflow-runtime-${AKOFLOW_RELEASE_TAG}-linux-${AKOFLOW_ARCH}.sha256"

sha256sum --check "akoflow-runtime-${AKOFLOW_RELEASE_TAG}-linux-${AKOFLOW_ARCH}.sha256"
```

Every checked file must report `OK`. Stop if a checksum fails; remove the
downloaded files and obtain them again from the same release.

Load the verified images into the instance's local Docker image store:

```bash
docker image load --input "akoflow-daemon-${AKOFLOW_RELEASE_TAG}-linux-${AKOFLOW_ARCH}.tar"
docker image load --input "akoflow-buildkit-${AKOFLOW_RELEASE_TAG}-linux-${AKOFLOW_ARCH}.tar"

docker image inspect \
  "akoflow/daemon:${AKOFLOW_RELEASE_TAG}" \
  "akoflow/buildkit:${AKOFLOW_RELEASE_TAG}" \
  --format '{{.RepoTags}}'
```

The last command must print both versioned image tags. The Compose stack uses
only those local tags, so it cannot silently pull a newer image.

## 3. Configure the local server

Download the versioned Compose file supplied with this documentation:

```bash
cd ~/akoflow-server
curl --fail-with-body --location --remote-name \
  "https://akoflow.com/examples/server-instance/compose.yaml"
```

Create a private `.env` file. Create a long random bearer token in the
operator's password manager, then enter it below; it is required by every
operational API request. The shell commands below avoid putting the token in
the shell history.

```bash
umask 077
read -r -s -p "AkôFlow API token: " AKOFLOW_API_TOKEN
printf '\n'
printf 'AKOFLOW_RELEASE_TAG=%s\nAKOFLOW_API_TOKEN=%s\nAKOFLOW_PORT=8080\n' \
  "$AKOFLOW_RELEASE_TAG" "$AKOFLOW_API_TOKEN" > .env
unset AKOFLOW_API_TOKEN
```

The Compose file persists SQLite, managed credentials, simulation workspaces,
artifacts, and BuildKit state in named Docker volumes. It disables the
interactive console. It also mounts `/var/run/docker.sock`; retain that mount
only on a trusted host where the server is allowed to create local
containers.

If a trusted browser client must call this server directly, set
`AKOFLOW_API_ALLOWED_ORIGINS` to the exact comma-separated HTTPS origins and
add that variable to the server service. The server only accepts origins that
match the list exactly; it has no wildcard origin mode. Prefer an SSH tunnel or
a reverse proxy when this is not necessary.

## 4. Start and verify

Start the two services using the local images:

```bash
docker compose -f compose.yaml up -d
docker compose -f compose.yaml ps
docker compose -f compose.yaml logs --tail=100 akoflow-server
```

The server's public preflight endpoint reports the server and local dependency
state without exposing operational data:

```bash
set -o pipefail
curl --fail-with-body --silent http://127.0.0.1:8080/akoflow-api/preflight/ | jq .
```

Check that `server.available` is `true`. Then prove that the bearer token is
accepted for an operational request:

```bash
set -o pipefail
read -r -s -p "AkôFlow API token: " AKOFLOW_API_TOKEN
printf '\n'
curl --fail-with-body --silent \
  -H "Authorization: Bearer ${AKOFLOW_API_TOKEN}" \
  http://127.0.0.1:8080/akoflow-api/environments/ | jq .
unset AKOFLOW_API_TOKEN
```

An empty JSON list is a valid new-instance result. A `401 Unauthorized` means
the token in the request differs from `.env`; correct the file and restart the
server with `docker compose -f compose.yaml up -d`.

For an operator working from another machine, use a tunnel rather than exposing
the API port:

```bash
read -r -p "SSH user: " AKOFLOW_SSH_USER
read -r -p "Instance hostname: " AKOFLOW_SERVER_HOST
ssh -N -L 8080:127.0.0.1:8080 "${AKOFLOW_SSH_USER}@${AKOFLOW_SERVER_HOST}"
```

Run the same `curl` commands against your local `127.0.0.1:8080` while the
tunnel is open. Continue with the [API overview](/docs/reference/api-overview)
or register infrastructure and execute the [first simulated workflow](/docs/guides/workflows/first-run).

## Operate, update, and remove

To update, download and verify the archives for the newer release, then load
its versioned images. Change only `AKOFLOW_RELEASE_TAG` in `.env` and run
`docker compose -f compose.yaml up -d`.
The named volumes remain attached, so plans, runs, artifacts, and managed
credentials are retained. Export the instance before changing versions if you
need an additional recovery point; see [Instance management](/docs/guides/operations/instance-management).

To stop the services while retaining their state:

```bash
docker compose -f compose.yaml down
```

Do not add `--volumes` unless you intend to irreversibly remove the AkôFlow
database, artifacts, managed credentials, and BuildKit cache. To inspect the
local volumes before any destructive action, run `docker volume ls | grep
akoflow`.

## Troubleshooting

| Symptom                                              | Check and recovery                                                                                                                                       |
| ---------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `manifest unknown` or Compose tries to pull an image | Verify `AKOFLOW_RELEASE_TAG` in `.env` and repeat `docker image load`; `docker image inspect akoflow/daemon:<tag>` must succeed before starting Compose. |
| Preflight reports BuildKit unavailable               | Run `docker compose -f compose.yaml logs buildkitd`; the supplied service needs a Docker host that permits privileged containers.                        |
| `401 Unauthorized` from an API route                 | Re-enter the token from `.env`. The preflight route is public, but environments, workflows, plans, runs, and credentials require the bearer token.       |
| State disappeared after a restart                    | Use `docker compose ... down`, not `down --volumes`. Inspect the `akoflow-state` volume before recreating or removing it.                                |
| A request needs browser CORS access                  | Configure only the exact trusted origin in `AKOFLOW_API_ALLOWED_ORIGINS`; do not use a wildcard or expose the API port directly.                         |

For server logs, Docker/BuildKit diagnostics, and network checks, see
[Troubleshooting](/docs/guides/operations/troubleshooting).
