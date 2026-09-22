import fs from "node:fs/promises";
import path from "node:path";
import { SpreadsheetFile, Workbook } from "@oai/artifact-tool";

const root = process.argv[2] ?? process.cwd();
const out = path.join(root, "outputs/prism-paper-experiments");
const read = async (name) => JSON.parse(await fs.readFile(path.join(out, name), "utf8"));

const baseline = await read("baseline-analysis.json");
const sla = await read("sla-analysis.json");
const interference = await read("interference-analysis-r2.json");
const frontier = await read("reference-frontier-r2-analysis.json");
const beam = await read("beam-analysis-r2.json");

const wb = Workbook.create();
const C = { navy: "#17365D", blue: "#2F75B5", pale: "#D9EAF7", green: "#E2F0D9", amber: "#FFF2CC", red: "#FCE4D6", gray: "#E7E6E6", white: "#FFFFFF" };
const font = "Arial";

function sheet(name, color) {
  const s = wb.worksheets.add(name);
  s.tabColor = color;
  s.showGridLines = false;
  return s;
}

function title(s, text, lastCol = "H") {
  s.getRange(`A2:${lastCol}2`).merge();
  s.getRange("A2").values = [[text]];
  s.getRange("A2").format = { font: { name: font, size: 15, bold: true, color: C.navy }, verticalAlignment: "center" };
  s.getRange(`A3:${lastCol}3`).format.borders = { bottom: { style: "thin", color: C.blue } };
}

function table(s, startRow, headers, rows, name) {
  const r = s.getRangeByIndexes(startRow - 1, 0, rows.length + 1, headers.length);
  r.values = [headers, ...rows];
  r.format.font = { name: font, size: 10 };
  r.format.verticalAlignment = "center";
  const h = s.getRangeByIndexes(startRow - 1, 0, 1, headers.length);
  h.format = { fill: C.blue, font: { name: font, size: 10, bold: true, color: C.white }, wrapText: true, horizontalAlignment: "center", verticalAlignment: "center" };
  h.format.rowHeight = 32;
  r.format.borders = { insideHorizontal: { style: "thin", color: "#D9E2F3" }, bottom: { style: "thin", color: "#D9E2F3" } };
  if (rows.length) {
    const t = s.tables.add(r, true, name);
    t.style = "TableStyleMedium2";
  }
  r.format.autofitColumns();
  r.format.autofitRows();
  return r;
}

function stat(x, key = "mean") { return x?.[key] ?? null; }
function algLabel(x) { return ({ heft: "HEFT", "prism-time": "PRISM Time", "prism-cost": "PRISM Cost" })[x] ?? x; }
function environmentLabel(scopeId) {
  return String(scopeId ?? "")
    .replace(/^scheduler-/, "")
    .replace(/-scope-v\d+$/, "")
    .replaceAll("_", " ");
}

