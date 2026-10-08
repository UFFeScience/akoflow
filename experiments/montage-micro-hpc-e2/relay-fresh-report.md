# Montage 58 — execução real concluída

Run: `montage-micro-hpc-e2-real-20261008-relay-fresh-v1`.
Resultados detalhados e tempos das 58 atividades: [relay-fresh-comparison.json](relay-fresh-comparison.json).
Produto exportado: `storage/montage-fog-20261006/mosaic-color.png` (arquivo local preservado, não depende das VMs).

## Resultado e integridade

58 IDs esperados concluídos na última tentativa; nenhum ID ausente ou inesperado.
122 registros de transferência concluídos, cobrindo as 114 dependências de workspace.
Zero transferências terminando após o início das consumidoras, zero leases não liberados,
zero sobreposições de execução nas dez VMs micro com capacidade de uma atividade.
Produto PNG RGB 1800 × 1800, 4.920.343 bytes, renderização conferida.
SHA256: `0f75c2e811e89ac56ae44784fa22e4e01368abf19a10cbe59a77bd19446fdf4a`.

Máquinas: dez e2-micro, duas e2-medium e PlaFRIM; distribuição do plano:
40 atividades micro, dez medium e oito HPC. O run executou todas as atividades com
workspaces próprios; as VMs existentes foram reativadas, portanto não é benchmark
de provisionamento frio. Duas recuperações foram necessárias, preservando resultados
produzidos neste próprio run. Não houve reutilização de atividades de runs anteriores.

## Comparação

| Métrica (segundos, salvo indicação) | Plano | SimGrid | API real |
| --- | ---: | ---: | ---: |
| Makespan | 3617,938 | 3617,498 | 7796,457 |
| Compute agregado | 27568,495¹ | 27557,976 | 13131,178 |
| Transfer agregado | — | 39,960 | 1895,052 |
| Queue agregado | — | 5162,888 | 16026,769 |
| Interferência reportada | — | 3339,976 | 0 |
| Overhead reportado | — | 0 | 35,795 |
| Bytes reportados pela API | — | 880135168 | 4249956208 |

¹ Soma dos runtimes previstos nas atribuições, não makespan.

Makespan real reportado: 129,94 min, aproximadamente 2,155 vezes o plano (60,30 min).
Não é uma comparação contínua limpa: pausas/recuperações contaminam os tempos.
Os timestamps de início/fim do run representam apenas a última recuperação (589 s),
não os 7796 s do makespan acumulado. Queue, compute e transfer podem sobrepor-se;
não somar essas colunas para reconstruir makespan. Há timestamps nativos validados
para 50 handles cloud; não extrapolar essa validação para os oito handles HPC.

Custo zero não foi calibrado e não significa execução gratuita. A velocidade micro
0,25 é aproximada, a matriz e2-medium foi extrapolada para micro/HPC e a rede do plano
foi assumida. Interferência real reportada zero não prova ausência de contenção física.

## Rotas e tráfego

24 dependências cloud ↔ PlaFRIM usaram relay (12 em cada sentido); nenhuma rota
direta violou a política. Quatro rotas HPC ↔ HPC usaram NFS comprovado. Entre cloud,
72 rotas privadas autenticadas e 14 cópias locais. Oito registros adicionais são
reutilizações verificadas de artefato de runtime, não dependências de workspace.

Tráfego contabilizado por rotas: 33.564.916.733 bytes, dos quais 10.486.984.513 bytes
pelo relay e 23.077.932.220 nas rotas diretas. Esses contadores não são equivalentes
ao `transferredBytes` agregado da API: incluem múltiplos saltos/arquivos empacotados.
O workspace inteiro foi mantido, portanto não comparar esses volumes como se fossem
apenas os outputs/arestas estimados no plano.

## Pontos de atraso medidos

| Consumidora | Preparação total | Execução reportada |
| --- | ---: | ---: |
| mImgtbl55 | 975,990 s | 2,939 s |
| mAdd56 | 902,054 s | 8,433 s |
| mBackground15 | 771,440 s | 3,896 s |
| mConcatFit11 | 656,020 s | 2,755 s |

