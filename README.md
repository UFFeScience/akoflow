<div align="center">
  <img src="docs/static/img/brand/akoflow-macos.png" alt="AkôFlow" width="120" />

  # AkôFlow

  **Plan, execute, observe scientific workflows.**

  <a href="https://github.com/UFFeScience/akoflow/releases/latest"><img src="https://img.shields.io/github/v/release/UFFeScience/akoflow?display_name=tag&label=Desktop&color=111111" alt="Latest Desktop release" /></a>
  <a href="https://akoflow.com/docs/"><img src="https://img.shields.io/badge/docs-Read%20the%20guide-111111" alt="AkôFlow documentation" /></a>
  <a href="https://github.com/UFFeScience/akoflow/actions/workflows/docs-checks.yaml"><img src="https://github.com/UFFeScience/akoflow/actions/workflows/docs-checks.yaml/badge.svg?branch=main" alt="Documentation checks" /></a>
</div>

<br />

<div align="center">
  <a href="https://github.com/UFFeScience/akoflow/releases/latest"><img src="https://img.shields.io/badge/Download-Ak%C3%B4Flow%20Desktop-111111?style=for-the-badge" alt="Download AkôFlow Desktop" /></a>
  &nbsp;
  <a href="https://akoflow.com/docs/getting-started"><img src="https://img.shields.io/badge/Start-Documentation-f3f4f6?style=for-the-badge&logoColor=111111" alt="Open the getting started guide" /></a>
</div>

<br />

AkôFlow is an open-source control plane for containerized scientific workflows.
Build a workflow, model or connect its environment, compare execution plans, and
retain the evidence from each run.

<p align="center">
  <img src="docs/static/img/architecture/lifecycle-overview.svg" alt="AkôFlow lifecycle from infrastructure to observed evidence" width="960" />
</p>

| I want to… | Go to |
| --- | --- |
| Install AkôFlow Desktop | [Download the latest release](https://github.com/UFFeScience/akoflow/releases/latest) |
| Run an end-to-end SimGrid example | [First simulated workflow](https://akoflow.com/docs/guides/workflows/first-run) |
| Use ready-to-run workflow examples | [Workflow Showcase](https://akoflow.com/docs/showcase/) |
| Configure SimGrid, Kubernetes, SLURM, storage, or cloud capacity | [Infrastructure guides](https://akoflow.com/docs/guides/infrastructure/) |
| Work with the Engine programmatically | [API reference](https://akoflow.com/docs/api/) |

## Desktop requirements

Docker Desktop on macOS and Windows, or Docker Engine with the Compose v2 plugin
on Linux. The release installer loads the corresponding runtime archives locally;
no container-registry package is required.

## Releases

A semantic tag such as `v1.2.3` creates a GitHub Release with Desktop installers,
runtime archives, and SHA-256 checksums. See [all releases](https://github.com/UFFeScience/akoflow/releases).

## Academic context

AkôFlow is developed within the e-Science Research Group, Institute of Computing,
Fluminense Federal University (UFF). The project is described in the 2024
[SBBD publication](https://doi.org/10.5753/sbbd.2024.241126).
