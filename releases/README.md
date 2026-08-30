# AkôFlow container distribution

Tagged releases publish two public, multi-platform OCI images:

- `ghcr.io/uffescience/akoflow-daemon`
- `ghcr.io/uffescience/akoflow-buildkit`

The first publication of each GHCR package must be made public in the GitHub
organization package settings. Desktop installations intentionally do not
carry GitHub credentials and therefore cannot pull private packages.

Copy `.env.example` to `.env`, replace the API token with a random value, and
start the pinned stack:

```sh
docker compose --env-file releases/.env -f releases/compose.yaml pull
docker compose --env-file releases/.env -f releases/compose.yaml up -d
docker compose --env-file releases/.env -f releases/compose.yaml ps
docker compose --env-file releases/.env -f releases/compose.yaml port daemon 8080
```

Docker selects an available API port bound only to `127.0.0.1`; the final
command displays it. BuildKit is reachable only from the private Compose
network. Database, credentials, artifacts and simulation data live in the
`akoflow-desktop-data` volume; BuildKit cache lives in
`akoflow-desktop-buildkit`.

Recreating or upgrading containers preserves both volumes. Removing the
volumes is reserved for the explicit factory-reset flow.
