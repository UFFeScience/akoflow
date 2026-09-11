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

const captures = [
  {
    name: "desktop-overview",
    url: process.env.AKOFLOW_DESKTOP_URL || "http://127.0.0.1:5173/",
    output: "static/img/interface/overview/desktop-overview.png",
  },
  {
    name: "desktop-environments",
    url: `${process.env.AKOFLOW_DESKTOP_URL || "http://127.0.0.1:5173"}/environments`,
    output: "static/img/interface/infrastructure/environments.png",
  },
  {
    name: "desktop-workflows",
    url: `${process.env.AKOFLOW_DESKTOP_URL || "http://127.0.0.1:5173"}/workflows`,
    output: "static/img/interface/workflows/definitions.png",
  },
  {
    name: "generated-api-endpoint",
    url:
      process.env.AKOFLOW_DOCS_ENDPOINT_URL ||
      "http://localhost:3000/akoflow/docs/api/endpoints/workflows/get-workflow-definitions-workflowid",
    output: "static/img/interface/api/generated-endpoint.png",
  },
];

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

async function capture(target, output) {
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
  if (apiToken && target.url.includes("127.0.0.1:5173")) {
    await command("Runtime.evaluate", {
      expression: `localStorage.setItem('akoflow-api-token', ${JSON.stringify(apiToken)})`,
    });
    await command("Page.reload", { ignoreCache: true });
  }
  await delay(target.url.includes("127.0.0.1:5173") ? 7000 : 2500);
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
  for (const item of captures) {
    const target = await createTarget(item.url);
    const output = path.resolve(docsDirectory, item.output);
    await capture(target, output);
    console.log(
      `Captured ${item.name} -> ${path.relative(docsDirectory, output)}`,
    );
  }
} finally {
  if (chrome.exitCode === null) {
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
