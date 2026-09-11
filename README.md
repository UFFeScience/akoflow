```
 █████╗ ██╗  ██╗ ██████╗ ███████╗██╗      ██████╗ ██╗    ██╗
██╔══██╗██║ ██╔╝██╔═══██╗██╔════╝██║     ██╔═══██╗██║    ██║
███████║█████╔╝ ██║   ██║█████╗  ██║     ██║   ██║██║ █╗ ██║
██╔══██║██╔═██╗ ██║   ██║██╔══╝  ██║     ██║   ██║██║███╗██║
██║  ██║██║  ██╗╚██████╔╝██║     ███████╗╚██████╔╝╚███╔███╔╝
╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝     ╚══════╝ ╚═════╝  ╚══╝╚══╝
```

# AkôFlow - Open Source Engine for Containerized Scientific Workflows

AkôFlow is an open-source engine for orchestrating and executing container-based scientific workflows in the computing continuum. It was originally developed within the e-Science Research Group at the Institute of Computing, Fluminense Federal University (UFF).

Although initially focused on Kubernetes-based workloads, AkôFlow has evolved to support general containerized execution across multiple infrastructures.

- To learn more about AkôFlow, please visit our project page: [https://akoflow.com/](https://akoflow.com)

- To see our documentation, please visit: [https://uffescience.github.io/akoflow/](https://uffescience.github.io/akoflow/)

## Getting Started

### Desktop application

The Desktop is the intended installation path: it starts the graphical client
and the version-matched daemon and BuildKit services through Docker Compose.

> **Current public-release limitation.** Do not use the latest public release
> as a new production installation yet. On 2026-09-11, release `v1.0.3`
> published Desktop assets named `1.0.0`, and anonymous manifest requests for
> both runtime images returned `401 Unauthorized`. A public release must have
> matching Desktop assets and anonymously pullable GHCR images before the
> steps below are reproducible. See the
> [installation guide](https://uffescience.github.io/akoflow/installation/) for
> verification commands and the manual-stack recovery path.

After that release condition is met, download the matching installer for macOS,
Windows, or Linux from [AkôFlow Releases](https://github.com/UFFeScience/akoflow/releases/latest).

- **macOS:** open the universal `.dmg`, drag **AkôFlow Desktop** to
  `Applications`, and launch it.
- **Windows:** run the x64 installer `.exe`, or use the portable `.exe` without
  installation.
- **Linux:** run the x64 `.AppImage` after `chmod +x`, or install the `.deb`
  package with `sudo apt install ./Akoflow-Desktop-*.deb`.

AkôFlow Desktop requires Docker Desktop on macOS and Windows, or Docker Engine
with the Compose v2 plugin on Linux. Docker Desktop for Windows must use Linux
containers. At startup, Desktop checks those requirements and starts the
version-matched daemon and BuildKit containers.

Docker selects an available API port bound only to `127.0.0.1`; no fixed host
port is reserved. BuildKit remains entirely inside the Compose network.

### Updates

Update behavior must be validated against the packaged public release before it
is relied upon operationally. Preserve the local Docker volumes and export the
instance before changing versions. Do not treat the current public release as a
recovery path until matching installers and public runtime images are available.

Release maintainers should configure `MACOS_CSC_LINK`,
`MACOS_CSC_KEY_PASSWORD`, `MACOS_APPLE_ID`,
`MACOS_APP_SPECIFIC_PASSWORD`, `MACOS_TEAM_ID`, `WINDOWS_CSC_LINK` and
`WINDOWS_CSC_KEY_PASSWORD` as GitHub Actions secrets. These credentials sign
and notarize the installers; they are never included in the application.

### Container images

The release workflow is configured to publish versioned images for
`linux/amd64` and `linux/arm64` to:

- `ghcr.io/uffescience/akoflow-daemon`
- `ghcr.io/uffescience/akoflow-buildkit`

The production Compose bundle and configuration instructions are available in
[`releases/`](./releases/README.md).

## Releases

The release workflow runs when a semantic-version tag such as `v1.2.3` is
pushed. Its intended output is Desktop installers plus versioned daemon and
BuildKit images. Verify that those artifacts are available and version-aligned
before calling a release installable.

See all releases: [https://github.com/UFFeScience/akoflow/releases](https://github.com/UFFeScience/akoflow/releases)

## Contributors

- [D.Sc. Daniel de Oliveira — Research Advisor](http://profs.ic.uff.br/~danielcmo/)
- [Wesley Ferreira - @ovvesley — Maintainer - IC/UFF](https://github.com/ovvesley)
- Liliane Kunstmann - COPPE/UFRJ
- Debora Pina - COPPE/UFRJ
- Raphael Garcia — IC/UFF
- [Yuri Frota — IC/UFF](http://www.ic.uff.br/~yuri/)
- [Marcos Bedo — IC/UFF](https://www.professores.uff.br/marcosbedo/)
- [Aline Paes — IC/UFF](http://www.ic.uff.br/~alinepaes/)
- [Luan Teylo — INRIA/Université de Bordeaux](https://team.inria.fr/)

## Publications (in Portuguese)

- Ferreira, W., Kunstmann, L., Paes, A., Bedo, M., & de Oliveira, D. (2024, October). `AkôFlow`: um Middleware para execução de Workflows científicos em múltiplos ambientes conteinerizados. In 39th Simpósio Brasileiro de Banco de Dados (SBBD) (pp. 27-39). SBC. ([DOI:10.5753/sbbd.2024.241126.](https://doi.org/10.5753/sbbd.2024.241126.))

- Ferreira, W., Kunstmann, L., Garcia R., Bedo, M., & de Oliveira, D. (2025, October). _Plug and Flow_: Execução de Workflows Científicos em Contêineres com o Middleware `AkôFlow`. In 40th Simpósio Brasileiro de Banco de Dados (SBBD). (_Paper just accepted_)

## Academic Context

AkôFlow originated as a final undergraduate project and has since expanded with broader contributions and integrations. It continues to serve both academic and industrial workflow execution scenarios.
