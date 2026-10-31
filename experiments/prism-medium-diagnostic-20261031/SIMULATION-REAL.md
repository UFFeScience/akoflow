# Escalonamento e fidelidade da simulação: HPC + medium

A análise foi ampliada após a pergunta sobre a precisão do modelo. Trata
apenas de Montage 58 / HPC + medium. Os dados micro não foram usados para
justificar o erro desta execução. Não houve execução física, alteração de
calibração ou substituição dos resultados da campanha.

## Resultado e escopo

PRISM-TIME: planejado 3003.882921 s; simulado 3007.138091 s; real
5256.823834 s. Diferença real menos simulado: 2249.685743 s; erro absoluto
com denominador real: 42.795532%. O real é 74.811521% maior que o simulado.
A concordância entre planejado e simulado não demonstra validade externa:
ambos usam a mesma família de hipóteses de modelagem.

O monitor da simulação foi recuperado por GET do daemon local e preservado
em `outputs/prism-medium-investigation-20261031/simulation-monitor.json`.
O relatório Go verifica 58 atividades únicas, concluídas e com a mesma
alocação nos dois modos. O resultado físico continua com 58 tentativas,
sem recuperação.

## 1. O que o PRISM-TIME escalonou

A busca usa beamWidth=120, readyBranchLimit=3 e optionCount=25. A função
objetivo é tempo, não custo. Os 58 bindings são 30 atividades no HPC,
14 na primeira medium e 14 na segunda medium. O HPC dispõe de uma vaga;
cada medium, duas. O modelo não contém matriz de interferência.

O replay reproduz a previsão original. As limitações da busca e a comparação
com HEFT pelo mesmo avaliador estão no README deste diretório. Há ainda
uma divergência mais básica no significado de executar o plano:

- `internal/controlplane/execution/admission.go`: a ordem planejada é uma
  preferência, explicitamente não uma dependência. O supervisor ordena as
  atividades atualmente prontas e pode adiantar outra quando uma tarefa
  anterior não está pronta. A capacidade é controlada por recurso, não
  como compromisso de respeitar cada cadeia persistida por CoreID.
- `simgrid-runner/src/main.cpp`: `resourceLaneOrders` cria sinais de
  precedência obrigatórios entre tarefas da mesma lane. O avaliador
  interno do PRISM também usa essas cadeias.

Logo, manter o mesmo SchedulePlan e os mesmos recursos não implica que o
simulado e o real executem a mesma ordem. A auditoria anterior de inputs e
assinaturas não verificava a conformidade da sequência efetivamente observada.

Exemplo inequívoco no HPC de uma única vaga: o plano coloca mBackground34
antes de mProject2. Na simulação, mBackground34 termina em 1762.68 s e
mProject2 começa nesse instante. No real, mProject2 começa em 1380.39 s,
mas mBackground34 só termina em 3489.58 s. Não é um desvio de poucos
segundos de relógio. Há quatro relações da ordem planejada que não são
respeitadas como precedência pelo real; a simulação respeita todas.
Nas VMs de duas vagas, isso não demonstra violação da capacidade física:
a lane persistida não é necessariamente uma CPU fisicamente fixada.
As dependências do workflow são respeitadas nos dois modos.

### Contrafactuais locais, sem recalibrar

Mesmo avaliador interno do PRISM, mesmos bindings e física:

| Ordem imposta ao avaliador | Previsão (s) |
|---|---:|
| Original | 3003.882921 |
| Só HPC ordenado pelos inícios reais | 2808.192493 |
| Todas as lanes originais ordenadas pelos inícios reais | 3123.940704 |

Esses contrafactuais isolam alterações de ordem estática no avaliador;
não reproduzem o despacho dinâmico real nem demonstram qual política é
melhor. Mostram também que a divergência de ordem, sozinha nesses testes,
não explica os 5256.82 s: continuam faltando tempos de serviço observados.
Teste diagnóstico e log preservados em `order-counterfactual-test.go.txt`
e `order-counterfactual.log` na pasta de outputs, fora do pacote de produção.

## 2. Onde o simulado difere do real

Comparação com limites iguais: da conclusão do último produtor do workflow
até o início da atividade consumidora. Inclui rota, preparação e espera
por recurso/lançamento; não é tempo isolado de transmissão.

