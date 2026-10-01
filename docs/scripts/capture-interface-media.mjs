import { spawn } from "node:child_process";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const docsDirectory = path.resolve(scriptDirectory, "..");
const chromeExecutable =
  process.env.CHROME_BIN ||
  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
const debuggingPort = 9227;
const profileDirectory = await mkdtemp(
  path.join(os.tmpdir(), "akoflow-docs-chrome-"),
);

async function captureToken() {
  if (process.env.AKOFLOW_CAPTURE_TOKEN)
    return process.env.AKOFLOW_CAPTURE_TOKEN;
  try {
    const environment = await readFile(
      path.resolve(docsDirectory, "../.env"),
      "utf8",
    );
    return environment
      .match(/^AKOFLOW_API_TOKEN=(.+)$/m)?.[1]
      ?.trim()
      .replace(/^['"]|['"]$/g, "");
  } catch {
    return undefined;
  }
}

const apiToken = await captureToken();

const desktopBase = (process.env.AKOFLOW_DESKTOP_URL || "http://127.0.0.1:5173").replace(/\/$/, "");
const cloudOperations = await fetch(`${desktopBase}/akoflow-api/cloud-operations/`)
  .then((response) => response.ok ? response.json() : [])
  .catch(() => []);
const cloudOperation = cloudOperations.find((operation) => operation.environmentId === "documentation-cloud");
const desktopPages = [
  ["environments", "/environments", "infrastructure/environments-current.png"],
  ["environments-simulation", "/environments", "infrastructure/environments-simulation.png", "Simulation infrastructure"],
  ["resources", "/resources", "infrastructure/resources.png"],
  ["execution-scopes", "/execution-scopes", "infrastructure/execution-scopes-current.png"],
  ["machine-configurations", "/machine-configurations", "infrastructure/machine-configurations.png"],
  ["network", "/network", "infrastructure/network.png"],
  ["workflows", "/workflows", "workflows/definitions-current.png"],
  ["plans", "/plans", "planning/plans.png"],
  ["planning-sessions", "/planning-sessions", "planning/sessions.png"],
  ["executions", "/executions", "runs/executions.png"],
  ["new-execution", "/executions/new", "runs/new-execution.png"],
  ["data-catalog", "/data", "data/catalog.png"],
  ["artifacts", "/artifacts", "data/artifacts.png"],
  ["artifact-locations", "/artifact-locations", "data/artifact-locations.png"],
  ["materializations", "/materializations", "data/materializations.png"],
  ["provenance", "/provenance", "data/provenance.png"],
  ["provenance-sql", "/provenance", "data/provenance-sql.png", "SQL"],
  ["provenance-lineage", "/provenance", "data/provenance-lineage.png", "Lineage"],
  ["audit", "/audit", "operations/audit.png"],
  ["console", "/console", "operations/console.png"],
  ["settings", "/settings", "operations/settings.png"],
  ["settings-ssh", "/settings?section=ssh", "operations/settings-ssh.png"],
  ["settings-data", "/settings?section=data", "operations/settings-data.png"],
  ["settings-credits", "/settings?section=credits", "operations/settings-credits.png"],
  ["settings-danger", "/settings?section=reset", "operations/settings-danger.png"],
  ["new-environment", "/environments/new", "infrastructure/new-environment.png"],
  ["new-workflow", "/workflows/new", "workflows/new-workflow.png"],
  ["new-scope", "/execution-scopes/new", "infrastructure/new-scope.png"],
  ["new-topology", "/network/new", "infrastructure/new-topology.png"],
  ["new-artifact", "/artifacts/new", "data/new-artifact.png"],
  ["environment-detail", "/environments/simulation-example", "infrastructure/environment-detail.png"],
  ["environment-inventory", "/environments/simulation-example/inventory", "infrastructure/environment-inventory.png"],
  ["environment-storage", "/environments/simulation-example/storages", "infrastructure/environment-storage.png"],
  ["cloud-environment-detail", "/environments/documentation-cloud", "infrastructure/cloud-environment-detail.png"],
  ["environment-cloud-capacity", "/environments/documentation-cloud/cloud-capacity", "infrastructure/environment-cloud-capacity.png"],
  ["environment-provisioning", "/environments/documentation-cloud/provisioning", "infrastructure/environment-provisioning.png"],
  ["cloud-resource-detail", "/resources/documentation-capacity", "infrastructure/cloud-resource-detail.png"],
  ...(cloudOperation ? [["cloud-operation-detail", `/resources/documentation-capacity/provisioning/${cloudOperation.instanceId}`, "infrastructure/cloud-operation-detail.png"]] : []),
  ...(cloudOperation ? [["cloud-operation-terraform", `/resources/documentation-capacity/provisioning/${cloudOperation.instanceId}`, "infrastructure/cloud-operation-terraform.png", "Provisioning (Terraform)"]] : []),
  ...(cloudOperation ? [["cloud-operation-ansible", `/resources/documentation-capacity/provisioning/${cloudOperation.instanceId}`, "infrastructure/cloud-operation-ansible.png", "Configuration (Ansible)"]] : []),
  ["environment-edit", "/environments/simulation-example/edit", "infrastructure/environment-edit.png"],
  ["resource-detail", "/resources/simulated-edge", "infrastructure/resource-detail.png"],
  ["scope-detail", "/execution-scopes/simulation-example-v1-scope", "infrastructure/scope-detail.png"],
  ["topology-detail", "/network/simulation-network-v1", "infrastructure/topology-detail.png"],
  ["workflow-detail", "/workflows/simulation-example-workflow", "workflows/workflow-detail.png"],
  ["workflow-plans", "/workflows/simulation-example-workflow", "workflows/workflow-plans.png", "Plans"],
  ["workflow-runs", "/workflows/simulation-example-workflow", "workflows/workflow-runs.png", "Runs"],
  ["planning-create", "/workflows/simulation-example-workflow/plans/new", "planning/planning-create.png"],
  ["plan-detail", "/workflows/simulation-example-workflow/plans/simulation-example-plan-v1", "planning/plan-detail.png"],
  ["plan-executions", "/workflows/simulation-example-workflow/plans/simulation-example-plan-v1", "planning/plan-executions.png", "Executions"],
  ["run-detail", "/workflows/simulation-example-workflow/plans/simulation-example-plan-v1/executions/simulation-example-run-v2", "runs/run-detail.png"],
  ["run-activities", "/workflows/simulation-example-workflow/plans/simulation-example-plan-v1/executions/simulation-example-run-v2", "runs/run-activities.png", "Activities"],
  ["run-timeline", "/workflows/simulation-example-workflow/plans/simulation-example-plan-v1/executions/simulation-example-run-v2", "runs/run-timeline.png", "Timeline"],
  ["run-resources", "/workflows/simulation-example-workflow/plans/simulation-example-plan-v1/executions/simulation-example-run-v2", "runs/run-resources.png", "Resources"],
  ["run-plan-comparison", "/workflows/simulation-example-workflow/plans/simulation-example-plan-v1/executions/simulation-example-run-v2", "runs/run-plan-comparison.png", "Plan vs execution"],
  ["run-data", "/workflows/simulation-example-workflow/plans/simulation-example-plan-v1/executions/simulation-example-run-v2", "runs/run-data.png", "Data"],
  ["run-prerequisites", "/workflows/simulation-example-workflow/plans/simulation-example-plan-v1/executions/simulation-example-run-v2", "runs/run-prerequisites.png", "Prerequisites"],
  ["run-events", "/workflows/simulation-example-workflow/plans/simulation-example-plan-v1/executions/simulation-example-run-v2", "runs/run-events.png", "Events"],
  ["activity-detail", "/workflows/simulation-example-workflow/plans/simulation-example-plan-v1/executions/simulation-example-run-v2/activities/simulation-example-workflow-analyze", "runs/activity-detail.png"],
  ["planning-session-detail", "/planning-sessions/planning-simulation-example", "planning/session-detail.png"],
  ["artifact-detail", "/artifacts/busybox", "data/artifact-detail.png"],
];

const captures = [
  ...desktopPages.map(([name, route, output, click]) => ({
    name,
    url: `${desktopBase}${route || "/"}`,
    output: `static/img/interface/${output}`,
    click,
  })),
];
const selectedCaptures = process.env.AKOFLOW_CAPTURE_ONLY
  ? captures.filter((item) => process.env.AKOFLOW_CAPTURE_ONLY.split(",").includes(item.name))
  : captures;

function delay(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

async function waitForDebugger() {
  for (let attempt = 0; attempt < 40; attempt += 1) {
    try {
      const response = await fetch(
        `http://127.0.0.1:${debuggingPort}/json/version`,
      );
      if (response.ok) return;
    } catch {
      // Chrome is still starting.
    }
    await delay(100);
  }
  throw new Error("Chrome DevTools did not become available");
}

async function createTarget(url) {
  const response = await fetch(
    `http://127.0.0.1:${debuggingPort}/json/new?${encodeURIComponent(url)}`,
    { method: "PUT" },
  );
  if (!response.ok)
    throw new Error(`Could not open ${url}: ${response.status}`);
  return response.json();
}

async function capture(target, output, click) {
  const socket = new WebSocket(target.webSocketDebuggerUrl);
  const pending = new Map();
  let sequence = 0;

  await new Promise((resolve, reject) => {
    socket.addEventListener("open", resolve, { once: true });
    socket.addEventListener("error", reject, { once: true });
  });

  function command(method, params = {}) {
    sequence += 1;
    const id = sequence;
    socket.send(JSON.stringify({ id, method, params }));
    return new Promise((resolve, reject) =>
      pending.set(id, { resolve, reject }),
    );
  }

  socket.addEventListener("message", (event) => {
    const message = JSON.parse(event.data);
    if (!message.id || !pending.has(message.id)) return;
    const handler = pending.get(message.id);
    pending.delete(message.id);
    if (message.error) handler.reject(new Error(message.error.message));
    else handler.resolve(message.result);
  });

  await command("Emulation.setDeviceMetricsOverride", {
    width: 1440,
    height: 900,
    deviceScaleFactor: 1,
    mobile: false,
  });
  await command("Emulation.setEmulatedMedia", {
    features: [{ name: "prefers-reduced-motion", value: "reduce" }],
  });
  const page = await fetch(target.url);
  if (!page.ok) throw new Error(`Page unavailable: ${target.url} (${page.status})`);
  if (target.url.startsWith(desktopBase)) {
    await command("Runtime.evaluate", {
      expression: `localStorage.setItem('akoflow-environment-onboarding-dismissed', 'true'); ${apiToken ? `localStorage.setItem('akoflow-api-token', ${JSON.stringify(apiToken)})` : ""}`,
    });
    await command("Page.reload", { ignoreCache: true });
  }
  await delay(target.url.includes("127.0.0.1:5173") ? 1800 : 1200);
  if (click) {
    const selection = await command("Runtime.evaluate", {
      expression: `(() => { const label = ${JSON.stringify(click)}; const buttons = [...document.querySelectorAll('button')]; const button = buttons.find((item) => item.textContent.trim() === label) || buttons.find((item) => item.textContent.trim().startsWith(label)); if (!button) return false; button.click(); return true; })()`,
      returnByValue: true,
    });
    if (!selection.result.value) throw new Error(`Tab ${click} was not found on ${target.url}`);
    await delay(450);
  }
  const state = await command("Runtime.evaluate", {
    expression: "({title: document.title, text: document.body?.innerText?.slice(0, 1200) || ''})",
    returnByValue: true,
  });
  const visible = state.result.value;
  if (/This site can.t be reached|Instance identity unavailable|Loading AkôFlow|Welcome to AkôFlow|Unable to load this page|Loading operation…/.test(visible.text))
    throw new Error(`Invalid capture for ${target.url}: ${visible.text.slice(0, 120)}`);
  if (target.url.startsWith(desktopBase)) {
    await command("Runtime.evaluate", {
      expression: `(() => { const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT); while (walker.nextNode()) walker.currentNode.textContent = walker.currentNode.textContent.replace(/srv[0-9]+/g, 'demo-instance').replace(/run-[0-9]{10,}/g, 'demo-run').replace(/cloud-instance-[0-9a-f-]{36}/g, 'demo-cloud-instance'); for (const input of document.querySelectorAll('input')) if (/^run-[0-9]{10,}$/.test(input.value)) { const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set; setter.call(input, 'demo-run'); input.dispatchEvent(new Event('input', {bubbles: true})); } })()`,
    });
  }
  const result = await command("Page.captureScreenshot", {
    format: "png",
    captureBeyondViewport: false,
    fromSurface: true,
  });
  await mkdir(path.dirname(output), { recursive: true });
  await writeFile(output, Buffer.from(result.data, "base64"));
  socket.close();
}

const chrome = spawn(
  chromeExecutable,
  [
    "--headless=new",
    "--no-sandbox",
    "--disable-gpu",
    "--hide-scrollbars",
    `--remote-debugging-port=${debuggingPort}`,
    `--user-data-dir=${profileDirectory}`,
    "about:blank",
  ],
  { stdio: "ignore" },
);

try {
  await waitForDebugger();
  for (const item of selectedCaptures) {
    const target = await createTarget(item.url);
    const output = path.resolve(docsDirectory, item.output);
    await capture(target, output, item.click);
    console.log(
      `Captured ${item.name} -> ${path.relative(docsDirectory, output)}`,
    );
  }
} finally {
  if (chrome.exitCode === null && chrome.signalCode === null) {
    const exited = new Promise((resolve) => chrome.once("exit", resolve));
    chrome.kill("SIGTERM");
    await exited;
  }
  await rm(profileDirectory, {
    recursive: true,
    force: true,
    maxRetries: 4,
    retryDelay: 100,
  });
}
