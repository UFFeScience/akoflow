# PRISM-TIME no Montage 58 / HPC + medium: diagnóstico

Esta análise preserva a campanha concluída. Nenhum plano executado, CSV,
gráfico ou calibração foi alterado; os replays abaixo são diagnósticos locais,
sem provisionamento nem execução física.

## Evidência verificada

| Etapa (s) | HEFT | PRISM-TIME |
|---|---:|---:|
| Previsão nativa | 1736.356864 | 3003.882921 |
| Simulação da campanha | 2521.499463 | 3007.138091 |
| Execução real | 3669.887435 | 5256.823834 |
| Mesmo avaliador interno do PRISM, diagnóstico | 2528.622680 | 3003.882921 |

As previsões nativas têm coberturas diferentes e não medem isoladamente
qualidade de busca. O avaliador interno do PRISM e o simulador da campanha
são implementações distintas: os valores diagnósticos não substituem as
observações simuladas da campanha.

O replay com beamWidth=120, readyBranchLimit=3 e optionCount=25 reproduziu
exatamente a previsão original de 3003.8829210098506 s. A auditoria original
`outputs/fig8-canonical-20261031/prism-time-followup/input-equivalence-audit.json`
confirma entradas científicas iguais nos quatro deployments medium. PRISM
completou 58 atividades, todas com attempt=1, sem recuperação.

## Busca e implementação

- `internal/planning/algorithms/prism_state.go`, compactPRISMPlacementStart:
  o início usa max(data-ready, disponibilidade final do núcleo). O estado
  registra o fim da agenda, sem procurar lacunas internas.
- `internal/planning/algorithms/heft.go`, evaluateHEFTCalendar: percorre o
  calendário e insere uma tarefa em uma lacuna quando ela cabe. No plano
  HEFT desta campanha, quatro atividades foram inseridas antes do fim da
  agenda já construída, segundo a ordem de construção persistida no array
  de assignments. No PRISM executado, a contagem é zero.
- `prism.go`: a avaliação detalhada dos planos completos acontece depois
  da busca limitada e da deduplicação. Ela não recupera estados já podados.
- O HEFT avaliado pelo próprio avaliador do PRISM continua melhor. Isso
  demonstra uma oportunidade perdida pela busca; não é apenas diferença
  entre previsões nativas nem somente variação da nuvem.
- Os 25 planos do replay padrão têm a mesma previsão e somente duas
  configurações distintas de atividade/recurso/núcleo. Os calendários e
  prioridades não são todos idênticos: não se deve chamar os 25 de duplicatas.

A ausência de inserção e a poda limitada são limitações concretas. Ainda
não foi isolado quanto cada uma explica da diferença. Não há evidência
suficiente para atribuir todo o resultado a uma única linha de código.

## Sensibilidade local (não são novas execuções da campanha)

| beamWidth | readyBranchLimit | Melhor previsão PRISM (s) |
|---:|---:|---:|
| 120 | 3 | 3003.882921 |
| 480 | 3 | 3032.544268 |
| 120 | 16 | 3163.844290 |
| 1200 | 16 | 3379.017377 |

Aumentar os parâmetros não corrigiu este caso. As fronteiras retidas não
são conjuntos aninhados: a política de seleção e suas cotas de diversidade
mudam com a largura. Os resultados não provam que toda ampliação piora.

Reprodução, a partir da raiz, usando os snapshots locais preservados:

```
go run ./experiments/prism-medium-diagnostic-20261031/replay.go 120 3
go run ./experiments/prism-medium-diagnostic-20261031/replay.go 480 3
go run ./experiments/prism-medium-diagnostic-20261031/replay.go 120 16
go run ./experiments/prism-medium-diagnostic-20261031/replay.go 1200 16
```

Saídas: `outputs/prism-medium-investigation-20261031/replay-*.json`.
A comparação pelo avaliador interno foi feita com um teste diagnóstico
local, preservado como `common-evaluator-test.go.txt` nessa pasta de outputs;
o log está em `common-evaluator.log`. O teste temporário foi removido do
pacote de produção após a verificação.

## Execução real e transferências

PRISM atribuiu 30 atividades ao HPC e 28 à nuvem; HEFT, 17 e 41.
O volume de rede do PRISM foi menor (8.23 contra 9.86 GB decimais), mas
isso não garante menor tempo de preparação no caminho do workflow.

| Atividade | Preparação HEFT (s) | Preparação PRISM (s) |
|---|---:|---:|
| mConcatFit30 | 225.24 | 669.25 |
| mConcatFit11 | 23.62 | 209.25 |
| mViewer57 | 7.05 | 142.46 |

mConcatFit30 ficou no HPC nos dois planos; mConcatFit11 ficou na nuvem
no HEFT e no HPC no PRISM. Seus tempos de computação são curtos frente
à preparação. Os intervalos de preparação incluem transferência e
materialização; não são uma medição isolada da transmissão de rede.

O trabalho agregado em fases de transferência foi 11784.48 s no PRISM
contra 6629.83 s no HEFT. Há operações concorrentes: esses totais não podem
ser somados ou subtraídos como uma decomposição causal do makespan.
A preparação final de mViewer58 foi menor no PRISM (431.40 contra 447.86 s),
portanto nem toda preparação foi mais lenta.

## Próxima validação proposta

Isolar inserção em lacunas e diversidade da busca em replays, com os
mesmos dados e o mesmo avaliador. Verificar se o plano HEFT é alcançável
pelo espaço de estados do PRISM ou em qual etapa sua trajetória é
excluída. Uma eventual inclusão do HEFT como candidato de referência
seria uma alteração explícita do algoritmo, não uma correção retroativa
da campanha. A execução real existente permanece válida como observação.