const resumo = sheet("Resumo", C.navy);
title(resumo, "Experimentos simulados PRISM × HEFT", "J");
resumo.getRange("A5:B11").values = [
  ["Conjunto", "Execuções concluídas"], ["Baseline", 147], ["SLA", 441], ["Interferência", 90], ["Fronteira", 200], ["Beam", 32], ["Total", 910],
];
resumo.getRange("A5:B5").format = { fill: C.blue, font: { name: font, bold: true, color: C.white } };
resumo.getRange("A5:B11").format.borders = { preset: "all", style: "thin", color: "#D9E2F3" };
resumo.getRange("D5:G9").values = [
  ["Resultado", "HEFT", "PRISM Cost", "PRISM Time"],
  ["Vitórias makespan baseline", baseline.winnerCounts.makespan.heft, baseline.winnerCounts.makespan["prism-cost"], baseline.winnerCounts.makespan["prism-time"]],
  ["Vitórias custo baseline", baseline.winnerCounts.cost.heft, baseline.winnerCounts.cost["prism-cost"], baseline.winnerCounts.cost["prism-time"]],
  ["SLA atendido (147)", sla.winnerCounts.sla.heft, sla.winnerCounts.sla["prism-cost"], sla.winnerCounts.sla["prism-time"]],
  ["Beam selecionado", "n.a.", beam.selectedBeams["prism-cost"], beam.selectedBeams["prism-time"]],
];
resumo.getRange("D5:G5").format = { fill: C.blue, font: { name: font, bold: true, color: C.white } };
resumo.getRange("D5:G9").format.borders = { preset: "all", style: "thin", color: "#D9E2F3" };
resumo.getRange("A14:J19").values = [
  ["Conclusão", "Evidência"],
  ["PRISM Time foi o melhor para makespan", "36,5/49 vitórias na baseline e 107,5/147 no experimento com SLA."],
  ["PRISM Cost foi o melhor para custo", "33,67/49 vitórias na baseline e 95/147 no experimento com SLA."],
  ["PRISM atendeu mais SLAs", "127/147 para cada variante do PRISM, contra 96/147 do HEFT."],
  ["Beam 4 atende a regra para PRISM Time", "Perda mediana 0,83% e pior perda 1,62% nos dois Montages testados."],
  ["Beam 1 do PRISM Cost exige ressalva", "A referência de custo igual a zero torna o gap percentual degenerado; não sustenta recomendação universal."],
];
resumo.getRange("A14:B14").format = { fill: C.blue, font: { name: font, bold: true, color: C.white } };
resumo.getRange("A14:B19").format.wrapText = true;
resumo.getRange("A14:B19").format.borders = { preset: "all", style: "thin", color: "#D9E2F3" };
const c1 = resumo.charts.add("column", resumo.getRange("D5:G8"));
c1.title = "Créditos de vitória e atendimento ao SLA";
c1.hasLegend = true;
c1.setPosition("D11", "J28");
resumo.getRange("A:J").format.font = { name: font, size: 10 };
resumo.getRange("A:A").format.columnWidth = 30;
resumo.getRange("B:B").format.columnWidth = 62;
resumo.getRange("D:G").format.columnWidth = 17;

const bsl = sheet("Baseline SLA", C.blue);
title(bsl, "Baseline e restrições de SLA", "L");
const algRows = [];
for (const [exp, obj] of [["Baseline", baseline], ["SLA", sla]]) {
  for (const a of obj.algorithmSummaries) algRows.push([
    exp, algLabel(a.algorithm), a.runCount, a.completedCount, a.failedCount, a.slaSatisfiedCount,
    stat(a.planningElapsedSeconds, "median"), stat(a.observedMakespanSeconds), stat(a.observedMakespanSeconds, "median"),
    stat(a.observedCost), stat(a.observedCost, "median"), stat(a.predictionErrorMakespan, "median"), stat(a.predictionErrorCost, "median"),
    a.makespanWinnerCredits, a.costWinnerCredits,
  ]);
}
table(bsl, 5, ["experimento","algoritmo","runs","concluídos","falhas","SLA atendido","planejamento mediano (s)","makespan médio (s)","makespan mediano (s)","custo médio","custo mediano","erro makespan mediano","erro custo mediano","créditos makespan","créditos custo"], algRows, "BaselineSlaTable");
bsl.getRange(`H6:I${algRows.length + 5}`).setNumberFormat("#,##0.00");
bsl.getRange(`J6:K${algRows.length + 5}`).setNumberFormat("0.0000");
bsl.getRange(`L6:M${algRows.length + 5}`).setNumberFormat("0.00%");
bsl.freezePanes.freezeRows(5);

