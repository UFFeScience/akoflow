# Análise crítica e frentes de evolução da documentação

Data: 2026-09-12. Estado: proposta editorial para discussão.

## Escopo e método

Revisão dos percursos de instalação, primeira execução, definição, planejamento,
execução, proveniência, infraestrutura e referência; inspeção da navegação, do
gerador de API, do plano de qualidade e do workflow de documentação.

A referência de publicação examinada é `origin/main` em `d9bf02d`, atualizada
por fetch. O checkout está em `issue/ako-12`, `8d7626a`: sua única diferença de
conteúdo em `docs/` em relação à main, antes deste relatório, é o guia adicional
`guides/hybrid-workspace-transfers.md`. Esse guia não foi tratado como conteúdo
publicado. O comportamento de autenticação foi confrontado também com a main
do repositório Desktop.

Esta é uma análise editorial e técnica. Não foram repetidos os experimentos,
instaladas as versões Desktop nem realizados testes de usabilidade ou inspeção
visual do site renderizado. As evidências históricas de execução são as
registradas nos guias; não representam novas validações desta revisão.

## Diagnóstico

A documentação já explica bem os registros e as fronteiras do sistema. Sua
próxima evolução deve fechar tarefas completas e tornar confiáveis os contratos
consultados durante essas tarefas. Hoje, um leitor pode compreender uma página
e ainda não conseguir passar à seguinte sem conhecimento externo.

Pontos a preservar:

- Separação entre definição, candidato, plano e execução observada.
- Exemplos com arquivos, contagens, bytes e checksums verificáveis.
- Distinção explícita entre fixture SLURM e cluster real.
- Explicação de que HEFT e PRISM têm modelos de previsão diferentes.
- Guias de proveniência com interpretação das telas e exportação de evidências.
- Geração do catálogo de rotas e verificação automatizada de links.

## Achados e locais de evolução

### DOC-01 — P0: fechar o percurso de primeira utilização

**Locais:** `docs/installation.md`, `docs/getting-started.md`,
`docs/guides/workflows/first-run.md`, `docs/downloads.md`.

A instalação apresenta o aplicativo empacotado e diz que não é necessário
copiar o token. O primeiro tutorial exige clone, curl, jq, token e endereço
da stack de desenvolvimento. A transição entre esses contextos não é ensinada.
A seção Desktop resume operações como recriar ambiente e criar escopo, mas não
oferece o mesmo detalhamento do percurso API. O preflight de instalação também
pressupõe shell e jq antes de o leitor instalar o aplicativo.

**Proposta:** manter um percurso inicial completo pelo Desktop, com arquivos
compatíveis para baixar, valores dos formulários e checkpoints. Dar ao percurso
API pré-requisitos e configuração próprios. Explicar instalação e primeira
abertura por sistema operacional; deslocar detalhes de publicação de releases
para conteúdo de manutenção.

**Aceite:** uma pessoa com apenas o aplicativo instalado consegue concluir e
verificar 3/3 atividades e duas transferências, sem precisar adivinhar um token,
clonar código ou reconstruir objetos a partir da descrição do modelo.

### DOC-02 — P0: corrigir contratos incorretos na referência gerada

**Locais:** `scripts/generate-api-reference.mjs`, `src/components/ApiEndpoint.tsx`,
`docs/reference/api-overview.md`. Alterar as fontes, não as páginas geradas.

Caso confirmado: a página gerada de `POST /instances/import/` informa
`hasRequestBody={false}` e `No request body`. O handler `ImportArchiveInstance`
lê `r.Body`, e o guia de instâncias corretamente envia um ZIP com
`Content-Type: application/zip`. O comando gerado não pode cumprir essa tarefa.

O componente também fixa JSON para requisições com corpo. A referência de
`POST /build-contexts/` mostra a alternativa JSON, sem apresentar o upload
multipart explicado no guia de artefatos. Tipos de campos são inferidos dos
exemplos, que usam valores como `string` e `0`; isso não descreve campos
obrigatórios, enums, restrições ou um payload válido. O comando usa
`@request.json`, mas o componente não exibe o exemplo de requisição completo.

