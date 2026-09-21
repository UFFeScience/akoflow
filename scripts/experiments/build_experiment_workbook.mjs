import fs from "node:fs/promises";
import path from "node:path";
import { SpreadsheetFile, Workbook } from "@oai/artifact-tool";

const [snapshotPath, outputPath] = process.argv.slice(2);
if (!snapshotPath || !outputPath) {
  throw new Error("usage: build_experiment_workbook.mjs SNAPSHOT.json OUTPUT.xlsx");
}

const snapshot = JSON.parse(await fs.readFile(snapshotPath, "utf8"));
const workbook = Workbook.create();
const font = "Arial";
const colors = {
  navy: "#17365D",
  blue: "#2F75B5",
  lightBlue: "#D9EAF7",
  green: "#E2F0D9",
  yellow: "#FFF2CC",
  red: "#FCE4D6",
  gray: "#E7E6E6",
  white: "#FFFFFF",
};

function isoDate(value) {
  return value ? new Date(value) : null;
}

function elapsedSeconds(start, finish) {
  if (!start || !finish) return null;
  return (new Date(finish).getTime() - new Date(start).getTime()) / 1000;
}

function addSheet(name, tabColor) {
  const sheet = workbook.worksheets.add(name);
  sheet.tabColor = tabColor;
  sheet.showGridLines = false;
  return sheet;
}

function styleTitle(sheet, range, title) {
  sheet.getRange(range).merge();
  const cell = sheet.getRange(range.split(":")[0]);
  cell.values = [[title]];
  cell.format = {
    fill: colors.navy,
    font: { name: font, size: 16, bold: true, color: colors.white },
    verticalAlignment: "center",
  };
  cell.format.rowHeight = 28;
}

function writeTable(sheet, startRow, headers, rows, tableName) {
  const start = startRow - 1;
  const matrix = [headers, ...rows];
  const range = sheet.getRangeByIndexes(start, 0, matrix.length, headers.length);
  range.values = matrix;
  range.format.font = { name: font, size: 10 };
  const header = sheet.getRangeByIndexes(start, 0, 1, headers.length);
  header.format = {
    fill: colors.blue,
    font: { name: font, size: 10, bold: true, color: colors.white },
    wrapText: true,
    verticalAlignment: "center",
  };
  header.format.rowHeight = 32;
  range.format.borders = { preset: "all", style: "thin", color: "#D9E2F3" };
  if (rows.length > 0) {
    const table = sheet.tables.add(
      sheet.getRangeByIndexes(start, 0, matrix.length, headers.length),
      true,
      tableName,
    );
    table.style = "TableStyleMedium2";
  }
  sheet.freezePanes.freezeRows(startRow);
  range.format.autofitColumns();
  range.format.autofitRows();
  return range;
}

const sessions = [];
const runs = [];
const candidates = [];
for (const record of snapshot.records) {
  const detail = record.session;
  const session = detail.session ?? detail;
  sessions.push([
    session.id,
    session.workflowVersionId,
    session.executionScopeId,
    session.networkTopologyId,
    session.status,
    session.progress,
    session.candidateCount,
    session.deadlineSeconds ?? 0,
    session.budget ?? 0,
    isoDate(session.createdAt),
    isoDate(session.startedAt),
    isoDate(session.completedAt),
    elapsedSeconds(session.startedAt, session.completedAt),
    session.failureReason ?? "",
    snapshot.runtimeRevision,
  ]);
  for (const run of detail.algorithmRuns ?? []) {
    runs.push([
      run.id,
      session.id,
      session.workflowVersionId,
      session.executionScopeId,
      run.algorithm,
      run.objective,
      run.status,
      run.progress,
      run.candidateCount,
      run.configuration?.beamWidth ?? null,
      run.configuration?.optionCount ?? null,
      run.estimate?.durationSeconds ?? null,
      run.estimate?.expandedStates ?? null,
      run.estimate?.activityCount ?? null,
      run.estimate?.dependencyCount ?? null,
      run.estimate?.compatibleResources ?? null,
      run.estimate?.readyBranchLimit ?? null,
      run.estimate?.confidence ?? "",
      isoDate(run.startedAt),
      isoDate(run.completedAt),
      elapsedSeconds(run.startedAt, run.completedAt),
      run.failureReason ?? "",
    ]);
  }
  for (const candidate of record.candidates ?? []) {
    candidates.push([
      candidate.id,
      session.id,
      session.workflowVersionId,
      session.executionScopeId,
      candidate.algorithm,
      candidate.objective,
      candidate.rank,
      Boolean(candidate.paretoOptimal),
      Boolean(candidate.dominated),
      Boolean(candidate.feasible),
      candidate.predicted?.makespanSeconds ?? null,
      candidate.predicted?.cost ?? null,
      candidate.plan?.assignmentCount ?? null,
      candidate.fingerprint,
      isoDate(candidate.createdAt),
    ]);
  }
}