const env = sheet("Por Ambiente", "#5B9BD5");
title(env, "Planejado versus executado por workflow e ambiente", "Q");
env.getRange("A4:Q4").merge();
env.getRange("A4").values = [["Baseline sem SLA. Cada gráfico compara o mesmo workflow nos sete ambientes e nos três algoritmos."]];
env.getRange("A4").format = { font: { name: font, size: 10, italic: true, color: "#595959" } };
const workflowIds = [...new Set(baseline.runRows.map(x => x.workflowVersionId))].sort();
let envRow = 6;
for (let workflowIndex = 0; workflowIndex < workflowIds.length; workflowIndex += 1) {
  const workflowId = workflowIds[workflowIndex];
  const workflowRuns = baseline.runRows
    .filter(x => x.workflowVersionId === workflowId)
    .sort((a, b) => `${environmentLabel(a.executionScopeId)}-${algLabel(a.algorithm)}`.localeCompare(`${environmentLabel(b.executionScopeId)}-${algLabel(b.algorithm)}`));
  env.getRange(`A${envRow}:Q${envRow}`).merge();
  env.getRange(`A${envRow}`).values = [[workflowId]];
  env.getRange(`A${envRow}`).format = { fill: C.navy, font: { name: font, size: 11, bold: true, color: C.white } };
  const rows = workflowRuns.map(x => [
    environmentLabel(x.executionScopeId),
    algLabel(x.algorithm),
    `${environmentLabel(x.executionScopeId)} · ${algLabel(x.algorithm)}`,
    x.predicted?.makespanSeconds ?? null,
    x.observedMakespanSeconds,
    x.predicted?.cost ?? null,
    x.observedCost,
    x.predictionErrorMakespan,
    x.predictionErrorCost,
  ]);
  table(env, envRow + 2, ["ambiente","algoritmo","série","makespan planejado (s)","makespan executado (s)","custo planejado","custo executado","erro makespan","erro custo"], rows, `EnvironmentWorkflow${workflowIndex + 1}`);
  env.getRange(`D${envRow + 3}:E${envRow + 23}`).setNumberFormat("#,##0.00");
  env.getRange(`F${envRow + 3}:G${envRow + 23}`).setNumberFormat("0.0000");
  env.getRange(`H${envRow + 3}:I${envRow + 23}`).setNumberFormat("0.00%");
  const makespanChart = env.charts.add("column", env.getRange(`C${envRow + 2}:E${envRow + 23}`));
  makespanChart.title = `${workflowId}: makespan planejado × executado`;
  makespanChart.hasLegend = true;
  makespanChart.setPosition(`K${envRow + 2}`, `Q${envRow + 13}`);
  const costChart = env.charts.add("column", env.getRange(`C${envRow + 2}:C${envRow + 23}`));
  costChart.delete();
  const costHelperStart = envRow + 26;
  env.getRange(`K${costHelperStart}:M${costHelperStart + 21}`).values = [
    ["série", "custo planejado", "custo executado"],
    ...rows.map(row => [row[2], row[5], row[6]]),
  ];
  const costComparisonChart = env.charts.add("column", env.getRange(`K${costHelperStart}:M${costHelperStart + 21}`));
  costComparisonChart.title = `${workflowId}: custo planejado × executado`;
  costComparisonChart.hasLegend = true;
  costComparisonChart.setPosition(`K${envRow + 14}`, `Q${envRow + 25}`);
  envRow += 50;
}
env.getRange("A:A").format.columnWidth = 29;
env.getRange("B:B").format.columnWidth = 16;
env.getRange("C:C").format.columnWidth = 47;
env.getRange("D:I").format.columnWidth = 20;
env.getRange("K:M").format.columnWidth = 22;
env.freezePanes.freezeRows(4);

const intf = sheet("Interferencia", "#ED7D31");
title(intf, "Interferência por cobertura e algoritmo", "N");
const intRows = interference.interferenceSummaries.map(x => [
  algLabel(x.algorithm), x.coveragePercent, x.runCount, x.seedCount, x.slaSatisfiedCount, x.slaSatisfiedRate,
  stat(x.planningElapsedSeconds), stat(x.observedMakespanSeconds), stat(x.observedMakespanSeconds, "stdev"),
  stat(x.observedCost), stat(x.observedInterferenceSeconds), stat(x.makespanDegradationPercent), stat(x.costDegradationPercent), x.failedCount,
]);
table(intf, 5, ["algoritmo","cobertura (%)","runs","seeds","SLA atendido","taxa SLA","planejamento médio (s)","makespan médio (s)","desvio makespan (s)","custo médio","interferência média (s)","degradação makespan (%)","degradação custo (%)","falhas"], intRows, "InterferenceTable");
intf.getRange(`F6:F${intRows.length + 5}`).setNumberFormat("0.0%");
intf.getRange(`L6:M${intRows.length + 5}`).setNumberFormat("0.000");
const ci = intf.charts.add("line", intf.getRange(`A5:B${intRows.length + 5}`));
ci.delete();
const helperStart = intRows.length + 9;
const coverages = [0,10,20,50,80,100];
const helper = [["Cobertura", "HEFT", "PRISM Cost", "PRISM Time"]];
for (const cov of coverages) helper.push([cov, ...["heft","prism-cost","prism-time"].map(a => stat(interference.interferenceSummaries.find(x => x.algorithm === a && x.coveragePercent === cov)?.makespanDegradationPercent))]);
intf.getRangeByIndexes(helperStart - 1, 0, helper.length, 4).values = helper;
intf.getRangeByIndexes(helperStart - 1, 0, 1, 4).format = { fill: C.blue, font: { name: font, bold: true, color: C.white } };
intf.getRange(`B${helperStart + 1}:D${helperStart + 6}`).setNumberFormat("0.000");
const c2 = intf.charts.add("line", intf.getRange(`A${helperStart}:D${helperStart + 6}`));
c2.title = "Degradação média do makespan por cobertura";
c2.hasLegend = true;
c2.setPosition(`F${helperStart}`, `N${helperStart + 18}`);
intf.freezePanes.freezeRows(5);