**Proposta:** contratos explícitos para os endpoints prioritários, com mídia,
campos, payload mínimo válido, resposta e erros específicos. Identificar o que
é apenas uma forma inferida. Preservar a geração automática de método e rota.

**Aceite:** copiar o exemplo e fornecer os IDs/arquivos documentados basta para
fazer a requisição. Cobrir primeiro importação de instância, upload de contexto,
workflow, planejamento e execução; verificar esses exemplos contra os handlers.

### DOC-03 — P0: uniformizar conexão e autenticação entre guias

**Locais:** `docs/reference/api-overview.md`,
`docs/guides/operations/instance-management.md`,
`docs/guides/operations/troubleshooting.md`, exemplos curl dos guias.

`AKOFLOW_URL` significa origem sem prefixo no overview da API, mas inclui
`/akoflow-api` no guia de instâncias. Outros guias usam `AKOFLOW_API_URL` e
alternam os nomes das variáveis de token. Copiar comandos de páginas diferentes
pode resultar em URL incorreta.

O troubleshooting manda salvar um token no navegador para resolver 401/403.
Esse controle existe na interface, mas o proxy do Desktop empacotado injeta seu
próprio token em `electron/desktop-server.cjs`. A orientação precisa distinguir
Desktop empacotado, interface web/desenvolvimento e acesso direto à API.

No showcase local, o script usa o token configurado, mas o curl de consulta que
vem depois não envia o cabeçalho. Exportar uma variável não autentica curl.

**Proposta e aceite:** adotar uma convenção única, explicada em uma página
canônica; todos os snippets devem funcionar na modalidade declarada. Conferir
os erros 401 e 403 separadamente, conforme o motivo retornado pelo servidor.

### DOC-04 — P1: ensinar decisões e adaptação de um experimento

**Locais:** `docs/guides/workflows/definitions.md`, `planning.md`, `executions.md`,
`docs/explanations/prism-and-heft.md`, `docs/guides/data/provenance-and-audit.md`.

O primeiro tutorial usa um plano fixo, uma boa escolha para verificar a
instalação. Falta um próximo exercício contínuo que transforme esse resultado
em autonomia: adaptar o workflow, gerar alternativas, escolher, executar e
interpretar diferenças. As peças existem, mas o leitor precisa montar a aula.

**Proposta:** novo tutorial em `docs/tutorials/compare-plans.md`, reutilizando
o exemplo inicial. Alterar uma duração ou link; comparar candidatos HEFT/PRISM;
executar as alternativas; consultar previsto versus observado; exportar SQL,
parâmetros e resultados. Explicar o que a evidência permite concluir e as
diferenças entre os modelos de custo e tempo.

**Aceite:** cada etapa tem entradas conhecidas, resultado esperado e próxima
ação. O leitor conclui com um pequeno conjunto de evidências reproduzíveis.

### DOC-05 — P1: organizar a descoberta por objetivo e nível de validação

**Locais:** `sidebars.ts`, `docs/getting-started.md`, `docs/showcase/index.mdx`,
`docs/reference/feature-coverage.md`.

A divisão Tutorials/How-to/Explanations/Reference é útil, mas a categoria de
tutoriais também contém download, instalação e mapa de interface. A matriz
`feature-coverage.md` não está na sidebar nem tem links encontrados nas outras
páginas examinadas. O índice de showcases promete percursos equivalentes
Desktop/API para todos; o SLURM requer iniciar uma fixture por terminal.

**Proposta:** oferecer entradas por objetivo: primeira execução, infraestrutura
real, comparação de planos, análise de resultados e automação. Manter a
organização por tipo documental abaixo dessas entradas. Identificar cada
showcase como simulação, execução real ou fixture, incluindo público,
pré-requisitos, versão testada e o que não foi validado. Decidir se a matriz é
referência pública ou instrumento de manutenção e posicioná-la de acordo.

**Aceite:** o leitor sabe qual exemplo escolher e o que terá comprovado ao
terminá-lo. Nenhum recurso relevante depende de conhecer sua URL diretamente.

