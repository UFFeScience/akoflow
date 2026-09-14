#!/usr/bin/env node

/**
 * Ships every checked-in workflow example with the documentation site. The
 * Showcase pages can then offer their runnable sources without requiring a
 * separate GitHub checkout just to inspect an example.
 */
import {cpSync, existsSync, mkdirSync, rmSync} from "node:fs";
import {dirname, join, resolve} from "node:path";
import {fileURLToPath} from "node:url";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const docsDirectory = resolve(scriptDirectory, "..");
const repositoryDirectory = resolve(docsDirectory, "..");
const sourceDirectory = join(repositoryDirectory, "examples");
const outputDirectory = join(docsDirectory, "static", "examples", "showcase");

if (!existsSync(sourceDirectory)) {
  throw new Error("Showcase source directory is missing: examples/");
}

rmSync(outputDirectory, {force: true, recursive: true});
mkdirSync(outputDirectory, {recursive: true});
cpSync(sourceDirectory, outputDirectory, {
  dereference: true,
  filter: (path) => !path.includes("/.runtime/") && !path.endsWith("/.runtime"),
  recursive: true,
});

console.log("Published the Showcase example bundles.");
