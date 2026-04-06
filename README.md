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

* To learn more about AkôFlow, please visit our project page: [https://akoflow.com/](https://akoflow.com)

* To see our documentation, please visit: [https://uffescience.github.io/akoflow/](https://uffescience.github.io/akoflow/)

## Getting Started

AkôFlow can be installed and run with a single command, making it easy to get started with containerized workflow execution.

```bash
curl -fsSL https://akoflow.com/run | bash 
```


## Docker images

All images are published to [Docker Hub](https://hub.docker.com/u/akoflow) for `linux/amd64` and `linux/arm64`.

## Releases

New versions are released automatically when a tag in the format `v0.x.y` is pushed to this repository.  
Each release includes binaries, desktop installers, and versioned Docker images.

See all releases: [https://github.com/UFFeScience/akoflow/releases](https://github.com/UFFeScience/akoflow/releases)

## Contributors
* [D.Sc. Daniel de Oliveira — Research Advisor](http://profs.ic.uff.br/~danielcmo/)  
* [Wesley Ferreira - @ovvesley — Maintainer - IC/UFF](https://github.com/ovvesley)  
* Liliane Kunstmann - COPPE/UFRJ
* Debora Pina - COPPE/UFRJ
* Raphael Garcia — IC/UFF
* [Yuri Frota — IC/UFF](http://www.ic.uff.br/~yuri/)  
* [Marcos Bedo — IC/UFF](https://www.professores.uff.br/marcosbedo/)  
* [Aline Paes — IC/UFF](http://www.ic.uff.br/~alinepaes/)  
* [Luan Teylo — INRIA/Université de Bordeaux](https://team.inria.fr/)  

## Publications (in Portuguese)

* Ferreira, W., Kunstmann, L., Paes, A., Bedo, M., & de Oliveira, D. (2024, October). `AkôFlow`: um Middleware para execução de Workflows científicos em múltiplos ambientes conteinerizados. In 39th Simpósio Brasileiro de Banco de Dados (SBBD) (pp. 27-39). SBC. ([DOI:10.5753/sbbd.2024.241126.]( https://doi.org/10.5753/sbbd.2024.241126. ))


* Ferreira, W., Kunstmann,  L., Garcia R., Bedo, M., & de Oliveira, D. (2025, October). _Plug and Flow_: Execução de Workflows Científicos em Contêineres com o Middleware `AkôFlow`. In 40th Simpósio Brasileiro de Banco de Dados (SBBD). (_Paper just accepted_)

## Academic Context

AkôFlow originated as a final undergraduate project and has since expanded with broader contributions and integrations. It continues to serve both academic and industrial workflow execution scenarios.
