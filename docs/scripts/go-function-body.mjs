// Extract handler bodies without treating braces in Go strings/comments as code.
export function extractFunctionBody(source, handler) {
  const escaped = handler.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = new RegExp(
    `func\\s+(?:\\(h \\*Handler\\)\\s+)?${escaped}\\([^)]*\\)\\s*\\{`,
  ).exec(source);
  if (!match) return "";
  const start = match.index + match[0].length;
  let depth = 1;
  let state = "code";
  for (let index = start; index < source.length; index += 1) {
    const character = source[index];
    const next = source[index + 1];
    if (state === "line-comment") {
      if (character === "\n") state = "code";
      continue;
    }
    if (state === "block-comment") {
      if (character === "*" && next === "/") {
        state = "code";
        index += 1;
      }
      continue;
    }
    if (state === "raw-string") {
      if (character === "`") state = "code";
      continue;
    }
    if (state === '"' || state === "'") {
      if (character === "\\") index += 1;
      else if (character === state) state = "code";
      continue;
    }
    if (character === "/" && next === "/") {
      state = "line-comment";
      index += 1;
    } else if (character === "/" && next === "*") {
      state = "block-comment";
      index += 1;
    } else if (character === "`") state = "raw-string";
    else if (character === '"' || character === "'") state = character;
    else if (character === "{") depth += 1;
    else if (character === "}" && --depth === 0)
      return source.slice(start, index);
  }
  return "";
}
