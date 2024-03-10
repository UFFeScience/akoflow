# uff-tcc-scientific-workflow-k8s

Repositório para os artefatos de código e documentação desenvolvidos para o Trabalho de Conclusão de Curso (TCC) do curso de Sistemas de Informação


## Tema
Workflow Científico em núvem com Kubernetes

## Objetivo
...

### Objetivos Específicos
...


### instalar a api de servidor para o k8s

kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

### for development

kubectl edit deployment.apps/metrics-server -n kube-system

--kubelet-insecure-tls=true

---
### to do

- nova versão do arquivo de workflow, CPU e Mémoria por Atividades.
- mecanismo de atividades sequencial
- v1 do client, recebendo o arquivo de workflow e enviando para o server (k8s)
- mecanismo de compartilhamento de dados entre atividades
- proveniencia e monitoramento
- mecanismo de atividades paralelas