Em mAdd56: espera conjunta de preparação 619,040 s; merge 283,014 s
(checagem 180,436 s, cópia 55,054 s, verificação 46,694 s).
São 46 preparações conjuntas distintas; o relatório conta cada barreira/merge uma vez,
não uma vez por predecessora. Trabalho total desses merges: 2223,347 s, sobreposto
entre atividades, não duração de caminho crítico.

Nas 24 rotas relay, trabalho de pack/receive 2087,705 s, validação de arquivo 42,245 s,
upload 5332,971 s e verificação/extração no destino 326,661 s. Execução rsync medida
5326,674 s versus espera de canal SSH 6,289 s. Portanto a fila de canal não explica
a maior parte do upload. Esses valores são somas de operações concorrentes e não
prova isolada de gargalo de rede ou CPU. Nas rotas cloud, aquisição de snapshot,
extração e merge também custam tempo; reduzir apenas a fila de transferências não
elimina o custo de empacotar/verificar um workspace inteiro.

mProject21 executou em 1808,364 s versus 1444 s previstos; outros mProject ficaram
abaixo da previsão. A aproximação de velocidade não é uniforme nem calibrada para
todas as atividades. Não atribuir toda a diferença de makespan ao relay.

## Correções relevantes aplicadas e evidências

1. AKO-99/100: respeitar instância explícita e não reutilizar provisionamento terminal
   de uma geração já destruída na recuperação.
2. AKO-120: preservar capacidade micro de uma atividade; auditoria final sem overlap.
3. AKO-125/126/127: rotas cloud privadas elegíveis, quoting seguro e diagnóstico da
   conectividade autenticada; 72 rotas privadas concluídas neste run.
4. AKO-128: separar fases de preparação/runtime Docker para não confundir launch com
   transferência; instrumentação por tentativa preservada.
5. AKO-129: reutilização de snapshot selado privado com validação; evidências anteriores
   registradas em `single-lane-snapshot-reuse-real-evidence.json`.
6. AKO-130: medir espera de canal e execução rsync separadamente; evidência real mostra
   espera pequena perante upload, sem afirmar causa exclusiva.
7. AKO-131: merge streaming, conflitos explícitos e validação de conteúdo; barreiras
   finais respeitadas. O merge ainda é significativo em workspaces grandes.
8. AKO-132: isolar stdout de diagnóstico SSH na validação do diretório de preparação;
   primeira recuperação progrediu de 45 para 56 atividades concluídas.
9. AKO-133: preservar categoria limitada de falha SSH na criação de snapshot selado,
   sem contaminar protocolo nem expor stderr sensível. A falha transitória original
   não teve causa exclusiva comprovada; a recuperação concluiu os dois viewers.
10. AKO-134: remover somente inputs cloud inalterados, com identidade/caminho/hash
    validados, inclusive arquivos criados por Docker root; não remover outputs.
11. AKO-135: auditoria por IDs/tentativas, cobertura de dependências, política do relay,
    contabilidade de estágios conjuntos e exportação científica por allowlist.

Testes da auditoria: oito casos, 30 assertions. Testes de transferência das correções
AKO-132/133/134 passaram antes da aplicação no daemon. Alterações e evidências mantidas
na main local, sem push.

## Encerramento

PNG exportado e hash verificado antes da destruição. Doze operações destroy concluídas;
API sem instâncias não destruídas e sem operações cloud ativas. Consulta direta de leitura
ao GCP: zero instâncias e zero discos com prefixo gerenciado. As regras de firewall destas
doze instâncias foram removidas; três regras gerenciadas antigas de outras gerações
continuam no projeto, não são recursos computacionais pagos e não foram alteradas nesta
auditoria. PlaFRIM `squeue` vazio; diagnóstico direto não criou job `srun` e a configuração
temporária da conexão foi restaurada. Não há VM/disco pago desta campanha em uso residual.
