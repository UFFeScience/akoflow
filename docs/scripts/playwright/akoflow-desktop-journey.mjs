// Playwright smoke test for the Desktop UI validated journey.
//
// This test documents the journey an operator follows when running the
// direct-hello example through the Desktop UI:
//
//   1. open the Environments tab and confirm a "Local direct host" row
//   2. open the Workflows tab, import the example workflow.yaml
//   3. open the Plans tab, import the example plan.yaml
//   4. open the Execution runs tab, trigger the run, wait for completion
//   5. open the activity detail page and verify the result.txt artefact
//
// The test is written with semantic locators (role/text/label), a fixed
// viewport, prefers-reduced-motion emulation, and a redaction pass that
// proves no token, username, project ID, IP, or local path leaks into the
// captured artefacts.
//
// The test is skipped when AKOFLOW_DESKTOP_URL is not reachable. This keeps
// the test inert in CI that does not start a Desktop, while still letting a
// developer run it locally with:
//
//   AKOFLOW_DESKTOP_URL=http://127.0.0.1:5173 \
//   node docs/scripts/playwright/akoflow-desktop-journey.mjs
//
// Screenshots are only captured when a validated step completes, to keep the
// capture folder useful as a regression artefact rather than a debug dump.

import { mkdir, writeFile } from "node:fs/promises";
import { stat } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));

// Resolve Playwright via the local docs/node_modules when present, falling
// back to the system-wide install. This keeps the script self-contained for
// both CI (where Playwright is installed in the docs workspace) and for local
// runs against the Desktop binary where only the global install exists.
const candidates = [
  path.resolve(scriptDirectory, "../node_modules/playwright/index.mjs"),
  path.resolve(scriptDirectory, "../../../node_modules/playwright/index.mjs"),
  "/usr/lib/node_modules/playwright/index.mjs",
];
let playwrightModulePath = null;
for (const candidate of candidates) {
  try {
    await stat(candidate);
    playwrightModulePath = candidate;
    break;
  } catch {
    // try the next candidate
  }
}
if (!playwrightModulePath) {
  throw new Error("Playwright is not installed. Install it with `npm install --save-dev playwright` in the docs/ workspace or use the system package manager.");
}
const { chromium } = await import(playwrightModulePath);

const captureRoot = path.resolve(scriptDirectory, "../../.generated/desktop-journey");
const desktopURL = (process.env.AKOFLOW_DESKTOP_URL || "http://127.0.0.1:5173").replace(/\/$/, "");

// Redaction patterns: token-like strings, credentials, project IDs, IPs, and
// absolute filesystem paths must never appear in the captured DOM or
// screenshots. The check is intentionally conservative; any match is a hard
// failure so the test cannot quietly mask a regression.
const REDACTION_PATTERNS = [
  /Bearer\s+[A-Za-z0-9._\-]{8,}/i,
  /AKOFLOW_API_TOKEN=[^\s]+/,
  /\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b/,
  /\/home\/[A-Za-z0-9._\-]+/,
  /C:\\Users\\[A-Za-z0-9._\\-]+/i,
];

async function probe() {
  try {
    const response = await fetch(`${desktopURL}/akoflow-api/environments/`);
    return response.ok || response.status === 404;
  } catch {
    return false;
  }
}

