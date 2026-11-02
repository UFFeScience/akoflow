# Montage 58 / PRISM-TIME / HPC + medium: execução com ordem obrigatória

A repetição concluiu **58 atividades, em 58 tentativas, sem recuperação**.
O auditor verificou **149 relações de precedência**, incluindo dependências
do workflow e sequência por recurso/núcleo/slot, com **zero violações** e
alocação correspondente ao plano. O auditor também detectou as quatro
violações conhecidas na execução anterior, como controle negativo.

## Resultados

| Execução | Planejado (s) | Simulado (s) | Real (s) | Erro absoluto da simulação |
|---|---:|---:|---:|---:|
| Anterior, ordem flexível | 3003,88 | 3007,14 | 5256,82 | 42,80% |
| Nova, ordem obrigatória | 3003,88 | 3007,14 | 5212,84 | 42,31% |

Erro = `100 * abs(simulado - real) / real`. A diferença entre os tempos
reais é pequena: a correção confirmou o cumprimento da ordem, mas não
eliminou o erro de previsão. Não se atribui causalmente a diferença de
duração à correção, pois as velocidades reais de serviço também variaram.
O histórico de 4143,89 s com recuperação pertence a outro plano/campanha
e permanece preservado separadamente.

A estimativa final de ciclo de vida das VMs e discos foi USD 0,101562073,
contra USD 0,103345813 na execução anterior. Não são faturas; HPC,
egress de rede, IPv4 público, imagens, descontos e impostos ficam fora
dessa base. O volume de rede real foi 8,233037026 GB decimais.

## Protocolo e comprovação da ordem

- Daemon atualizado a partir de `230b2fc`, após confirmar zero execuções
  ativas; sem mudança de esquema ou limpeza do banco de dados.
- Plano anterior reutilizado integralmente:
  `fig8-prism-time-matched-20261031-v1-medium-0e673f4ddfbb`.
- Novo run: `prism-time-medium-strict-20261031-v1-medium-real`.
- Seed 1; workflow, recursos, calibrações, rede e regras de transferência
  e limpeza preservados. Hash normalizado incluindo o plano:
  `9c11a54e5cb585c9a7a33e58cf64a2bd615191e73dc0879cbce9f77c11a7054e`.
- HPC: uma sequência em diablo03; duas e2-medium com duas sequências cada,
  discos padrão de 30 GB, conforme a campanha anterior.
- Auditor read-only `cmd/montage-order-audit`, commit `90f1e76`, consultando
  eventos de início/conclusão a cada 15 s e conferindo os recursos usados.
- Exemplo crítico: o evento de conclusão de mBackground34 ocorreu em
  01/11/2026 00:30:25.833908044 UTC; o evento de início/submissão aceita de
  mProject2 só ocorreu em 00:30:27.656368253 UTC. No Slurm, a computação
  efetiva começa após a fila; o auditor exige que até a submissão respeite
  a predecessora. No legado, mProject2 havia sido adiantada.
- Nenhum restart, novo plano ou ajuste do modelo durante a execução.

As datas UTC acima correspondem à noite de 31/10 em America/Sao_Paulo.
O run completo durou de 20:40:04 a 22:07:49 no horário local; esse intervalo
inclui etapas fora do makespan, que mede da primeira atividade iniciada
à última atividade terminada.

## Onde permanece a diferença simulado/real

O HPC tem uma única sequência, com 30 atividades, incluindo a primeira e
a última do workflow. Assim é possível reconciliar exatamente o makespan
pela soma de intervalos consecutivos desse caminho, sem somar trabalhos
de transferência paralelos.

| Parcela desse caminho do HPC | Simulado (s) | Real (s) | Diferença (s) |
|---|---:|---:|---:|
| Computação | 1797,15 | 1827,68 | 30,54 |
| Demais intervalos | 1209,99 | 3385,16 | 2175,16 |
| Total | 3007,14 | 5212,84 | 2205,70 |

A identidade fecha com resíduo zero nos dois modos. A computação do HPC
ficou aproximadamente 1,7% acima da previsão. Cerca de 98,6% da diferença
total está nos demais intervalos do caminho: preparação, transferências,
espera por dados de outros recursos, lançamento/Slurm e observação.
Isso é uma localização temporal do erro, não uma atribuição causal
exclusiva à rede ou ao disco.

Preparações reais de consumidores importantes, com uma única medida
elapsed por consumidor, sem multiplicar as barreiras compartilhadas:

| Atividade | Anterior (s) | Nova (s) |
|---|---:|---:|
| mConcatFit30 | 669,25 | 329,13 |
| mConcatFit11 | 209,25 | 207,16 |
| mViewer58 | 431,40 | 420,22 |

A simulação conserva taxas históricas e links assumidos; a auditoria de
ordem não valida esses coeficientes. Há ainda um ponto de contrato a
investigar: os fluxos do SimGrid podem começar quando todos os produtores
estão prontos, antes da predecessora da sequência; o executor real só
admite a preparação após essa predecessora. Esse comportamento não foi
alterado nem recalibrado durante a campanha. O campo de preparação local
simulado no CSV não inclui necessariamente todas as etapas anteriores
de transferência e não deve ser comparado sozinho ao preparo real completo.

## Limpeza e preservação

- Duas VMs, discos e firewalls direcionados removidos; conferência
  independente nas APIs do Google Compute com inventários completos.
- Oito arquivos temporários de credenciais removidos, com guarda de
  estado destruído e Terraform vazio; credenciais-base preservadas.
- Três PNGs finais do HPC conferidos contra os hashes dos manifests,
  decodificados como PNG e preservados em arquivo tar.gz com CRC válido.
  A imagem colorida foi inspecionada visualmente.
- Arquivados 30 scripts Slurm do daemon e 90 arquivos remotos de logs,
  sentinelas e métricas, também com CRC válido.
- Nenhum job HPC pertencente à execução permaneceu ativo. Diretório
  remoto exato do run, seus logs e diretório local dos scripts removidos.
  Cache compartilhado de imagens e ambientes preservados.
- A checagem final encontrou zero execuções ativas; uso da conta HPC
  voltou a 1657 MiB de quota de 20480 MiB. São observações de quota,
  não uma medição contínua de pico.

Arquivo de produtos:
`outputs/prism-time-medium-strict-20261031/hpc-final-products.tar.gz`.
SHA-256: `84285d670e41ff87060ff2c5385519c82a32e483d95a0ca02cb5efa353f61f78`.

## Arquivos e reprodução

`comparison.csv` resume as duas execuções; `activities.csv` contém os
tempos das 58 atividades. Os JSONs preservam as auditorias resumidas.
A última coluna do CSV é a diferença entre início registrado pelo Slurm
e confirmação da submissão no daemon; não inclui todo o lançamento remoto.
Snapshots completos, manifests, logs e produtos ficam no diretório local
`outputs/prism-time-medium-strict-20261031/`; os resultados anteriores
não foram sobrescritos e o paper não foi alterado nesta repetição.

Com os snapshots locais preservados, os exports podem ser reproduzidos:

```
go run experiments/prism-medium-strict-20261031/report.go
go run experiments/prism-medium-strict-20261031/serial_accounting.go
```

Ambos os comandos exigem a auditoria final completa e não submetem
execuções nem alteram infraestrutura.