const fr = sheet("Fronteira", "#70AD47");
title(fr, "Qualidade da fronteira de referência", "N");
const frRows = frontier.metrics.map(x => [x.workflowVersionId, algLabel(x.algorithm), x.evaluatedCount, x.nondominatedCount, x.bestMakespanSeconds, x.bestCost, x.bestMakespanGapPercent, x.bestCostGapPercent, x.hypervolume, x.hypervolumeRatio, x.multiplicativeEpsilon, x.meanDistanceToReference, x.maximumDistanceToReference, x.frontierSpan, x.frontierSpacing]);
table(fr, 5, ["workflow","algoritmo","avaliados","não dominados","melhor makespan (s)","melhor custo","gap makespan (%)","gap custo (%)","hypervolume","razão HV","epsilon multiplicativo","distância média","distância máxima","amplitude","espaçamento"], frRows, "FrontierTable");
fr.getRange(`J6:J${frRows.length + 5}`).setNumberFormat("0.000");
fr.getRange("A12:B15").values = [["Limitação","Interpretação"],["Montage 58","A referência é a união avaliada; não há certificado de ótimo global."],["Montage 6448","A referência é a melhor fronteira conhecida entre as execuções."],["Custo zero","Gap percentual e epsilon podem ficar indefinidos ou numericamente extremos."]];
fr.getRange("A12:B12").format = { fill: C.blue, font: { name: font, bold: true, color: C.white } };
fr.getRange("A12:B15").format.wrapText = true;
fr.getRange("A:A").format.columnWidth = 24; fr.getRange("B:B").format.columnWidth = 65;

const bm = sheet("Beam", "#A5A5A5");
title(bm, "Calibração do beam", "L");
const bmRows = beam.beamSummaries.map(x => [algLabel(x.algorithm), x.beamWidth, x.workflowCount, x.medianObjectiveGapPercent, x.worstObjectiveGapPercent, stat(x.planningElapsedSeconds), stat(x.planningElapsedSeconds,"median"), stat(x.planningElapsedSeconds,"maximum"), x.qualifiesByRule, x.selectedByRule]);
table(bm, 5, ["algoritmo","beam","workflows","gap mediano (%)","pior gap (%)","planejamento médio (s)","planejamento mediano (s)","planejamento máximo (s)","qualifica regra","selecionado"], bmRows, "BeamTable");
bm.getRange("A24:B27").values = [["Regra","Menor beam com perda mediana < 2% e pior perda < 5%."],["PRISM Time",`Beam ${beam.selectedBeams["prism-time"]}: resultado válido nos dois workflows testados.`],["PRISM Cost",`Beam ${beam.selectedBeams["prism-cost"]}: seleção formal afetada por custo de referência zero.`],["Escopo","Montage 58 e Montage 6448; não extrapolar automaticamente para outros workflows."]];
bm.getRange("A24:B27").format.wrapText = true;
bm.getRange("A:A").format.columnWidth = 18;
bm.getRange("B:C").format.columnWidth = 32;
bm.getRange("A24:A27").format.font = { name: font, bold: true };
bm.getRange("A24:B27").format.rowHeight = 42;
const cb = bm.charts.add("line", bm.getRange(`A5:H${bmRows.length + 5}`));
cb.delete();
const bh = 30;
bm.getRange(`A${bh}:C${bh + bmRows.length}`).values = [["beam","PRISM Time gap mediano (%)","PRISM Time planejamento médio (s)"], ...beam.beamSummaries.filter(x => x.algorithm === "prism-time").map(x => [x.beamWidth,x.medianObjectiveGapPercent,stat(x.planningElapsedSeconds)])];
const c3 = bm.charts.add("line", bm.getRange(`A${bh}:C${bh + 8}`));
c3.title = "PRISM Time: qualidade e tempo por beam";
c3.hasLegend = true;
c3.setPosition(`E${bh}`, `L${bh + 17}`);
bm.freezePanes.freezeRows(5);