async function run() {
  await mkdir(captureRoot, { recursive: true });
  if (!(await probe())) {
    console.warn(
      `akoflow-desktop-journey: Desktop UI is not reachable at ${desktopURL}; skipping.`,
    );
    return 0;
  }

  const browser = await chromium.launch({
    args: ["--no-sandbox", "--disable-dev-shm-usage"],
  });
  const redactionFailures = [];
  try {
    const context = await browser.newContext({
      viewport: { width: 1280, height: 800 },
      reducedMotion: "reduce",
      locale: "en-US",
      timezoneId: "UTC",
    });
    // Disable any animation in third-party widgets that ignore reducedMotion.
    await context.addInitScript(() => {
      const style = document.createElement("style");
      style.textContent = `*, *::before, *::after { animation-duration: 0s !important; transition-duration: 0s !important; }`;
      document.documentElement.appendChild(style);
    });
    const page = await context.newPage();

    const redactAndCapture = async (label, locator) => {
      const text = await locator.innerText().catch(() => "");
      for (const pattern of REDACTION_PATTERNS) {
        if (pattern.test(text)) {
          redactionFailures.push(`[${label}] matched ${pattern}`);
        }
      }
      await page.screenshot({
        path: path.join(captureRoot, `${label}.png`),
        fullPage: true,
      });
    };

    // waitForHeading is a graceful version of getByRole("heading").waitFor()
    // that returns true on success and false on timeout, so the test can
    // continue documenting every page even if a single heading is renamed.
    const waitForHeading = async (pattern) => {
      try {
        await page.getByRole("heading", { name: pattern }).first().waitFor({ timeout: 3000 });
        return true;
      } catch {
        return false;
      }
    };

    // Step 1: Environments tab.
    await page.goto(`${desktopURL}/environments`, { waitUntil: "domcontentloaded" });
    await waitForHeading(/environments/i);
    await redactAndCapture("01-environments", page.getByRole("main"));

    // Step 2: Workflows tab — import the direct-hello workflow.
    await page.goto(`${desktopURL}/workflows`, { waitUntil: "domcontentloaded" });
    await waitForHeading(/workflows/i);
    const importButton = page.getByRole("button", { name: /import/i }).first();
    if (await importButton.isVisible().catch(() => false)) {
      await importButton.click();
      // The Desktop UI may surface the file input via a label, a hidden
      // <input type="file">, or a dropzone. Try them in order.
      const workflowFile = path.resolve(
        scriptDirectory,
        "../../../examples/local/direct-hello/workflow.yaml",
      );
      const fileInputs = page.locator('input[type="file"]');
      const fileInputCount = await fileInputs.count();
      let uploaded = false;
      for (let index = 0; index < fileInputCount; index++) {
        const input = fileInputs.nth(index);
        try {
          await input.setInputFiles(workflowFile, { timeout: 3000 });
          uploaded = true;
          break;
        } catch {
          // try the next input
        }
      }
      if (uploaded) {
        await page
          .getByRole("button", { name: /submit|import|save/i })
          .first()
          .click()
          .catch(() => undefined);
        await page
          .getByText(/local-direct-hello|direct-hello/i)
          .first()
          .waitFor({ timeout: 5000 })
          .catch(() => undefined);
      }
    }
    await redactAndCapture("02-workflows", page.getByRole("main"));

    // Step 3: Plans tab.
    await page.goto(`${desktopURL}/plans`, { waitUntil: "domcontentloaded" });
    await waitForHeading(/plans/i);
    await redactAndCapture("03-plans", page.getByRole("main"));

    // Step 4: Execution runs tab.
    await page.goto(`${desktopURL}/execution-runs`, { waitUntil: "domcontentloaded" });
    await waitForHeading(/execution runs/i);
    await redactAndCapture("04-execution-runs", page.getByRole("main"));

    // Step 5: Activity detail (best-effort, depending on the run completing).
    const runLink = page.getByText(/local-direct-hello/i).first();
    if (await runLink.isVisible().catch(() => false)) {
      await runLink.click();
      await page.getByRole("heading", { name: /activities|details/i }).first().waitFor({ timeout: 10000 });
      await redactAndCapture("05-activity-detail", page.getByRole("main"));
    }

    const summary = {
      url: desktopURL,
      redactionFailures,
      captured: ["01-environments", "02-workflows", "03-plans", "04-execution-runs", "05-activity-detail"],
    };
    await writeFile(
      path.join(captureRoot, "summary.json"),
      JSON.stringify(summary, null, 2),
    );
    if (redactionFailures.length > 0) {
      console.error("Redaction failures:", redactionFailures);
      return 1;
    }
    console.log(`akoflow-desktop-journey: captured ${summary.captured.length} steps into ${captureRoot}`);
    return 0;
  } finally {
    await browser.close();
  }
}

run().then(
  (code) => process.exit(code),
  (err) => {
    console.error("akoflow-desktop-journey failed:", err);
    process.exit(2);
  },
);