| Atividade | Simulado (s) | Real (s) |
|---|---:|---:|
| mConcatFit30 | 117.42 | 684.36 |
| mConcatFit11 | 111.33 | 413.15 |
| mViewer58 | 362.77 | 528.31 |

Comparar somente os 27.24 s de `workspacePreparationSeconds` simulados de
mConcatFit30 com os 669.25 s de preparação real seria incorreto: no simulado
há ainda etapas de transferência anteriores a essa fase. A tabela acima
usa o intervalo completo comparável. Não se somam essas diferenças como
explicação independente dos 2249.69 s, pois há filas e concorrência.

### Transferência, relay e junção

mConcatFit30 recebe dados de seis produtores. Para o produtor mDiffFit29,
na medium-2, há evidência de dois trechos reais:

- arquivo compactado: 206539861 bytes;
- pack-and-receive: 247.09 s;
- archive-upload: 374.88 s;
- validação e extração são fases adicionais;
- a barreira compartilhada de preparação dura 637.43 s;
- a junção/verificação compartilhada posterior dura 28.45 s.

A barreira ocorre enquanto os trechos de transferência acontecem: ela não
deve ser somada a eles. Também é repetida em registros de produtores que
compartilham a mesma preparação; somá-los multiplicaria a espera.

O perfil congelado medium -> HPC usa ingressArchiveBytesPerSecond=
12094092.53 e uploadArchiveBytesPerSecond=3368447.57, com **samples=1**.
Para o arquivo observado, só os tempos nominais bytes/taxa seriam cerca
de 17.08 e 61.32 s, antes de outras fases e concorrência. Os tempos reais
foram 247.09 e 374.88 s. Isso identifica uma inadequação dos coeficientes
para essa condição, mas não separa causalmente rede, empacotamento,
compartilhamento de uplink, CPU e disco.

A topologia ainda marca os links do controlador como
`measurementStatus=assumed-inherited-200Mbps`; o próprio campo
linkBandwidthBasis declara hipóteses herdadas. O modelo congelado não
configura BulkServiceGroupID/BulkServiceSlots nos recursos. Existe suporte
no código, mas ele não representa filas de capacidade de serviço neste
snapshot. Portanto, não há base para exigir precisão quase exata dessa
entrada: ela ainda combina hipóteses e taxas históricas de pouca amostragem.

### Computação também varia

Não é exclusivamente transferência. mProject41 na medium-1 tem 830.23 s
simulados contra 1213.36 s reais; mProject23 na medium-2, 578.41 contra
912.93 s. Os totais de runtime, somados sobre todas as tarefas e recursos,
são 7916.60 s simulados e 8386.62 s reais. Esses totais se sobrepõem no
tempo e não podem ser adicionados ao makespan.

O script também reconstrói uma cadeia temporal usando apenas dependências
do workflow, com soma telescópica igual ao makespan e sem violações de
precedência. A cadeia que determina o último produtor muda: no trecho
final mDiffFit9 depende de mProject2 no caso simulado e de mProject4 no
caso real como produtor temporalmente mais tardio. O restante engloba
mConcatFit11, mBgModel12, mBackground16, mImgTbl17, mAdd18 e mViewer58.
O residual inclui esperas externas e de recurso; não é uma decomposição
causal de rede. Cadeias distintas impedem interpretar sua diferença de
runtime como um efeito isolado.

## Conclusão e validação necessária

Há três questões verificadas: seleção limitada de planos pelo PRISM,
contrato de ordem diferente entre simulação e execução, e estimativas de
serviço que subestimam fases reais de preparação/transferência e algumas
atividades de computação. Corrigir apenas a busca não corrige a fidelidade.

Antes de uma nova campanha: definir e alinhar a política de despacho entre
planejador, simulador e executor; auditar ordem, capacidade e fases além dos
bindings; medir separadamente os serviços de empacotamento, controlador,
links e extração sob a concorrência usada; e validar as previsões em dados
independentes. Ajustar um fator para reproduzir essa mesma execução seria
calibração retrospectiva, não validação de precisão. Uma tolerância baixa
precisa ser medida em repetições, não prometida como exatidão numérica.

Reprodução da análise temporal, após obter o monitor preservado:

```
go run experiments/prism-medium-diagnostic-20261031/phases.go
```

Saídas: `phase-ledger.json` e `phase-ledger.log` em
`outputs/prism-medium-investigation-20261031/`. O script não inicia execuções.