const runs = sheet("Execucoes", "#8064A2");
title(runs, "Execuções simuladas consolidadas", "Q");
const allRuns = [
  ...baseline.runRows.map(x => ["baseline", x]), ...sla.runRows.map(x => ["sla", x]),
  ...interference.runRows.map(x => ["interferência", x]), ...beam.runRows.map(x => ["beam", x]),
];
const runRows = allRuns.map(([exp,x]) => [exp,x.executionRunId,x.workflowVersionId,x.executionScopeId,environmentLabel(x.executionScopeId),algLabel(x.algorithm),x.status,x.seed,x.coveragePercent,x.beamWidth,x.planningElapsedSeconds,x.planningCandidateCount,x.predicted?.makespanSeconds,x.observedMakespanSeconds,x.predicted?.cost,x.observedCost,x.predictionErrorMakespan,x.predictionErrorCost,x.slaSatisfied,x.failureReason]);
table(runs, 5, ["experimento","run_id","workflow","scope","ambiente","algoritmo","status","seed","cobertura (%)","beam","planejamento (s)","candidatos","makespan previsto (s)","makespan observado (s)","custo previsto","custo observado","erro makespan","erro custo","SLA atendido","falha"], runRows, "RunsTable");
runs.getRange(`Q6:R${runRows.length + 5}`).setNumberFormat("0.00%");
runs.freezePanes.freezeRows(5); runs.freezePanes.freezeColumns(6);
runs.getRange("A:A").format.columnWidth = 16; runs.getRange("B:D").format.columnWidth = 34; runs.getRange("E:E").format.columnWidth = 28; runs.getRange("T:T").format.columnWidth = 28;

const proto = sheet("Protocolo", "#7F8C8D");
title(proto, "Protocolo, fontes e limitações", "F");
const protocolRows = [
  ["Modo", "simulation", "Nenhuma execução real foi iniciada."],
  ["Baseline", "49 sessões; 147 simulações", "7 workflows × 7 ambientes × 3 algoritmos; beam 20."],
  ["SLA", "147 sessões; 441 simulações", "Fatores 1,1; 1,2; 1,5 sobre referência."],
  ["Interferência", "30 sessões; 90 simulações", "Montage 6448; 5 seeds; coberturas 0,10,20,50,80,100; slowdown 1,5."],
  ["Fronteira", "2 sessões; 200 simulações", "Referência empírica, sem certificado de ótimo global."],
  ["Beam", "16 sessões; 32 simulações", "Beams 1,4,8,16,20,32,64,120; Montage 58 e 6448."],
  ["Fonte baseline", "baseline-analysis.json", "Agregados e linhas por execução."],
  ["Fonte SLA", "sla-analysis.json", "Agregados e linhas por execução."],
  ["Fonte interferência", "interference-analysis-r2.json", "Campanha r2 é a autoridade final."],
  ["Fonte fronteira", "reference-frontier-r2-analysis.json", "Métricas de referência."],
  ["Fonte beam", "beam-analysis-r2.json", "Qualidade e duração por largura."],
  ["Dados brutos", "*-simulation-*-results.json.gz", "Atividades, transferências e eventos preservados fora do Excel."],
  ["Ressalva", "Custo zero", "Razões e gaps relativos ficam degenerados; usar métricas absolutas em conjunto."],
];
table(proto, 5, ["Campo","Valor","Observação"], protocolRows, "ProtocolTable");
proto.getRange("A:A").format.columnWidth = 24; proto.getRange("B:B").format.columnWidth = 38; proto.getRange("C:C").format.columnWidth = 72;

