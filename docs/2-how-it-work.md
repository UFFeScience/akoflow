---
title: 2 - Como Funciona
layout: page
---

# Como Funciona

O scik8sflow utiliza a API do Kubernetes para criar e gerenciar recursos de execução de workflows, como pods, jobs, persistent volumes. Ele permite a execução de workflows científicos de forma distribuída e paralela em um cluster Kubernetes.

## Servidor

O scik8sflow é composto por um servidor que gerencia a execução de workflows. O servidor é responsável por receber as requisições de execução de workflows, criar e gerenciar os recursos necessários para a execução dos workflows, como pods, jobs, persistent volumes. O servidor coleta métricas e logs dos recursos de execução de workflows e armazena em um banco de dados em arquivo (SQLite).

O servidor é composto por quatro componentes: HTTP Server, Monitor, Worker e Orquestrador. Cada componente está separado em uma goroutine e se comunicam através dos registros no banco de dados em arquivo (SQLite) e [channels](https://golang.org/doc/effective_go.html#channels).

Goroutines são [greenthreads](https://wikipedia.org/wiki/Green_threads) que são executadas em um único processo. Elas são leves e são gerenciadas pelo runtime do Go. Elas são mais eficientes que as threads do sistema operacional, pois são mais leves e consomem menos recursos. [Goroutines](https://golang.org/doc/effective_go.html#goroutines)

Channels são canais de comunicação entre goroutines. Eles são utilizados para enviar e receber mensagens entre goroutines. Eles são seguros para concorrência e são utilizados para sincronizar a execução das goroutines. [Channels](https://golang.org/doc/effective_go.html#channels)


### HTTP Server
O servidor possui um servidor HTTP que recebe as requisições de execução de workflows. Ele é responsável por receber as requisições, deserializar os workflows, criar os registros no banco de dados e deixar os workflows prontos para orquestração pelo Orquestrador.

### Monitor

O monitor é responsável por coletar métricas e logs dos recursos de execução de workflows. Ele coleta métricas de CPU, memória, disco e rede dos pods e jobs que estão em execução. Ele também coleta os logs dos pods e jobs que estão em execução. As métricas e logs coletados são armazenados em um banco de dados em arquivo (SQLite).

### Worker

O worker é responsável por executar os workflows. Ele é responsável por receber as mensagens da fila de execução, criar e gerenciar os recursos de execução de workflows, como pods e jobs. Ele coleta métricas e logs dos recursos de execução de workflows e envia para o monitor.

### Orquestrador

Aqui que está a inteligência do scik8sflow. O orquestrador é responsável por orquestrar a execução dos workflows. Ele é responsável por criar e gerenciar a fila de execução, ele verifica se os recursos de execução de workflows estão disponíveis e envia as mensagens para o worker. Ele garante que os workflows sejam executados de forma distribuída e paralela, respeitando a dependencia de dados entre os workflows.


## Cliente

O scik8sflow client é a ferramenta de linha de comando que permite a execução de workflows científicos. O cliente é responsável por enviar as requisições de execução de workflows para o servidor, ele serializa os workflow em formato base64 e envia para o servidor para criação e execução dos recursos necessários para a execução do workflow.

