#!/usr/bin/env node

// Check the syntax of fenced Bash examples in authored documentation.
import { readdirSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { dirname, extname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const docsDirectory = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const contentDirectory = join(docsDirectory, "docs");
const errors = [];
let checked = 0;

function walk(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    return entry.isDirectory() ? walk(path) : [path];
  });
}

for (const file of walk(contentDirectory)) {
  if (![".md", ".mdx"].includes(extname(file))) continue;
  const lines = readFileSync(file, "utf8").split("\n");
  for (let index = 0; index < lines.length; index++) {
    if (!/^```(?:bash|sh)\s*$/.test(lines[index])) continue;
    const start = index + 1;
    const body = [];
    while (++index < lines.length && !/^```\s*$/.test(lines[index])) {
      body.push(lines[index]);
    }
    if (index === lines.length) {
      errors.push(`${relative(docsDirectory, file)}:${start}: unclosed shell fence`);
      break;
    }
    checked++;
    const result = spawnSync("bash", ["-n"], { input: body.join("\n"), encoding: "utf8" });
    if (result.error || result.status !== 0) {
      errors.push(`${relative(docsDirectory, file)}:${start}: ${(result.stderr || result.error?.message || "invalid Bash").trim()}`);
    }
  }
}

if (errors.length) {
  for (const error of errors) process.stderr.write(`${error}\n`);
  process.exitCode = 1;
} else {
  process.stdout.write(`Shell examples verified: ${checked} Bash/sh block(s).\n`);
}