wb.recalculate();
const checks = [];
for (const [name, range] of [["Resumo","A1:J28"],["Baseline SLA","A1:O12"],["Por Ambiente","A1:Q356"],["Interferencia","A1:N45"],["Fronteira","A1:O15"],["Beam","A1:L48"],["Execucoes","A1:T20"],["Protocolo","A1:C20"]]) {
  checks.push((await wb.inspect({ kind: "table", range: `${name}!${range}`, include: "values,formulas", tableMaxRows: 20, tableMaxCols: 20, maxChars: 6000 })).ndjson);
}
checks.push((await wb.inspect({ kind: "match", searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A|#NUM!|#NULL!|#SPILL!|#CALC!", options: { useRegex: true, maxResults: 300 }, summary: "final formula error scan" })).ndjson);
await fs.writeFile(path.join(out, "experimentos-prism-heft.xlsx.inspect.ndjson"), checks.join("\n"));

const previewDir = path.join(out, "previews-final");
await fs.mkdir(previewDir, { recursive: true });
for (const name of ["Resumo","Baseline SLA","Por Ambiente","Interferencia","Fronteira","Beam","Execucoes","Protocolo"]) {
  const p = await wb.render({ sheetName: name, autoCrop: "all", scale: 1, format: "png" });
  await fs.writeFile(path.join(previewDir, `${name.replaceAll(" ", "-").toLowerCase()}.png`), new Uint8Array(await p.arrayBuffer()));
}
const output = await SpreadsheetFile.exportXlsx(wb);
await output.save(path.join(out, "experimentos-prism-heft.xlsx"));

const md = `# Relatório dos experimentos PRISM × HEFT\n\n` +
`## Escopo\n\nForam concluídas 910 execuções simuladas: 147 na baseline, 441 com SLA, 90 com interferência, 200 para a fronteira de referência e 32 para calibração do beam. Nenhuma execução real foi iniciada.\n\n` +
`## Resultado principal\n\nPRISM Time dominou a métrica de makespan: recebeu 36,5 de 49 créditos de vitória na baseline e 107,5 de 147 no experimento com SLA. PRISM Cost dominou custo: 33,67 de 49 créditos na baseline e 95 de 147 com SLA. Com restrições, PRISM Time e PRISM Cost atenderam 127 de 147 casos cada, contra 96 do HEFT.\n\n` +
`![Resumo](previews-final/resumo.png)\n\n` +
`## Interferência\n\nO experimento variou cumulativamente as atividades afetadas em 0%, 10%, 20%, 50%, 80% e 100%, com cinco seeds e slowdown 1,5 no Montage 6448. A tabela e o gráfico abaixo preservam média, dispersão, SLA e tempo de interferência para os três algoritmos. A degradação deve ser lida junto com a diferença estrutural dos planos: o HEFT começa com makespan muito maior que o PRISM.\n\n` +
`![Interferência](previews-final/interferencia.png)\n\n` +
`## Fronteira de referência\n\nNo Montage 58, as 100 soluções avaliadas de PRISM Cost formaram a referência empírica observada. No Montage 6448, a união não dominada é apenas a melhor fronteira conhecida. Não há certificado de ótimo global. Quando o custo de referência é zero, gap percentual e epsilon tornam-se indefinidos ou numericamente extremos; por isso, custo absoluto e makespan devem acompanhar essas métricas.\n\n` +
`![Fronteira](previews-final/fronteira.png)\n\n` +
`## Calibração do beam\n\nPara PRISM Time, beam 4 é o menor que atende à regra pré-definida: perda mediana inferior a 2% e pior perda inferior a 5% nos Montages 58 e 6448. A seleção formal de beam 1 para PRISM Cost não deve ser generalizada porque a referência de custo zero produz um gap degenerado. O beam 120 aumenta fortemente o tempo de planejamento no Montage 6448 sem evidência suficiente, nesta amostra, de ganho que justifique usá-lo como padrão.\n\n` +
`![Beam](previews-final/beam.png)\n\n` +
`## Conclusão\n\nPara o conjunto testado, a configuração recomendada é beam 4 para PRISM Time quando a prioridade é reduzir makespan com baixo custo computacional de busca. PRISM Cost deve manter uma calibração separada baseada em custo absoluto ou regret aditivo quando a referência é zero. Beam 20 continua útil como configuração conservadora de comparação, mas não é necessário como padrão universal; beam 120 deve funcionar como teto opcional, acionado somente quando tamanho, diversidade de recursos e estagnação da busca justificarem o orçamento adicional.\n\n` +
`## Reprodutibilidade\n\nOs agregados e as 710 linhas individuais de baseline, SLA, interferência e beam estão na planilha. As 200 amostras de fronteira e os dados completos por atividade, transferência e evento permanecem nos arquivos JSON compactados na mesma pasta, pois excedem o nível de detalhe útil do Excel.\n`;
await fs.writeFile(path.join(out, "relatorio-experimentos-prism-heft.md"), md);
console.log(JSON.stringify({ workbook: path.join(out,"experimentos-prism-heft.xlsx"), report: path.join(out,"relatorio-experimentos-prism-heft.md"), previews: previewDir, runRows: runRows.length }));
