# Pacote do Experimento Preliminar do Montage em 50d no akoflow. Local com 500m CPU


O Arquivo [wf-montage-050d.yaml](wf-montage-050d.yaml) possui o workflow definido para execução pelo akoflow. Ele foi gerado a partir do workflow original do Montage, disponível em [montage-workflow-0.9.tar.gz](https://montage.ipac.caltech.edu/docs/download.html). O workflow foi adaptado para execução no akoflow.

O Arquivo [wf-montage-050d_job_durations.txt](wf-montage-050d_job_durations-local.txt) contem os tempos de execução de cada job do workflow Montage em 50d. O workflow foi executado em um ambiente local com 500m CPU e 256Mi de memória.

O arquivo [ALL_montage-0-50d-filelist.txt](activities_file_list/ALL_montage-0-50d-filelist.txt) possui a lista de arquivos de entrada e saída para cada atividade combinada do workflow Montage utilizado no akoflow.

Nessa pasta, também estão disponíveis os arquivos de entrada e saída de cada atividade do workflow Montage.

A imagem [montage-0-50d.Dockerfile](montage-0-50d.Dockerfile) contém a definição da imagem Docker utilizada para execução do workflow montage no akoflow.

A imagem ![mosaic-color](mosaic-color.png) é a imagem resultante da execução do workflow Montage em 50d no akoflow. Ela é uma das imagens geradas pelo workflow, a principal imagem de saída do workflow Montage.



# Pacote do Experimento Preliminar do Montage em 50d no akoflow. GKE 1 NODE, 16vCPU, 16GB RAM (e2-highcpu-16)


Duração da execução do workflow Montage em 50d no akoflow com 1 nó GKE, 16vCPU e 16GB de RAM (e2-highcpu-16). Disponível em [wf-montage-050d_1vcpu_job_durations](wf-gcloud/wf-montage-050d_1vcpu_job_durations-gcloud.txt).