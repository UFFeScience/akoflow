#!/usr/bin/env node

/**
 * Checks links that are owned by this repository. Docusaurus deliberately
 * treats broken links as warnings so that a stale external URL cannot stop a
 * local preview. This check is stricter for things we can verify locally:
 * documentation routes, static media, and the checked-in files advertised by
 * the Workflow Showcase.
 */
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { dirname, extname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const docsDirectory = resolve(scriptDirectory, "..");
const repositoryDirectory = resolve(docsDirectory, "..");
const contentDirectory = join(docsDirectory, "docs");
const staticDirectory = join(docsDirectory, "static");
const documentationExtensions = [".md", ".mdx"];
const localErrors = [];
let checkedLinks = 0;
let checkedShowcaseDownloads = 0;

function walk(directory) {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const entryPath = join(directory, entry.name);
    return entry.isDirectory() ? walk(entryPath) : [entryPath];
  });
}

function inside(directory, candidate) {
  const path = relative(directory, candidate);
  return path === "" || (!path.startsWith(`..${sep}`) && path !== "..");
}

function parseFrontMatter(source) {
  if (!source.startsWith("---\n")) return {};
  const end = source.indexOf("\n---", 4);
  if (end < 0) return {};
  const fields = {};
  for (const line of source.slice(4, end).split("\n")) {
    const match = line.match(/^([A-Za-z][A-Za-z0-9_-]*):\s*(.+?)\s*$/);
    if (match) fields[match[1]] = match[2].replace(/^['"]|['"]$/g, "");
  }
  return fields;
}

function cleanTarget(target) {
  const withoutTitle = target.trim().split(/\s+(?=["'])/)[0];
  return withoutTitle.replace(/[?#].*$/, "").replace(/^<|>$/g, "");
}

function documentationRoute(file, frontMatter) {
  const filePath = relative(contentDirectory, file).replace(extname(file), "");
  const defaultPath = filePath.endsWith("/index")
    ? filePath.slice(0, -"/index".length)
    : filePath === "index"
      ? ""
      : filePath;
  const path = frontMatter.slug ?? `/${defaultPath}`;
  return `/docs${path === "/" ? "/" : path.replace(/\/$/, "")}`.replace(/\/$/, "") || "/docs";
}

const documentationFiles = walk(contentDirectory).filter((file) =>
  documentationExtensions.includes(extname(file)),
);
const routes = new Set();
for (const file of documentationFiles) {
  routes.add(documentationRoute(file, parseFrontMatter(readFileSync(file, "utf8"))));
}

function report(sourceFile, target, reason) {
  localErrors.push(`${relative(repositoryDirectory, sourceFile)} → ${target}: ${reason}`);
}

function candidateDocumentationFiles(path) {
  const candidates = [path];
  if (!extname(path)) {
    candidates.push(...documentationExtensions.map((extension) => `${path}${extension}`));
    candidates.push(...documentationExtensions.map((extension) => join(path, `index${extension}`)));
  }
  return candidates;
}

function checkRepositoryPath(sourceFile, target, path, label) {
  if (!inside(repositoryDirectory, path) || !existsSync(path)) {
    report(sourceFile, target, `${label} does not exist in this checkout`);
    return false;
  }
  return true;
}

function checkTarget(sourceFile, rawTarget, sourceIsShowcase) {
  const target = cleanTarget(rawTarget);
  if (!target || target.startsWith("#") || /^(mailto:|tel:|data:|javascript:)/i.test(target)) return;

  const githubMatch = target.match(
    /^https:\/\/(?:raw\.githubusercontent\.com\/UFFeScience\/akoflow\/(v[0-9]+\.[0-9]+\.[0-9]+|main)\/|github\.com\/UFFeScience\/akoflow\/(?:blob|tree)\/(v[0-9]+\.[0-9]+\.[0-9]+|main)\/)(examples\/.+)$/,
  );
  if (githubMatch && sourceIsShowcase) {
    checkedShowcaseDownloads += 1;
    const ref = githubMatch[1] || githubMatch[2];
    const examplePath = githubMatch[3];
    if (ref === "main") {
      checkRepositoryPath(sourceFile, target, resolve(repositoryDirectory, examplePath), "Showcase download");
    } else {
      const result = spawnSync("git", ["cat-file", "-e", `${ref}:${examplePath}`], { cwd: repositoryDirectory });
      if (result.status !== 0) report(sourceFile, target, `Showcase download is missing from ${ref}`);
    }
    return;
  }

  if (/^[a-z][a-z0-9+.-]*:/i.test(target) || target.startsWith("//")) return;

  checkedLinks += 1;
  if (target.startsWith("@site/static/")) {
    checkRepositoryPath(sourceFile, target, resolve(staticDirectory, target.slice("@site/static/".length)), "Static asset");
    return;
  }

  if (target.startsWith("/examples/") || target.startsWith("/img/") || target.startsWith("/media/") || target.startsWith("/downloads/") || target.startsWith("/showcase/")) {
    checkRepositoryPath(sourceFile, target, resolve(staticDirectory, target.slice(1)), "Static asset");
    return;
  }

  if (target.startsWith("/docs/") || target.startsWith("/akoflow/docs/")) {
    const route = target.replace(/^\/akoflow/, "").replace(/\/$/, "") || "/docs";
    if (!routes.has(route)) report(sourceFile, target, "Documentation route does not exist");
    return;
  }

  const localPath = resolve(dirname(sourceFile), target);
  if (candidateDocumentationFiles(localPath).some((candidate) => existsSync(candidate))) return;

  if (extname(target)) {
    checkRepositoryPath(sourceFile, target, localPath, "Local link");
    return;
  }

  report(sourceFile, target, "Documentation target does not exist");
}

for (const sourceFile of documentationFiles) {
  const source = readFileSync(sourceFile, "utf8");
  const sourceIsShowcase = inside(join(contentDirectory, "showcase"), sourceFile);
  const targets = new Set();
  for (const match of source.matchAll(/!?\[[^\]]*\]\(([^)]+)\)/g)) targets.add(match[1]);
  for (const match of source.matchAll(/<(?:a|img)\b[^>]+(?:href|src)=["']([^"']+)["']/gi)) targets.add(match[1]);
  for (const match of source.matchAll(/require\(['"](@site\/static\/[^'"]+)['"]\)/g)) targets.add(match[1]);
  for (const target of targets) checkTarget(sourceFile, target, sourceIsShowcase);
}

if (localErrors.length > 0) {
  console.error(`Documentation link check failed with ${localErrors.length} issue(s):`);
  for (const error of localErrors) console.error(`- ${error}`);
  process.exitCode = 1;
} else {
  console.log(`Documentation links verified: ${checkedLinks} local route/asset link(s) and ${checkedShowcaseDownloads} Showcase download(s).`);
}
