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

The Desktop is the intended end-user installation path. Download the installer
for macOS, Windows, or Linux from [AkôFlow Releases](https://github.com/UFFeScience/akoflow/releases/latest).

- **macOS:** open the universal `.dmg`, drag **AkôFlow Desktop** to
  `Applications`, and launch it.
- **Windows:** run the x64 installer `.exe`, or use the portable `.exe` without
  installation.
- **Linux:** run the x64 `.AppImage` after `chmod +x`, or install the `.deb`
  package with `sudo apt install ./Akoflow-Desktop-*.deb`.

Each release asset is associated with the semantic version in its GitHub release
and the corresponding Git tag. Desktop downloads the version-matched runtime
archives from that release and loads them into local Docker; the project does
not publish daemon or BuildKit packages to a container registry.

AkôFlow Desktop requires Docker Desktop on macOS and Windows, or Docker Engine
with the Compose v2 plugin on Linux. Docker Desktop for Windows must use Linux
containers.

### Updates

Update behavior must be validated against the Desktop release in use. Export an
instance before changing versions so that it can be restored if needed.

Release maintainers should configure `MACOS_CSC_LINK`,
`MACOS_CSC_KEY_PASSWORD`, `MACOS_APPLE_ID`,
`MACOS_APP_SPECIFIC_PASSWORD`, `MACOS_TEAM_ID`, `WINDOWS_CSC_LINK` and
`WINDOWS_CSC_KEY_PASSWORD` as GitHub Actions secrets. These credentials sign
and notarize the installers; they are never included in the application.

## Releases

The release workflow runs when a semantic-version tag such as `v1.2.3` is
pushed. Its output is a GitHub Release containing Desktop installers and the
runtime archives they load locally; the tag identifies the exact source
revision used for the release.

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