const resumo = addSheet("Resumo", colors.navy);
const protocolo = addSheet("Protocolo", colors.blue);
const sessoes = addSheet("Sessoes", colors.blue);
const algoritmos = addSheet("Algoritmos", colors.blue);
const candidatos = addSheet("Candidatos", colors.blue);
const sla = addSheet("SLA", "#70AD47");
const simulacoes = addSheet("Simulacoes", "#70AD47");
const interferencia = addSheet("Interferencia", "#ED7D31");
const beam = addSheet("Beam", "#A5A5A5");

styleTitle(resumo, "A1:H1", "Experimentos PRISM × HEFT — acompanhamento");
resumo.getRange("A3:B9").values = [
  ["Indicador", "Valor"],
  ["Campanha", snapshot.campaignPrefix],
  ["Capturado em", isoDate(snapshot.capturedAt)],
  ["Sessões esperadas", 49],
  ["Sessões coletadas", sessions.length],
  ["Execuções de algoritmo", runs.length],
  ["Candidatos", candidates.length],
];
resumo.getRange("B7").formulas = [["=COUNTA(Sessoes!$A$4:$A$52)"]];
resumo.getRange("B8").formulas = [["=COUNTA(Algoritmos!$A$4:$A$150)"]];
resumo.getRange("B9").formulas = [[`=COUNTA(Candidatos!$A$4:$A$${candidates.length + 3})`]];
resumo.getRange("A3:B3").format = {
  fill: colors.blue,
  font: { name: font, bold: true, color: colors.white },
};
resumo.getRange("A3:B9").format.borders = { preset: "all", style: "thin", color: "#D9E2F3" };
resumo.getRange("B5").setNumberFormat("yyyy-mm-dd hh:mm:ss");
resumo.getRange("A12:B16").values = [
  ["Status", "Quantidade"],
  ["completed", sessions.filter((row) => row[4] === "completed").length],
  ["running", sessions.filter((row) => row[4] === "running").length],
  ["queued", sessions.filter((row) => row[4] === "queued").length],
  ["failed", sessions.filter((row) => row[4] === "failed").length],
];
resumo.getRange("B13").formulas = [["=COUNTIF(Sessoes!$E$4:$E$52,A13)"]];
resumo.getRange("B13:B16").fillDown();
resumo.getRange("A12:B12").format = {
  fill: colors.blue,
  font: { name: font, bold: true, color: colors.white },
};
const statusChart = resumo.charts.add("bar", resumo.getRange("A12:B16"));
statusChart.title = "Situação da baseline";
statusChart.titleTextStyle.typeface = font;
statusChart.hasLegend = false;
statusChart.setPosition("D3", "H16");
resumo.getRange("A19:H24").values = [
  ["Etapa", "Situação", "Configuração", "Unidade observacional", "Variável principal", "Saída", "Execução real?", "Notas"],
  ["1A Baseline", "em execução", "beam 20; sem SLA", "workflow × ambiente × algoritmo", "algoritmo/ambiente", "planejamento + simulação", false, "7 workflows × 7 ambientes × 3 algoritmos"],
  ["1B SLA", "pendente", "D_W/B_W comuns por workflow", "workflow × ambiente × algoritmo", "ambiente", "viabilidade, makespan e custo", false, "sensibilidade 1.1/1.2/1.5"],
  ["2 Interferência", "modelo implementado", "λ=0..2; híbrido heterogêneo", "workflow × λ × planejador", "λ e conhecimento", "slowdown observado", false, "PRISM informado/desinformado e HEFT"],
  ["3 Qualidade", "pendente", "fronteira comum", "candidato", "beam/algoritmo", "HV, epsilon, gaps e diversidade", false, "ótimo apenas quando certificado"],
  ["4 Beam", "pendente", "1,4,8,16,20,32,64,120", "workflow × beam × objetivo", "beam", "tempo, memória e qualidade", false, "Montage substitui CyberShake"],
];
resumo.getRange("A19:H19").format = {
  fill: colors.blue,
  font: { name: font, bold: true, color: colors.white },
  wrapText: true,
};
resumo.getRange("A19:H24").format.borders = { preset: "all", style: "thin", color: "#D9E2F3" };
resumo.getRange("A1:H24").format.font = { name: font, size: 10 };
resumo.getRange("A1:H24").format.autofitColumns();
resumo.getRange("A:A").format.columnWidth = 21;
resumo.getRange("C:H").format.columnWidth = 22;

