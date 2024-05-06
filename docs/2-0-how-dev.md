---
title: 2.0 - Ambiente de Desenvolvimento
layout: page
---

# Como Instalar em um Cluster Local (Docker Desktop) em um Ambiente de Desenvolvimento

Para instalar o akoflow em um cluster local, você irá precisar de um cluster Kubernetes. Você pode instalar um cluster Kubernetes localmente utilizando o Docker Desktop. O Docker Desktop é uma ferramenta que permite a execução de containers Docker em um ambiente de desenvolvimento local.

Iremos criar o namespace `akoflow` e alguns outros recursos necessários para a execução do akoflow no cluster local, porém não iremos instalar o servidor do akoflow no cluster local, pois o servidor do akoflow, o akoflow-server, ele será executado como um processo separado.

Utilizaremos essa estratégia para facilitar o desenvolvimento do akoflow, pois o servidor do akoflow é um processo separado e não é necessário instalar o servidor do akoflow no cluster local, pois a cada alteração no código do servidor do akoflow, você precisará recriar a imagem Docker e recriar o deployment do servidor.

Isso tem um overhead desnecessário para o desenvolvimento, pois a cada alteração no código do servidor do akoflow, você precisará recriar a imagem Docker e recriar o deployment do servidor do akoflow.

Então, para facilitar o desenvolvimento, iremos executar o servidor do akoflow como um processo separado e iremos criar apenas os recursos necessários para a execução do akoflow no cluster local.

Para instalar os recursos necessários para a execução do akoflow no cluster local, você pode executar o seguinte comando:

```bash
kubectl apply -f https://raw.githubusercontent.com/ovvesley/akoflow/main/pkg/server/resource/akoflow-dev-dockerdesktop.yaml
```

Com isso você instalará o namespace `akoflow`, o service account `akoflow-service-account`, a role `akoflow-role`, a role binding `akoflow-role-binding` e o config map `akoflow-config` no seu cluster Kubernetes.

Agora os recursos necessários para a execução do akoflow no cluster local estão instalados no seu cluster Kubernetes.

Você precisará agora gerar o token de acesso para o servidor do akoflow. Essa será a forma do seu processo do akoflow separado se autenticar no cluster Kubernetes criado no Docker Desktop.

Para isso, você pode executar o seguinte comando:

```bash
kubectl create token akoflow-server-sa -n akoflow
```

Com isso você criará um token de acesso para o servidor do akoflow. Esse token de acesso será utilizado pelo servidor do akoflow para se autenticar no cluster Kubernetes.

Agora você pode executar o servidor do akoflow. Para isso, você pode executar o seguinte comando:

Você precisará configurar as variáveis de ambiente.

As variáveis de ambiente necessárias para a execução do servidor do akoflow são:

- `K8S_API_SERVER_TOKEN` - Token de acesso para o servidor do akoflow.
- `K8S_API_SERVER_HOST` - Host do servidor do Kubernetes com a porta. Por padrão, o host é `localhost:6443`, em ambiente com docker desktop.

Exemplo (Linux):

```bash
export K8S_API_SERVER_TOKEN=...
export K8S_API_SERVER_HOST=localhost:6443

```

Por fim, você pode executar o servidor do akoflow. Para isso, você pode executar o seguinte comando:
```bash
go run cmd/server/main.go
```

Com isso você executará o servidor do akoflow no seu ambiente de desenvolvimento.