### DOC-06 — P1: completar procedimentos de infraestrutura com evidência

**Locais:** `docs/guides/infrastructure/gcp.md`, `aws.md`, `hpc-slurm.md`,
`docs/showcase/slurm-local-fixture.mdx`, `quality-plan.md`.

O GCP já distingue inventário de chamadas e permissões efetivamente validadas.
O guia AWS descreve configuração, mas não oferece um payload completo e
verificado de armazenamento S3. O plano de qualidade registra ambas as
validações externas como pendentes. A fixture SLURM comprova o adaptador local,
não o ciclo completo em um cluster institucional.

**Proposta:** cenários pequenos com preparação, teste de conexão, tarefa,
resultado e limpeza. Validar GCP e S3 em recursos descartáveis e SLURM em um
ambiente autorizado quando disponíveis. Enquanto isso, explicitar o nível de
evidência e conferir os procedimentos de interface contra a versão suportada.

**Aceite:** registrar versão, ambiente, permissões observadas, comando ou
percurso executado, resultado e limpeza. Revisão de código, fixture local e
validação real devem ser estados distintos.

### DOC-07 — P1/P2: impedir que a documentação volte a ficar defasada

**Locais:** `.github/workflows/docs-checks.yaml`, `quality-plan.md`,
`docs/contributing/documentation-plan.md`, downloads dos showcases.

O workflow roda por mudanças em docs e exemplos, mas não por alterações apenas
no router, handlers ou domínio que alimentam a referência. Assim, uma mudança
de API isolada pode não regenerar e publicar o site. O plano editorial ainda
cita `akoflow-admin`; o resumo do quality-plan mantém a fixture SLURM como
pendência apesar de registrá-la como concluída. Os downloads de exemplos usam
`main`, uma referência móvel, sem assegurar compatibilidade com o Desktop
instalado pelo leitor.

**Proposta:** ampliar gatilhos de CI para fontes dos contratos; manter um
backlog único de estado atual; registrar versão/revisão testada dos exemplos;
publicar bundles ligados à versão suportada; executar exemplos canônicos em
ambientes isolados com verificações de resultado.

**Aceite:** uma alteração de API dispara os checks apropriados. Cada pendência
tem local, critério de aceite, evidência e dependência externa quando houver.
Um link válido é tratado como prova de existência, não de compatibilidade.

## Ordem de trabalho proposta

1. **Correções de confiança:** DOC-02 e DOC-03, começando pelos casos confirmados.
2. **Percurso de entrada:** DOC-01, validado do começo ao fim no Desktop.
3. **Autonomia científica:** DOC-04 e entradas de navegação da DOC-05.
4. **Infraestrutura:** DOC-06 conforme disponibilidade dos ambientes de teste.
5. **Manutenção contínua:** iniciar gatilhos e backlog da DOC-07 cedo; ampliar
   a execução automatizada conforme os tutoriais sejam validados.

Para cada unidade, registrar: problema do leitor, arquivos, resultado esperado,
evidência necessária e critério de conclusão. Medir tarefas concluídas sem
ajuda e exemplos executáveis, além de páginas publicadas e build bem-sucedido.

Capturas adicionais devem mostrar escolhas, transições e interpretação do
resultado. O trabalho visual deve incluir revisão em largura estreita,
legibilidade das capturas e navegação por teclado; esses pontos ainda precisam
de inspeção visual para se tornarem achados confirmados.

## Verificações desta revisão

Após instalar as dependências pelo lockfile com `npm ci --ignore-scripts`,
passaram `npm run typecheck`, `npm run build` e `npm run check:links` em `docs/`.
Foram geradas 125 páginas de endpoints e verificados 290 links locais e 53
downloads de showcases. A verificação de downloads compara os arquivos com o
repositório; não testa sua disponibilidade pela rede. O build foi executado no
checkout descrito no escopo e não comprova que os procedimentos são executáveis.

O único arquivo versionável adicionado nesta revisão é este relatório.
