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

Download the current installer for macOS, Windows or Linux from
[AkôFlow Releases](https://github.com/UFFeScience/akoflow/releases/latest).

AkôFlow Desktop requires Docker Desktop on macOS and Windows, or Docker Engine
with the Compose v2 plugin on Linux. At startup it verifies the requirements,
explains anything that is missing, and starts the version-matched AkôFlow daemon
and BuildKit containers automatically. Docker Desktop for Windows must use
Linux containers.

Docker selects an available API port bound only to `127.0.0.1`; no fixed host
port is reserved. BuildKit remains entirely inside the Compose network.

### Updates

The packaged desktop application checks the releases in this repository for a
new version. Updates are downloaded only after confirmation and installed when
the application restarts. The desktop application, daemon image and BuildKit
image use the same release version.

When the updated application starts, it pulls and health-checks the matching
containers. Persistent Docker volumes are retained. If the new runtime cannot
become healthy, AkôFlow restores the last healthy container version and reports
the failed update instead of deleting data.

Release maintainers should configure `MACOS_CSC_LINK`,
`MACOS_CSC_KEY_PASSWORD`, `MACOS_APPLE_ID`,
`MACOS_APP_SPECIFIC_PASSWORD`, `MACOS_TEAM_ID`, `WINDOWS_CSC_LINK` and
`WINDOWS_CSC_KEY_PASSWORD` as GitHub Actions secrets. These credentials sign
and notarize the installers; they are never included in the application.

### Container images

Versioned images are published to the GitHub Container Registry for
`linux/amd64` and `linux/arm64`:

- `ghcr.io/uffescience/akoflow-daemon`
- `ghcr.io/uffescience/akoflow-buildkit`

The production Compose bundle and configuration instructions are available in
[`releases/`](./releases/README.md).

## Releases

New versions are released automatically when a semantic-version tag such as
`v1.2.3` is pushed to this repository. Each release publishes the desktop
installers and versioned Docker images for the AkôFlow daemon and BuildKit.

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
