import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { test } from "node:test";

const require = createRequire(import.meta.url);
const braces = require("braces");

test("deep brace and parenthesis patterns fail with bounded explicit errors", () => {
  for (const [open, close] of [
    ["{", "}"],
    ["(", ")"],
  ]) {
    const pattern = open.repeat(4000) + "a,b" + close.repeat(4000);
    for (const method of ["parse", "compile", "expand", "stringify"]) {
      assert.throws(() => braces[method](pattern), {
        name: "SyntaxError",
        message: /safe nesting depth/,
      });
    }
  }
});

test("direct AST walkers reject excessive depth independently of parse", () => {
  const ast = { type: "root", nodes: [] };
  let node = ast;
  for (let i = 0; i < 1000; i++) {
    const child = { type: "brace", nodes: [], parent: node };
    node.nodes.push(child);
    node = child;
  }
  for (const method of ["compile", "expand", "stringify"]) {
    assert.throws(() => braces[method](ast), {
      name: "SyntaxError",
      message: /safe nesting depth/,
    });
  }
});

test("normal nested patterns, ranges and filesystem glob consumers remain compatible", () => {
  assert.deepEqual(braces.expand("a/{b,{c,d}}/{1..2}"), [
    "a/b/1",
    "a/b/2",
    "a/c/1",
    "a/c/2",
    "a/d/1",
    "a/d/2",
  ]);
  assert.equal(braces.compile("a/{b,c}/d"), "a/(b|c)/d");
  assert.deepEqual(
    require("micromatch")(
      ["src/a.js", "src/b.ts", "other/c.js"],
      "src/*.{js,ts}",
    ),
    ["src/a.js", "src/b.ts"],
  );
});
