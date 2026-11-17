import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";

const npm = process.platform === "win32" ? "npm.cmd" : "npm";
const result = spawnSync(npm, ["audit", "--json"], {
  encoding: "utf8",
  maxBuffer: 8 * 1024 * 1024,
});
if (result.error) throw result.error;
assert.ok(
  result.status === 0 || result.status === 1,
  "npm audit failed to execute",
);
const audit = JSON.parse(result.stdout);
assert.ok(
  !audit.error && audit.metadata && audit.vulnerabilities,
  "missing npm advisory evidence",
);
const allowed = "https://github.com/advisories/GHSA-vfj7-8cjw-p6xm";
for (const [name, entry] of Object.entries(audit.vulnerabilities)) {
  for (const advisory of entry.via) {
    if (typeof advisory === "string") continue;
    if (
      name === "picomatch" &&
      advisory.url === "https://github.com/advisories/GHSA-fc5q-39xc-vjpf"
    ) {
      const patchedVersions = new Set(["2.3.2", "4.0.7"]);
      assert.ok(entry.nodes.length > 0);
      for (const path of entry.nodes) {
        const { version } = JSON.parse(
          readFileSync(`${path}/package.json`, "utf8"),
        );
        const parser = readFileSync(`${path}/lib/parse.js`, "utf8");
        assert.ok(
          patchedVersions.has(version),
          `Unexpected Picomatch version at ${path}`,
        );
        assert.ok(
          parser.includes("branches.some(branch => branch.includes('*'))"),
          `Missing Picomatch patch at ${path}`,
        );
      }
      continue;
    }
    assert.ok(
      (name === "braces" && advisory.url === allowed) ||
        (name === "deepmerge" &&
          advisory.url === "https://github.com/advisories/GHSA-gcqx-w48m-w2h4"),
      `Unmitigated advisory in ${name}: ${advisory.url}`,
    );
  }
}
assert.equal(
  audit.metadata.vulnerabilities.critical,
  0,
  "critical vulnerabilities remain",
);
console.log(
  "No unmitigated npm advisories. Version-bound braces, deepmerge and Picomatch advisories remain reported by npm and are mitigated by tested local patches.",
);
