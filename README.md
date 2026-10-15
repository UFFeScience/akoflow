<div align="center">
  <img src="docs/static/img/brand/akoflow-readme-banner.png" alt="AkôFlow Desktop: scientific workflows across cloud, HPC, and on-premises environments" width="100%" />

  # AkôFlow

  **Run scientific workflows anywhere.**

  <a href="https://github.com/UFFeScience/akoflow/releases/latest"><img src="https://img.shields.io/github/v/release/UFFeScience/akoflow?display_name=tag&label=Desktop&color=111111" alt="Latest Desktop release" /></a>
  <a href="https://akoflow.com/docs/"><img src="https://img.shields.io/badge/docs-Read%20the%20guide-111111" alt="AkôFlow documentation" /></a>
  <a href="https://github.com/UFFeScience/akoflow/actions/workflows/docs-checks.yaml"><img src="https://github.com/UFFeScience/akoflow/actions/workflows/docs-checks.yaml/badge.svg?branch=main" alt="Documentation checks" /></a>
  <a href="https://codecov.io/gh/UFFeScience/akoflow"><img src="https://codecov.io/gh/UFFeScience/akoflow/branch/main/graph/badge.svg" alt="Backend test coverage" /></a>
</div>

<br />

<div align="center">
  <a href="https://github.com/UFFeScience/akoflow/releases/latest"><img src="https://img.shields.io/badge/Download-Ak%C3%B4Flow%20Desktop-111111?style=for-the-badge" alt="Download AkôFlow Desktop" /></a>
  &nbsp;
  <a href="https://akoflow.com/docs/getting-started"><img src="https://img.shields.io/badge/Start-Documentation-f3f4f6?style=for-the-badge&logoColor=111111" alt="Open the getting started guide" /></a>
</div>

<br />

AkôFlow helps you create workflows, choose where they run, execute them, and see
the result.

| To… | Open |
| --- | --- |
| Install Desktop | [Download AkôFlow](https://github.com/UFFeScience/akoflow/releases/latest) |
| Run a SimGrid example | [First workflow](https://akoflow.com/docs/guides/workflows/first-run) |
| Explore examples | [Workflow Showcase](https://akoflow.com/docs/showcase/) |
| Use the API | [API reference](https://akoflow.com/docs/reference/api-overview) |

## Desktop requirements

Use Docker Desktop on macOS and Windows, or Docker Engine with Compose v2 on
Linux. Runtime archives are included with each release; no container registry is required.

## Academic context

<a href="https://github.com/UFFeScience"><img src="https://github.com/UFFeScience.png?size=128" alt="UFFeScience" width="64" align="left" /></a>

AkôFlow is developed by the [UFFeScience](https://github.com/UFFeScience)
research group at the Institute of Computing, Fluminense Federal University
(UFF). The group develops open tools and research for data-intensive science,
workflows, and distributed computing.

<br clear="left" />

<a href="https://github.com/UFFeScience/akoflow/graphs/contributors"><img src="https://img.shields.io/github/contributors/UFFeScience/akoflow?label=contributors&color=111111" alt="AkôFlow contributors" /></a>
&nbsp;
<a href="https://github.com/UFFeScience/akoflow/graphs/contributors">Meet the contributors →</a>

<br /><br />

<a href="https://github.com/UFFeScience/akoflow/graphs/contributors"><img src="https://contrib.rocks/image?repo=UFFeScience/akoflow" alt="AkôFlow contributor avatars" /></a>

## Maintainer release packaging

Create the matching version tag in the Desktop repository before triggering the
backend release workflow. Application and runtime builds check out the frozen
source tag; packaging fixes can rebuild an existing draft without moving tags:

```sh
gh workflow run gh-akf-new-release.yaml \
  --repo UFFeScience/akoflow --ref main \
  --field release_tag=v1.0.10
```

The optional `runtime_artifact_run` input can reuse successful runtime archives
from a completed tagged-push run only when its repository, tag, source SHA and
architecture job all match. Downloaded archives are checked against their
SHA-256 manifests before upload. Desktop packages are always rebuilt and their
updater metadata verified; a failed publication does not justify reusing an
unverified installer.

Windows NSIS and portable executables use distinct `setup` and `portable`
filenames. Before upload, the Go `release-assets-check` command verifies updater
manifest versions, local filenames, sizes, SHA-512 values and legacy metadata
against the actual packages. The workflow creates a draft, not a public release.
Review successful checks, runtime checksums, package contents and screenshot
provenance before publishing the draft. Do not move an existing source tag to
hide a packaging correction.

## Cite AkôFlow

If AkôFlow contributes to your research, please cite:

```bibtex
@inproceedings{ferreira2024akoflow,
  author    = {Ferreira, Wesley and Kunstmann, Liliane and Paes, Aline and Bedo, Marcos and de Oliveira, Daniel},
  title     = {AkôFlow: um Middleware para execução de Workflows científicos em múltiplos ambientes conteinerizados},
  booktitle = {Proceedings of the 39th Brazilian Symposium on Databases (SBBD)},
  pages     = {27--39},
  year      = {2024},
  doi       = {10.5753/sbbd.2024.241126}
}
```

[Read the publication](https://doi.org/10.5753/sbbd.2024.241126).
