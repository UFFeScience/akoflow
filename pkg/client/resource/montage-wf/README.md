# Workflow Montage

Aqui você encontra o workflow do Montage e seus resultados.

as Pastas que contém os workflows são:

wf-montage-050d - Workflow com 0.50 degrees.

A imagem dockerfile do workflow é baseada na imagem é a montage-0-50d.Dockerfile, dentro dele temos os dados para se rodar no Google Cloud Platform (GCP), usando o Google Kubernetes Engine (GKE).

Dentro desse pacote temos os resultados considerando as seguintes especificações:

## 0.50 degrees

- 0.50 degrees com 0.50vCPU para execução do workflow e um disco em cada container de 2GB. No total dando 58 atividades que serão executadas de acordo com sua dependência, seguindo a especificação do arquivo de workflow.


- 0.50 degrees com 1.00vCPU para execução do workflow e um disco em cada container de 2GB. No total dando 58 atividades que serão executadas de acordo com sua dependência, seguindo a especificação do arquivo de workflow.


- 0.50 degrees com 2.00vCPU nas atividades mProject e as demais com 0.5VCpu para execução do workflow e um disco em cada container de 2GB. No total dando 58 atividades que serão executadas de acordo com sua dependência, seguindo a especificação do arquivo de workflow.


- 0.50 degrees com 4.00vCPU nas atividades mProject e as demais com 0.5VCpu para execução do workflow e um disco em cada container de 2GB. No total dando 58 atividades que serão executadas de acordo com sua dependência, seguindo a especificação do arquivo de workflow.

Na pasta durations de cada experimento, temos os resultados de cada atividade, com o tempo de execução de cada uma delas.

## 1.00 degrees

- 1.00 degrees com 0.50vCPU para execução do workflow e um disco em cada container de 32GB. No total dando 472 atividades que serão executadas de acordo com sua dependência, seguindo a especificação do arquivo de workflow.

- 1.00 degrees com 1.00vCPU para execução do workflow e um disco em cada container de 32GB. No total dando 472 atividades que serão executadas de acordo com sua dependência, seguindo a especificação do arquivo de workflow.

- 1.00 degrees com 2.00vCPU nas atividades mProject e as demais com 0.5VCpu para execução do workflow e um disco em cada container de 32GB. No total dando 472 atividades que serão executadas de acordo com sua dependência, seguindo a especificação do arquivo de workflow.

- 1.00 degrees com 4.00vCPU nas atividades mProject e as demais com 0.5VCpu para execução do workflow e um disco em cada container de 32GB. No total dando 472 atividades que serão executadas de acordo com sua dependência, seguindo a especificação do arquivo de workflow.

Na pasta durations de cada experimento, temos os resultados de cada atividade, com o tempo de execução de cada uma delas.





