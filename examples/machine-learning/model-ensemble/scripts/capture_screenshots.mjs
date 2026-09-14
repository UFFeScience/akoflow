#!/usr/bin/env node
// Generates browser screenshots after a successful Docker execution.
const {execFileSync} = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const root = path.resolve(__dirname, "..");
const output = path.join(root, "outputs");
const shots = path.join(root, "screenshots");
fs.mkdirSync(shots, {recursive: true});
const manifest = fs.readFileSync(path.join(output, "manifest.json"), "utf8");
execFileSync("npx", ["--yes", "playwright@1.52.0", "install", "chromium"], {stdio: "inherit"});
for (const view of ["workflow", "execution", "outputs"]) {
  const page = path.join(shots, view + ".html");
  fs.writeFileSync(page, "<!doctype html><meta charset=utf-8><style>body{font:16px system-ui;margin:48px;background:#101820;color:#e8f0f2}pre{padding:24px;background:#182833;border-radius:12px;white-space:pre-wrap}h1{color:#63d5b4}</style><h1>AkôFlow — " + view + "</h1><pre>" + manifest.replace(/</g, "&lt;") + "</pre>");
  execFileSync("npx", ["--yes", "playwright@1.52.0", "screenshot", "--device=Desktop Chrome", "file://" + page, path.join(shots, view + ".png")], {stdio: "inherit"});
  fs.unlinkSync(page);
}
