import assert from "node:assert/strict";
import { test } from "node:test";
import { extractFunctionBody } from "./go-function-body.mjs";

test("finds a handler in any package source file with multiline arguments", () => {
  const source = `package handler
func (h *Handler) WorkspaceStorageBudget(
  w http.ResponseWriter,
  r *http.Request,
) {
  observer := h.storage.(interface { Budget() string })
  writeJSON(w, http.StatusOK, observer)
}`;
  assert.match(
    extractFunctionBody(source, "WorkspaceStorageBudget"),
    /StatusOK/,
  );
  assert.equal(extractFunctionBody(source, "MissingHandler"), "");
});

test("ignores Go comments, escaped strings, raw strings and rune braces", () => {
  const source = [
    "func (h *Handler) Inspect(w http.ResponseWriter, r *http.Request) {",
    '  message := "} escaped \\" {"',
    "  raw := `}} raw { braces`",
    "  rune := '}'",
    "  // } ignored line comment",
    "  /* }} ignored block comment */",
    "  if ready { writeJSON(w, http.StatusOK, message) }",
    "}",
    "func unrelated() { panic() }",
  ].join("\n");
  const body = extractFunctionBody(source, "Inspect");
  assert.match(body, /StatusOK/);
  assert.doesNotMatch(body, /unrelated/);
});

test("fails closed on an unterminated handler", () => {
  assert.equal(
    extractFunctionBody("func Inspect() { if ready {", "Inspect"),
    "",
  );
});