styleTitle(protocolo, "A1:F1", "Protocolo e proveniência");
writeTable(
  protocolo,
  3,
  ["Campo", "Valor", "Unidade", "Fixo/variável", "Fonte", "Observação"],
  [
    ["Campanha baseline", snapshot.campaignPrefix, "id", "fixo", "AkôFlow API", "Sem deadline e sem budget"],
    ["Git do runtime baseline", snapshot.runtimeRevision, "commit", "fixo", "runtime implantado", "Código que executou os planejamentos"],
    ["Git do coletor", snapshot.collectorRevision, "commit", "fixo", "git", "Código que gerou este workbook"],
    ["Beam baseline", 20, "estados", "fixo", "configuração da sessão", "Confirmar largura efetiva nas métricas"],
    ["optionCount", 25, "candidatos", "fixo", "configuração da sessão", "PRISM Time e Cost"],
    ["Algoritmos", "HEFT; PRISM Time; PRISM Cost", "categoria", "variável", "sessões", "Mesmo simulador para comparação final"],
    ["Ambientes", 7, "ambientes", "variável", "escopos", "Inclui fog/HPC/cloud"],
    ["SLA sugerido", "1.2 × referência HEFT", "fator", "variável controlada", "protocolo", "Sensibilidade 1.1/1.2/1.5"],
    ["Interferência", "Iλ=1+λ(Ibase−1)", "slowdown", "variável", "protocolo", "λ=0,0.25,0.5,1,1.5,2"],
    ["Execução real", false, "booleano", "fora do escopo", "solicitação do usuário", "Somente simulation"],
    ["Documento-base", "/Users/ovvesley/.codex/attachments/9c94c544-fbc1-44c6-9c16-3da081d3fdaa/pasted-text-1.txt", "arquivo", "fixo", "anexo", "Especificação científica"],
  ],
  "ProtocoloTable",
);
protocolo.getRange("B4:B13").format.wrapText = true;
protocolo.getRange("E:F").format.columnWidth = 28;

styleTitle(sessoes, "A1:O1", "Baseline — sessões de planejamento");
writeTable(
  sessoes,
  3,
  ["session_id", "workflow_version_id", "execution_scope_id", "network_topology_id", "status", "progress", "candidate_count", "deadline_s", "budget", "created_at", "started_at", "completed_at", "planning_elapsed_s", "failure_reason", "git_revision"],
  sessions,
  "SessionsTable",
);
sessoes.getRange(`F4:F${sessions.length + 3}`).setNumberFormat("0.0%");
sessoes.getRange(`J4:L${sessions.length + 3}`).setNumberFormat("yyyy-mm-dd hh:mm:ss");
sessoes.getRange("A:D").format.columnWidth = 31;
sessoes.getRange("N:O").format.columnWidth = 28;

