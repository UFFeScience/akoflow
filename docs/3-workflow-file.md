---
title: 3 - Arquivo de Workflow
layout: page
---

O arquivo de workflow é um arquivo YAML que define um workflow. O arquivo de workflow é composto por uma lista de tarefas, onde cada tarefa é definida por um nome, uma imagem Docker e um comando a ser executado. O arquivo de workflow também pode definir dependências entre tarefas, para que uma tarefa só seja executada após a conclusão de outra tarefa.


A especificação abaixo detalha a configuração de um workflow denominado `wf-hello-world-gcp`, projetado para ser executado em um ambiente Kubernetes.


## Estrutura Geral

```yaml
name: wf-hello-world-gcp
spec:
  image: "alpine:3.7"
  namespace: "akoflow"
  storageClassName: "hostpath"
  storageSize: "32Mi"
  storageAccessModes: "ReadWriteOnce"
  mountPath: "/data"
  activities:
    - name: "a"
      ...
```

### Campos

- **name**: Nome do workflow. Serve como identificador único.
  - Tipo: String
  - Exemplo: `wf-hello-world-gcp`
  - Campo Obrigatório
- **spec**: Especificações do workflow, incluindo configurações de ambiente e definições de atividades.
  - Tipo: Object

Dentro de `spec`, temos:

- **image**: Imagem do container que será usado pelas atividades.
  - Tipo: String
  - Exemplo: `alpine:3.7`
- **namespace**: Namespace do Kubernetes onde o workflow será executado.
  - Tipo: String
  - Exemplo: `akoflow`
- **storageClassName**: Classe de armazenamento a ser utilizada para o volume persistente.
  - Tipo: String
  - Exemplo: `hostpath`
- **storageSize**: Tamanho do volume de armazenamento.
  - Tipo: String
  - Exemplo: `32Mi`
- **storageAccessModes**: Modo de acesso ao volume de armazenamento.
  - Tipo: String
  - Exemplo: `ReadWriteOnce`
- **mountPath**: Caminho onde o volume será montado dentro do container.
  - Tipo: String
  - Exemplo: `/data`



### Atividades

Cada atividade dentro de `activities` representa uma tarefa a ser executada dentro do workflow. Elas podem ter dependências entre si, definindo a ordem de execução.

- **name**: Nome da atividade. Identifica a tarefa dentro do workflow.
  - Tipo: String
  - Exemplo: `a`
- **memoryLimit**: Limite de memória para o container da atividade.
  - Tipo: String
  - Exemplo: `500Mi`
- **cpuLimit**: Limite de CPU para o container da atividade.
  - Tipo: String
  - Exemplo: `0.5`
- **nodeSelector**: Seletor de nó para especificar em qual nó do Kubernetes a atividade deve ser executada.
  - Tipo: String
  - Exemplo: `kubernetes.io/hostname=docker-desktop`
- **run**: Comando(s) que serão executados pela atividade.
  - Tipo: String
  - Exemplo:
    ```yaml
    run: |
      echo "Hello World" >> /data/a/output.txt
    ```
- **dependsOn**: Lista de atividades das quais esta atividade depende. A atividade só será executada após a conclusão das atividades listadas.
  - Tipo: Array de Strings
  - Exemplo:
    ```yaml
    dependsOn:
      - "a"
      - "b"
    ```

## Exemplo de Atividade

```yaml
- name: "a"
  memoryLimit: 500Mi
  cpuLimit: 0.5
  nodeSelector: "kubernetes.io/hostname=docker-desktop"
  run: |
    echo "Hello World" >> /data/a/output.txt
    sleep 5
    echo "Hello World Again" >> /data/a/output.txt
```

Este exemplo define uma atividade chamada `a`, que executa um script que escreve "Hello World" em um arquivo de saída, espera 5 segundos e escreve "Hello World Again" no mesmo arquivo. O script é executado em um container com a imagem `alpine:3.7`, limitado a 500Mi de memória e 0.5 de CPU, e é agendado para executar no nó `docker-desktop`.

| Parâmetro                         | Descrição                                                                 | Opcional/Obrigatório | Valor Padrão             | Exemplo                                    |
| --------------------------------- | ------------------------------------------------------------------------- | -------------------- | ------------------------ | ------------------------------------------ |
| `name`                            | Identificador único do workflow.                                          | Obrigatório          | N/A                      | `wf-hello-world-gcp`                       |
| `spec`                            | Contém as especificações do workflow.                                     | Obrigatório          | N/A                      |                                            |
| → `image`                         | Imagem do Docker a ser usada pelas atividades.                            | Obrigatório          | N/A                      | `alpine:3.7`                               |
| → `namespace`                     | Namespace do Kubernetes onde o workflow será executado.                   | Obrigatório          | `scik`                   | `akoflow`                               |
| → `storageClassName`              | Nome da classe de armazenamento para o volume persistente.                | Opcional             | Dependente do cluster    | `hostpath`                                 |
| → `storageSize`                   | Tamanho do volume de armazenamento.                                       | Obrigatório          | N/A                      | `32Mi`                                     |
| → `storageAccessModes`            | Modo de acesso ao volume.                                                 | Obrigatório          | N/A                      | `ReadWriteOnce`                            |
| → `mountPath`                     | Caminho no container onde o volume será montado.                          | Obrigatório          | N/A                      | `/data`                                    |
| → `activities`                    | Lista das atividades que compõem o workflow.                              | Obrigatório          | N/A                      |                                            |
| → → `name` (dentro de activities) | Nome da atividade, serve como identificador único dentro do workflow.     | Obrigatório          | N/A                      | `a`, `b`, `c`...                           |
| → → `memoryLimit`                 | Limite de memória para o container da atividade.                          | Opcional             | Sem limite               | `500Mi`                                    |
| → → `cpuLimit`                    | Limite de CPU para o container da atividade.                              | Opcional             | Sem limite               | `0.5`                                      |
| → → `nodeSelector`                | Seletor de nó para especificar em qual nó a atividade deve ser executada. | Opcional             | Todos os nós disponíveis | `kubernetes.io/hostname=docker-desktop`    |
| → → `run`                         | Comandos que serão executados pela atividade.                             | Obrigatório          | N/A                      | `echo "Hello World" >> /data/a/output.txt` |
| → → `dependsOn`                   | Lista de atividades das quais esta depende para ser executada.            | Opcional             | Nenhuma dependência      | `["a", "b"]`                               |


Aqui abaixo você irá encontrar os principais modos de criar tarefas e tudo mais. SUper gostaria que voce aprendesse isso. /
