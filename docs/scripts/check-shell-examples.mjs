#!/usr/bin/env node

// Check the syntax of fenced and Showcase JSX Bash examples.
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
  const source = readFileSync(file, "utf8");
  const lines = source.split("\n");
  const check = (body, line) => {
    checked++;
    const result = spawnSync("bash", ["-n"], { input: body, encoding: "utf8" });
    if (result.error || result.status !== 0) {
      errors.push(`${relative(docsDirectory, file)}:${line}: ${(result.stderr || result.error?.message || "invalid Bash").trim()}`);
    }
  };
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
    check(body.join("\n"), start);
  }
  for (const match of source.matchAll(/<pre><code>\{`([\s\S]*?)`\}<\/code><\/pre>/g)) {
    const line = source.slice(0, match.index).split("\n").length;
    // JSX template literals escape shell continuations and `${...}` variables.
    const body = match[1].replace(/\\([\\`$])/g, (_, escaped) => escaped);
    check(body, line);
  }
}

if (errors.length) {
  for (const error of errors) process.stderr.write(`${error}\n`);
  process.exitCode = 1;
} else {
  process.stdout.write(`Shell examples verified: ${checked} fenced/JSX Bash/sh block(s).\n`);
}