styleTitle(algoritmos, "A1:V1", "Baseline — execuções dos algoritmos");
writeTable(
  algoritmos,
  3,
  ["algorithm_run_id", "session_id", "workflow_version_id", "execution_scope_id", "algorithm", "objective", "status", "progress", "candidate_count", "beam_width", "option_count", "estimated_duration_s", "estimated_expanded_states", "activity_count", "dependency_count", "compatible_resources", "ready_branch_limit", "estimate_confidence", "started_at", "completed_at", "planning_elapsed_s", "failure_reason"],
  runs,
  "AlgorithmRunsTable",
);
algoritmos.getRange(`H4:H${runs.length + 3}`).setNumberFormat("0.0%");
algoritmos.getRange(`S4:T${runs.length + 3}`).setNumberFormat("yyyy-mm-dd hh:mm:ss");
algoritmos.getRange("A:D").format.columnWidth = 31;

styleTitle(candidatos, "A1:O1", "Baseline — candidatos produzidos");
writeTable(
  candidatos,
  3,
  ["candidate_id", "session_id", "workflow_version_id", "execution_scope_id", "algorithm", "objective", "rank", "pareto_optimal", "dominated", "feasible", "predicted_makespan_s", "predicted_cost", "assignment_count", "fingerprint", "created_at"],
  candidates,
  "CandidatesTable",
);
candidatos.getRange(`K4:L${candidates.length + 3}`).setNumberFormat("0.0000");
candidatos.getRange(`O4:O${candidates.length + 3}`).setNumberFormat("yyyy-mm-dd hh:mm:ss");
candidatos.getRange("A:D").format.columnWidth = 31;
candidatos.getRange("N:N").format.columnWidth = 28;

styleTitle(sla, "A1:L1", "Experimento 1B — definição e resultados de SLA");
writeTable(
  sla,
  3,
  ["workflow_version_id", "reference_scope_id", "heft_reference_makespan_s", "reference_cost", "sla_factor", "deadline_s", "budget", "environment_scope_id", "algorithm", "executed_makespan_s", "executed_cost", "feasible"],
  [],
  "SLATable",
);
sla.getRange("A5:L7").values = [
  ["Uso", "Preencher automaticamente após a baseline simulada; uma linha por workflow × fator × ambiente × algoritmo.", null, null, null, null, null, null, null, null, null, null],
  ["Fatores", "1.1, 1.2 e 1.5", null, null, null, null, null, null, null, null, null, null],
  ["Referência", "HEFT no ambiente híbrido heterogêneo, avaliado pelo simulador comum.", null, null, null, null, null, null, null, null, null, null],
];

styleTitle(simulacoes, "A1:R1", "Execuções simuladas — métricas observadas");
writeTable(
  simulacoes,
  3,
  ["experiment", "execution_run_id", "schedule_plan_id", "session_id", "workflow_version_id", "scope_id", "algorithm", "mode", "seed", "status", "predicted_makespan_s", "predicted_cost", "observed_makespan_s", "observed_cost", "prediction_error_makespan_pct", "prediction_error_cost_pct", "failure_reason", "captured_at"],
  [],
  "SimulationsTable",
);
simulacoes.getRange("A5:R6").values = [
  ["baseline", "Aguardando conclusão do planejamento", null, null, null, null, null, "simulation", null, "pending", null, null, null, null, null, null, null, isoDate(snapshot.capturedAt)],
  ["regra", "147 execuções: 49 sessões × 3 algoritmos", null, null, null, null, null, "simulation", null, "pending", null, null, null, null, null, null, null, isoDate(snapshot.capturedAt)],
];
simulacoes.getRange("R5:R6").setNumberFormat("yyyy-mm-dd hh:mm:ss");
simulacoes.getRange("A:B").format.columnWidth = 32;
simulacoes.getRange("C:Q").format.columnWidth = 18;

