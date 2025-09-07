```
 █████╗ ██╗  ██╗ ██████╗ ███████╗██╗      ██████╗ ██╗    ██╗
██╔══██╗██║ ██╔╝██╔═══██╗██╔════╝██║     ██╔═══██╗██║    ██║
███████║█████╔╝ ██║   ██║█████╗  ██║     ██║   ██║██║ █╗ ██║
██╔══██║██╔═██╗ ██║   ██║██╔══╝  ██║     ██║   ██║██║███╗██║
██║  ██║██║  ██╗╚██████╔╝██║     ███████╗╚██████╔╝╚███╔███╔╝
╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝     ╚══════╝ ╚═════╝  ╚══╝╚══╝
```

# AkôFlow - Open Source Middleware for Containerized Scientific Workflows

AkôFlow is an open-source middleware for orchestrating and executing container-based scientific workflows across heterogeneous environments. It was originally developed within the e-Science Research Group at the Institute of Computing, Fluminense Federal University (UFF).

Although initially focused on Kubernetes-based workloads, AkôFlow has evolved to support general containerized execution across multiple infrastructures.

## Getting Started

Full documentation is available at:
📘 [Wiki](https://github.com/UFFeScience/akoflow/wiki)

⸻
## Software Requirements


- **Operating System:** Linux, macOS or WSL2 (Windows Subsystem for Linux)
- **Docker:** [Install Docker](https://docs.docker.com/get-docker/)
- **kubectl:** [Install kubectl](https://kubernetes.io/docs/tasks/tools/)
- **Kubernetes Cluster:** One of the following:
  - [Kind](https://kind.sigs.k8s.io/) (local)
  - Docker Desktop Kubernetes (enable Kubernetes in settings)
  - Cloud providers (e.g., EKS, GKE, AKS)


## Instalation

Run the following command to install AkôFlow:
```bash
curl -fsSL https://akoflow.com/run | bash
```

AkôFlow will be available at `http://localhost:8080`.


## How to Set Up Kubernetes Runtime for AkôFlow

1. **Access the Web Interface**  
   Open your browser and go to:

2. **Connect to a Kubernetes Cluster**  
AkôFlow requires a Kubernetes runtime. You can use:
- [Kind](https://kind.sigs.k8s.io/)
- Docker Desktop
- Any Cloud Provider (e.g., EKS, GKE, AKS)

3. **Apply AkôFlow Resources**  
Run the following command:

```bash
kubectl apply -f https://raw.githubusercontent.com/UFFeScience/akoflow/main/pkg/server/resource/akoflow-dev-dockerdesktop.yaml
```
4. **Generate a Service Account Token**

```bash
kubectl create token akoflow-server-sa --duration=800h --namespace=akoflow
```

5. **Set Environment Variables**

```bash
K8S_API_SERVER_HOST=https://<your-k8s-api-endpoint>
K8S_API_SERVER_TOKEN=<your-generated-token>
```

## Demonstration video

[AkôFlow Demonstration _(In Portuguese)_](https://www.youtube.com/watch?v=RmrAMWkJij4)

## Maintainers
* [D.Sc. Daniel de Oliveira — Research Advisor](http://profs.ic.uff.br/~danielcmo/)
* [Wesley Ferreira - @ovvesley — Maintainer](https://github.com/ovvesley)
* Raphael Garcia — Maintainer

⸻
## Supported Environments

* Kubernetes (public cloud providers: AWS, GCP, Azure, etc.)
* Singularity (for local or HPC isolated execution)
* SDumont supercomputer (LNCC - Brazil)

⸻

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

## Publications

* Ferreira, W., Kunstmann, L., Paes, A., Bedo, M., & de Oliveira, D. (2024, October). `AkôFlow`: um Middleware para execução de Workflows científicos em múltiplos ambientes conteinerizados. In 39th Simpósio Brasileiro de Banco de Dados (SBBD) (pp. 27-39). SBC. ([DOI:10.5753/sbbd.2024.241126.]( https://doi.org/10.5753/sbbd.2024.241126. ))


* Ferreira, W., Kunstmann,  L., Garcia R., Bedo, M., & de Oliveira, D. (2025, October). _Plug and Flow_: Execução de Workflows Científicos em Contêineres com o Middleware `AkôFlow`. In 40th Simpósio Brasileiro de Banco de Dados (SBBD). (_Paper just accepted_)

## Academic Context

AkôFlow originated as a final undergraduate project and has since expanded with broader contributions and integrations. It continues to serve both academic and industrial workflow execution scenarios.

⸻


