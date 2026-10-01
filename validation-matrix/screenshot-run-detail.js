#!/usr/bin/env node
const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

const BASE = process.env.AKOFLOW_DESKTOP_URL || 'http://127.0.0.1:5173';
const RUN_ID = process.argv[2] || 'local-direct-hello-run-v1';
const OUT_DIR = process.argv[3] || '.';
fs.mkdirSync(OUT_DIR, { recursive: true });

const routes = [
  [`/runs/${RUN_ID}`, `${OUT_DIR}/${RUN_ID}-run-detail.png`],
  [`/runs/${RUN_ID}/artifacts`, `${OUT_DIR}/${RUN_ID}-artifacts.png`],
  [`/runs/${RUN_ID}/tasks`, `${OUT_DIR}/${RUN_ID}-tasks.png`],
];

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  for (const [routePath, out] of routes) {
    const url = BASE + routePath;
    console.log(`GET ${url}`);
    try {
      await page.goto(url, { waitUntil: 'networkidle', timeout: 30000 });
      await page.waitForTimeout(2500);
      await page.screenshot({ path: out, fullPage: true });
      console.log(`  saved ${out}`);
    } catch (e) {
      console.log(`  ERROR ${url}: ${e.message}`);
    }
  }
  await browser.close();
})();