styleTitle(interferencia, "A1:P1", "Experimento 2 — interferência pairwise slowdown");
writeTable(
  interferencia,
  3,
  ["workflow_version_id", "scope_id", "lambda", "planner", "interference_aware", "execution_interference", "base_slowdown", "effective_slowdown", "beam_width", "deadline_s", "budget", "planning_elapsed_s", "observed_makespan_s", "observed_cost", "interference_s", "feasible"],
  [],
  "InterferenceTable",
);
interferencia.getRange("A5:P8").values = [
  ["protocolo", "scheduler-hybrid_hetero-scope-v1", 0, "PRISM informado", true, true, null, 1, 20, null, null, null, null, null, null, null],
  ["protocolo", "scheduler-hybrid_hetero-scope-v1", 0.25, "PRISM desinformado", false, true, null, null, 20, null, null, null, null, null, null, null],
  ["protocolo", "scheduler-hybrid_hetero-scope-v1", 1, "HEFT", false, true, null, null, null, null, null, null, null, null, null, null],
  ["modelo", "pairwise-slowdown schema v2", null, "agregação maximum", null, null, null, null, null, null, null, null, null, null, null, null],
];
interferencia.getRange("A:B").format.columnWidth = 28;
interferencia.getRange("C:P").format.columnWidth = 18;
interferencia.getRange("A5:P8").format.wrapText = true;

styleTitle(beam, "A1:R1", "Experimento 4 — calibração de beam");
writeTable(
  beam,
  3,
  ["workflow_version_id", "scope_id", "algorithm", "beam_width", "option_count", "ready_branch_limit", "planning_elapsed_s", "peak_memory_mb", "expanded_states", "unique_states", "time_first_feasible_s", "time_best_s", "best_makespan_s", "best_cost", "gap_time_pct", "gap_cost_pct", "marginal_gain_pct", "selected_by_rule"],
  [],
  "BeamTable",
);
beam.getRange("A5:R7").values = [
  ["montage-58-v1", "scheduler-hybrid_hetero-scope-v1", "prism-time/prism-cost", "1,4,8,16,20,32,64,120", 25, 3, null, null, null, null, null, null, null, null, null, null, null, null],
  ["wfcommons-montage-6448-v1", "scheduler-hybrid_hetero-scope-v1", "prism-time/prism-cost", "1,4,8,16,20,32,64,120", 25, 3, null, null, null, null, null, null, null, null, null, null, null, null],
  ["Regra", "menor beam com perda mediana <2% e pior perda <5%", null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null],
];
beam.getRange("A:B").format.columnWidth = 28;
beam.getRange("C:R").format.columnWidth = 18;
beam.getRange("A5:R7").format.wrapText = true;

for (const sheet of [protocolo, sessoes, algoritmos, candidatos, sla, simulacoes, interferencia, beam]) {
  const used = sheet.getUsedRange();
  if (used) {
    used.format.font = { name: font, size: 10 };
    used.format.verticalAlignment = "center";
  }
}

workbook.recalculate();
const errors = await workbook.inspect({
  kind: "match",
  searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A|#NUM!|#NULL!|#SPILL!|#CALC!",
  options: { useRegex: true, maxResults: 300 },
  summary: "final formula error scan",
});
console.log(errors.ndjson);

const outputDir = path.dirname(outputPath);
await fs.mkdir(outputDir, { recursive: true });
const previewDir = path.join(outputDir, "previews");
await fs.mkdir(previewDir, { recursive: true });
for (const [name, range] of [
  ["Resumo", "A1:H24"],
  ["Protocolo", "A1:F14"],
  ["Sessoes", "A1:O25"],
  ["Algoritmos", "A1:V25"],
  ["Candidatos", "A1:O25"],
  ["SLA", "A1:L8"],
  ["Simulacoes", "A1:R7"],
  ["Interferencia", "A1:P9"],
  ["Beam", "A1:R8"],
]) {
  const preview = await workbook.render({ sheetName: name, range, scale: 1, format: "png" });
  await fs.writeFile(
    path.join(previewDir, `${name}.png`),
    new Uint8Array(await preview.arrayBuffer()),
  );
}
const output = await SpreadsheetFile.exportXlsx(workbook);
await output.save(outputPath);
