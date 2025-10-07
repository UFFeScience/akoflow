---
title: 4 - Sobre Atividades de Workflow
layout: page
---

Dentro de cada Workflow, as atividades são as tarefas que serão executadas. Cada atividade é definida por um nome, uma imagem Docker e um comando a ser executado. As atividades podem ser executadas em paralelo ou em sequência, dependendo das dependências definidas entre elas.

As atividades podem ser executadas em qualquer imagem Docker, desde que a imagem possua o shell padrão do sistema operacional. O comando a ser executado é definido como uma string, que será passada diretamente para o shell do sistema operacional(em geral, `/bin/sh`).
Cada atividade é executada em um container Docker separado, garantindo isolamento e segurança. As instruções de execução são passadas para o container através do entrypoint do container, com o comando codificado em base64.

Por exemplo, a atividade abaixo executa o comando `echo "Hello, World!"` em um container Docker baseado na imagem `alpine:3.7`:

### Exemplo de Atividade

```yaml
- name: "a"
  image: "alpine:3.7"
  run: "echo 'Hello, World!'"
```

Ela irá se transformar em um job Kubernetes com uma especificação semelhante a:

### Estrutura Geral do Job Kubernetes Montado

```json
{
    "apiVersion": "batch/v1",
    "kind": "Job",
    "metadata": {
        "name": "activity-95-a"
    },
    "spec": {
        "template": {
            "spec": {
                "containers": [
                    {
                        "name": "activity-057",
                        "image": "alpine:3.7",
                        "command": [
                            "/bin/sh",
                            "-c",
                            "echo <BASE_64_RUN_STRING>| base64 -d| sh"
                        ],
                        "volumeMounts": [
                            {
                                "name": "pvc-95-wfa",
                                "mountPath": "/data/a"
                            }
                        ],
                        "resources": {
                            "limits": {
                                "cpu": "0.5",
                                "memory": "500Mi"
                            }
                        }
                    }
                ],
                "restartPolicy": "Never",
                "backoffLimit": 0,
                "volumes": [
                    {
                        "name": "pvc-95-wfa",
                        "persistentVolumeClaim": {
                            "claimName": "pvc-95-wfa"
                        }
                    }
                ],
                "nodeSelector": {
                    "kubernetes.io/hostname": "docker-desktop"
                }
            }
        }
    }
}
```
O comando é executado em um container com a imagem `alpine:3.7`, limitado a 500Mi de memória e 0.5 de CPU, e é agendado para executar no nó `docker-desktop`, montando o volume `/data` no container.

### Atividades

Cada atividade dentro de `activities` representa uma tarefa a ser executada dentro do workflow. Elas podem ter dependências entre si, definindo a ordem de execução.

- **name**: Nome da atividade. Identifica a tarefa dentro do workflow.
  - Tipo: String
  - Exemplo: `a`

- **image**: Imagem Docker a ser utilizada para executar a atividade.
  - Tipo: String
  - Exemplo: `alpine:3.7`

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

Todos os comandos são executados em um /bin/sh, e o comando é codificado em base64 e passado para o container através do entrypoint.

### Atenção
Caso o container a ser executado não possua o shell padrão do sistema operacional, o comando não será executado corretamente. Por exemplo, a imagem `busybox` não possui o shell padrão o workflow não será executado corretamente.

Toda imagem DOCKER deve possuir o shell /bin/sh para que o comando seja executado corretamente. Imagens normalmente possuem o shell padrão do sistema operacional, como `alpine`, `ubuntu`, `debian`, `centos`, entre outras e as imagens baseadas nessas imagens também possuem o shell padrão, como `python`, `node`, `golang`, entre outras.

