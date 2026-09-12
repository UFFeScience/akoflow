# Installation and infrastructure tutorial verification

Date: 2026-09-12. Documentation branch: `docs/installation-hpc-cloud`, based on
backend main `d9bf02d`. Screenshots come from the official v1.0.8 Linux Desktop
package, not a mockup. Documentation language remains English to match the site.

## Download and first-launch evidence

- Full official asset download using `gh release download v1.0.8 --repo
UFFeScience/akoflow --pattern 'Akoflow-Desktop-1.0.8-linux-amd64.deb'`.
- SHA-256 `db73f5efd75789ff82db7aa6338e10ffad80b6c178f8650730c58b3ebb023315`,
  matching the release metadata.
- `dpkg-deb -f` reported package `akoflow-desktop`, version `1.0.8`, `amd64`.
- The documentation download link was clicked in headless Chrome; the completed
  DEB download again matched the release SHA-256.
- Official DEB, AppImage, DMG and Windows EXE URLs, plus both runtime checksum
  manifests, returned HTTP 206 to redirect-following `Range: bytes=0-0` requests.
  Only the DEB was fully downloaded for the installer comparison.
- Extracted DEB launched on Linux in a virtual display with a fresh temporary
  application profile and Docker access. This was not an `apt` install test.
- Desktop itself downloaded and loaded its release-matched runtime images.
  Docker reported `akoflow/daemon:v1.0.8` and `akoflow/buildkit:v1.0.8`; both
  containers became healthy.
- The application's actual `/akoflow-api/preflight/` response reported
  `server.available`, `docker.available`, and `buildkit.available` all `true`.
- The app reached its welcome screen, successful Engine checkup, and empty
  Overview. Chromium DevTools automation captured these and the real HPC/GCP
  forms. No HPC connection or cloud validation result was fabricated.

## Media

All files below are under `static/img/interface/onboarding/`:

| File                      | Evidence                                                      |
| ------------------------- | ------------------------------------------------------------- |
| `welcome.png`             | Initial screen of the downloaded package                      |
| `engine-checkup.png`      | Actual daemon, Docker and BuildKit readiness                  |
| `installation-result.png` | Overview with connected local control plane                   |
| `hpc-connection.png`      | Actual HPC form with example host/user/partition              |
| `gcp-connection.png`      | Actual GCP form with placeholder project and empty credential |

Form screenshots use a 1440 × 1200 viewport so the validation and save buttons
remain visible. Screenshots contain no private key, bearer token or cloud secret.
The sidebar's container identity belongs to the disposable local runtime.

## API and documentation verification

- HPC and GCP template envelopes posted to a separate local test daemon:
  each returned 201 on create and 200 on read, then 204 on cleanup.
- These tests validate the registration envelope and persistence only. They
  deliberately do not claim SSH connectivity, cloud IAM, discovery or scheduling.
- API instructions checked against the registered handlers, credential manager
  request shapes and frontend connection form. Connection testing precedes
  registration in both tutorial paths.
- Browser checks cover the five new/rewritten routes, image loading, and mobile
  widths; final build/link results are recorded with the change handoff.

## Still requiring external validation

- Actual package-manager installation on a clean Linux desktop; macOS and
  Windows installation and launch.
- An institutional account with gateway/host-key configuration and a real SLURM
  allocation, filesystem access and scheduler accounting.
- A disposable GCP project to verify IAM, catalog access and subsequent
  provisioning/cleanup. No provider resources were created for this work.

The release smoke test uses the application's normal Compose project name in an
otherwise empty Docker environment. Its two containers are stopped after capture;
volumes and downloaded archives are retained rather than deleting evidence.

Final checks passed: TypeScript typecheck, production build (125 generated API
pages), repository link check (325 local links/assets and 53 showcase downloads),
and `git diff --check`. Headless Chrome loaded all five routes and their images
without page errors. Installation, downloads, HPC and cloud pages had no
horizontal document overflow at 390px. Browser clicks downloaded both templates
with their documented `.template.json` filenames and the Linux DEB with the
expected SHA-256. The link checker now also recognizes `/examples/` static assets.

## Complete installation walkthrough follow-up

- Completed the official v1.0.8 local assistant: Welcome, Engine checkup,
  Environment, Connection, Ready, then the environment catalog. Registration,
  online health and discovery used real daemon responses. The catalog showed
  one local resource and 1/1 connections online.
- Added `environment-selection.png`, `connection-checkup.png`,
  `environment-ready.png`, and `environment-catalog.png`. One second of CDP
  network latency made the otherwise transient Connection step visible; no
  response was mocked or replaced. The temporary local environment was removed
  and recreated only in this disposable instance to recapture the assistant.
- Captured the actual Settings → SSH service keys tab as `ssh-service-keys.png`.
- Posted all six checked-in simulation YAML files to the release runtime:
  five HTTP 201 responses and one HTTP 202. The durable result was completed,
  3/3 activities, two transfers, 120000000 bytes, computeSeconds 9 and
  transferSeconds 11.69275257731959. `first-simulation-result.png` records the
  resulting Desktop run detail.
- Docker prerequisites link to official platform and Linux post-install docs
  reviewed on 2026-09-12. Commands are checked before opening the app; Linux
  desktop-session group refresh is explained.
- `apt-get --simulate install` resolved the downloaded DEB on this Ubuntu host:
  six new packages, zero upgrades/removals. This is dependency resolution, not
  an OS installation test. macOS/Windows and real HPC/GCP access remain outside
  the executed validation scope above.
- Server and first-run guides now use v1.0.8. The first-run guide explains how
  to obtain the source and API settings and bounds polling to 60 attempts.

Follow-up checks passed: production build, TypeScript typecheck, 335 local
route/asset links, 53 Showcase downloads, and `git diff --check`. Browser checks
loaded installation, downloads, API access, HPC, GCP, first simulation and server
installation with all article images decoded. The six visual guides fit a
390px viewport without document overflow. The three installation platform tabs
opened their corresponding instructions.
