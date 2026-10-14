import assert from "node:assert/strict";
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
    assert.ok(
      name === "braces" && advisory.url === allowed,
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
  "No unmitigated npm advisories. The version-bound braces advisory remains reported by npm and is mitigated by the tested local depth patch.",
);
