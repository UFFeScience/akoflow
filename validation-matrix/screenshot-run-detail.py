#!/usr/bin/env python3
"""Capture Playwright screenshots of the AkôFlow Desktop UI for a given run."""
import sys, os, time
from playwright.sync_api import sync_playwright

BASE = os.environ.get("AKOFLOW_DESKTOP_URL", "http://127.0.0.1:5173")
RUN_ID = sys.argv[1] if len(sys.argv) > 1 else "local-direct-hello-run-v1"
OUT_DIR = sys.argv[2] if len(sys.argv) > 2 else "."
os.makedirs(OUT_DIR, exist_ok=True)

routes = [
    (f"/runs/{RUN_ID}", f"{OUT_DIR}/{RUN_ID}-run-detail.png"),
    (f"/runs/{RUN_ID}/artifacts", f"{OUT_DIR}/{RUN_ID}-artifacts.png"),
    (f"/runs/{RUN_ID}/tasks", f"{OUT_DIR}/{RUN_ID}-tasks.png"),
]

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    ctx = browser.new_context(viewport={"width": 1440, "height": 900})
    page = ctx.new_page()
    for path, out in routes:
        url = BASE + path
        print(f"GET {url}")
        page.goto(url, wait_until="networkidle", timeout=30000)
        page.wait_for_timeout(2000)
        page.screenshot(path=out, full_page=True)
        print(f"  saved {out}")
    browser.close()
