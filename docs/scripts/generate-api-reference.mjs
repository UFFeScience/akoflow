import { mkdir, readFile, readdir, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const docsDirectory = path.resolve(scriptDirectory, "..");
const routerFile = path.resolve(
  docsDirectory,
  "..",
  "internal/api/httpserver/httpserver.go",
);
const handlerFile = path.resolve(
  docsDirectory,
  "..",
  "internal/api/handlers/workflow_engine_api_handler/workflow_engine_api_handler.go",
);
const outputDirectory = path.resolve(docsDirectory, "docs/api/endpoints");
const manifestDirectory = path.resolve(docsDirectory, ".generated");
const manifestFile = path.join(manifestDirectory, "api-manifest.json");
const sourceRoot = path.resolve(docsDirectory, "..", "internal");

const groupRules = [
  [
    "Instance",
    /^\/(instance|instances|instance-activations|factory-reset|user-preferences|preflight|search)/,
  ],
  [
    "Environments",
    /^\/(environments|environment-connections|connection-tests|ssh-keys|kubernetes-tokens)/,
  ],
  ["Cloud", /^\/(cloud-|machine-configuration)/],
  ["Storage", /^\/(storages|storage-downloads)/],
  ["Resources", /^\/(resources|network-topologies|execution-scopes)/],
  ["Workflows", /^\/(workflow-definitions|workflow-definition-actions)/],
  ["Planning", /^\/(schedule-plans|planning-)/],
  ["Executions", /^\/execution-runs/],
  ["Artifacts", /^\/(artifacts|artifact-|build-)/],
  ["Provenance", /^\/provenance/],
  ["Console", /^\/console-/],
  ["Audit", /^\/audit-events/],
];

const methodOrder = { GET: 1, POST: 2, PUT: 3, PATCH: 4, DELETE: 5 };

const groupMetadata = {
  Instance: [
    "/docs/installation",
    "instance configuration, archives, preferences, and search",
  ],
  Environments: [
    "/docs/guides/infrastructure/environments",
    "environments and connections",
  ],
  Cloud: [
    "/docs/guides/infrastructure/cloud-capacity",
    "cloud capacity and machine configuration",
  ],
  Storage: [
    "/docs/guides/infrastructure/storage",
    "remote storage content and operations",
  ],
  Resources: [
    "/docs/guides/infrastructure/execution-scopes",
    "resources, network topology, and execution scopes",
  ],
  Workflows: ["/docs/guides/workflows/definitions", "workflow definitions"],
  Planning: ["/docs/guides/workflows/planning", "plans and planning sessions"],
  Executions: [
    "/docs/guides/workflows/executions",
    "workflow and interactive runs",
  ],
  Artifacts: [
    "/docs/guides/data/artifacts",
    "artifacts, materializations, and builds",
  ],
  Provenance: [
    "/docs/guides/data/provenance-and-audit",
    "scientific provenance",
  ],
  Console: [
    "/docs/guides/interface-tour",
    "interactive console sessions and commands",
  ],
  Audit: ["/docs/guides/data/provenance-and-audit", "operational audit events"],
  System: ["/docs/getting-started", "daemon status"],
};

const queryDescriptions = {
  connectionId: "Limit results to this environment connection.",
  cursor: "Opaque cursor returned by the preceding page.",
  depth: "Maximum lineage traversal depth.",
  direction: "Lineage traversal direction.",
  environmentId: "Limit results to this environment.",
  eventType: "Limit results to this audit event type.",
  executionId: "Limit results to this execution run.",
  filterField: "Entity field to filter.",
  filterValue: "Value required in the selected filter field.",
  includeArtifacts:
    "Set to `true` to include artifact bytes in the exported archive.",
  kind: "Limit execution runs to this run kind.",
  limit: "Maximum number of results to return.",
  maxNodes: "Maximum number of nodes in the lineage graph.",
  mode: "Limit execution runs to this execution mode.",
  outcome: "Limit results to this audit outcome.",
  page: "One-based result page.",
  pageSize: "Number of results per page.",
  path: "Path inside the selected storage root.",
  q: "Free-text search query.",
  resourceId: "Limit results to this resource.",
  runId: "Limit results to this run.",
  sessionId: "Limit results to this session.",
  sortField: "Entity field used to order results.",
  sortOrder: "Sort direction for the selected field.",
  status: "Limit execution runs to this status.",
  types: "Comma-separated result types included in global search.",
};

const statusNames = {
  StatusOK: "200 OK",
  StatusCreated: "201 Created",
  StatusAccepted: "202 Accepted",
  StatusNoContent: "204 No Content",
};

// Route-specific wording is reserved for aliases whose handler name cannot
// explain the public operation on its own.
const routeTitles = {
  "GET /": "Check daemon health",
  "GET /akoflow-api/preflight/": "Inspect daemon preflight",
  "POST /akoflow-api/workflow-definitions/import/":
    "Import workflow definition",
  "GET /akoflow-api/storages/{storageId}/entry/": "Inspect storage entry",
  "GET /akoflow-api/storages/{storageId}/stat/":
    "Inspect storage entry (compatibility route)",
};

function humanizeHandler(handler) {
  const phrase = handler
    .replace(/([A-Z]+)([A-Z][a-z])/g, "$1 $2")
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .trim();
  return phrase.replace(/^Get /, "Retrieve ");
}

function endpointPath(routePath) {
  return routePath.replace(/^\/akoflow-api/, "") || "/";
}

function groupFor(routePath) {
  const relative = endpointPath(routePath);
  return (
    groupRules.find(([, pattern]) => pattern.test(relative))?.[0] || "Instance"
  );
}

function slugFor(method, routePath) {
  const pathPart =
    endpointPath(routePath)
      .replace(/[{}]/g, "")
      .replace(/[^a-zA-Z0-9]+/g, "-")
      .replace(/^-|-$/g, "")
      .toLowerCase() || "health";
  return `${method.toLowerCase()}-${pathPart}`;
}

function extractParameters(routePath) {
  return [...routePath.matchAll(/\{([^}]+)\}/g)].map((match) => match[1]);
}

function replaceParameters(routePath) {
  return routePath.replace(/\{([^}]+)\}/g, (_, name) => `<${name}>`);
}

function extractFunctionBody(source, handler) {
  const match = new RegExp(
    `func (?:\\(h \\*Handler\\) )?${handler}\\([^\\n]*\\) \\{`,
  ).exec(source);
  if (!match) return "";
  const start = match.index + match[0].length;
  let depth = 1;
  for (let index = start; index < source.length; index += 1) {
    if (source[index] === "{") depth += 1;
    if (source[index] === "}" && --depth === 0)
      return source.slice(start, index);
  }
  return "";
}

function extractQueryParameters(body) {
  return [
    ...new Set(
      [...body.matchAll(/r\.URL\.Query\(\)\.(?:Get|Has)\("([^"]+)"\)/g)].map(
        (match) => match[1],
      ),
    ),
  ].sort();
}

function extractSuccessStatuses(body) {
  const statuses = new Set();
  for (const match of body.matchAll(
    /http\.(Status(?:OK|Created|Accepted|NoContent))/g,
  )) {
    if (statusNames[match[1]]) statuses.add(statusNames[match[1]]);
  }
  if (/\b(?:writeList|writeItem)\(/.test(body)) statuses.add("200 OK");
  if (statuses.size === 0) statuses.add("200 OK");
  return [...statuses];
}

async function collectGoSources(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const sources = [];
  for (const entry of entries) {
    const target = path.join(directory, entry.name);
    if (entry.isDirectory()) sources.push(...(await collectGoSources(target)));
    else if (entry.name.endsWith(".go") && !entry.name.endsWith("_test.go")) {
      sources.push(await readFile(target, "utf8"));
    }
  }
  return sources;
}

function parseJSONFields(structBody) {
  const fields = [];
  for (const line of structBody.split("\n")) {
    const tagged =
      /^\s*([A-Za-z][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z][A-Za-z0-9_]*)*)\s+([^`]+?)\s+`json:"([^",]+)[^`]*`/.exec(
        line,
      );
    if (tagged) {
      if (tagged[3] !== "-")
        fields.push({ name: tagged[3], type: tagged[2].trim() });
      continue;
    }
    const untagged =
      /^\s*([A-Z][A-Za-z0-9_]*(?:\s*,\s*[A-Z][A-Za-z0-9_]*)*)\s+([^\s/]+)\s*$/.exec(
        line,
      );
    if (untagged) {
      for (const name of untagged[1].split(",").map((value) => value.trim())) {
        fields.push({ name, type: untagged[2] });
      }
    }
  }
  return fields;
}

function buildStructIndex(sources) {
  const index = new Map();
  const aliases = new Map();
  for (const sourceText of sources) {
    for (const match of sourceText.matchAll(
      /type\s+([A-Za-z0-9_]+)\s+struct\s*\{([\s\S]*?)\n\}/g,
    )) {
      const fields = parseJSONFields(match[2]);
      if (fields.length > 0) index.set(match[1], fields);
    }
    for (const match of sourceText.matchAll(
      /type\s+([A-Za-z0-9_]+)\s*=\s*(?:[A-Za-z0-9_]+\.)?([A-Za-z0-9_]+)/g,
    )) {
      aliases.set(match[1], match[2]);
    }
  }
  for (const [alias, target] of aliases) {
    const fields = index.get(target);
    if (fields) index.set(alias, fields);
  }
  return index;
}

function buildReturnTypeIndex(sources) {
  const index = new Map();
  const add = (name, signature) => {
    const first = signature.split(",")[0].trim();
    if (!first || first === "error") return;
    const values = index.get(name) || new Set();
    values.add(first);
    index.set(name, values);
  };
  for (const sourceText of sources) {
    for (const match of sourceText.matchAll(
      /^\s*([A-Z][A-Za-z0-9_]*)\([^\n]*\)\s+\(([^\n)]+)\)/gm,
    ))
      add(match[1], match[2]);
    for (const match of sourceText.matchAll(
      /^func\s+(?:\([^)]*\)\s*)?([A-Z][A-Za-z0-9_]*)\([^\n]*\)\s+\(([^\n)]+)\)/gm,
    ))
      add(match[1], match[2]);
  }
  return index;
}

function buildOwnerReturnTypeIndex(sources) {
  const index = new Map();
  for (const sourceText of sources) {
    for (const interfaceMatch of sourceText.matchAll(
      /type\s+([A-Za-z0-9_]+)\s+interface\s*\{([\s\S]*?)\n\}/g,
    )) {
      for (const method of interfaceMatch[2].matchAll(
        /^\s*([A-Z][A-Za-z0-9_]*)\([^\n]*\)\s+\(([^\n)]+)\)/gm,
      ))
        index.set(
          `${interfaceMatch[1]}.${method[1]}`,
          method[2].split(",")[0].trim(),
        );
    }
    for (const method of sourceText.matchAll(
      /^func\s+\([^)]*\*?([A-Za-z0-9_]+)\)\s+([A-Z][A-Za-z0-9_]*)\([^\n]*\)\s+\(([^\n)]+)\)/gm,
    ))
      index.set(`${method[1]}.${method[2]}`, method[3].split(",")[0].trim());
  }
  return index;
}

function extractHandlerFieldTypes(handlerSource) {
  const body =
    /type\s+Handler\s+struct\s*\{([\s\S]*?)\n\}/.exec(handlerSource)?.[1] || "";
  return new Map(
    [...body.matchAll(/^\s*([a-z][A-Za-z0-9_]*)\s+([^\s]+)\s*$/gm)].map(
      (match) => [match[1], match[2].replace(/^\*/, "").split(".").at(-1)],
    ),
  );
}

function sampleValue(typeName, structIndex, depth = 0) {
  const clean = typeName.replace(/^\*+/, "").trim();
  if (/^\[\]/.test(clean))
    return [sampleValue(clean.replace(/^\[\]/, ""), structIndex, depth + 1)];
  if (/^map\[/.test(clean) || /^(?:any|interface\{\})$/.test(clean)) return {};
  if (/bool$/.test(clean)) return true;
  if (/(?:^|\.)(?:Time|Duration)$/.test(clean)) return "2026-01-01T00:00:00Z";
  if (/(?:^|\b)(?:u?int(?:8|16|32|64)?|float(?:32|64))$/.test(clean)) return 0;
  const shortName = clean.split(".").at(-1);
  const fields = structIndex.get(shortName);
  if (fields && depth < 3) {
    return Object.fromEntries(
      fields.map((field) => [
        field.name,
        sampleValue(field.type, structIndex, depth + 1),
      ]),
    );
  }
  return "string";
}

function extractRequestContract(body, structIndex) {
  const decoded = /decode\(w, r, &([A-Za-z0-9_]+)\)/.exec(body)?.[1];
  if (!decoded) return null;
  const inline = new RegExp(
    `var\\s+${decoded}\\s+struct\\s*\\{([\\s\\S]*?)\\}`,
  ).exec(body);
  if (inline)
    return {
      type: "inline request",
      example: Object.fromEntries(
        parseJSONFields(inline[1]).map((field) => [
          field.name,
          sampleValue(field.type, structIndex),
        ]),
      ),
    };
  const named = new RegExp(`var\\s+${decoded}\\s+([^\\s]+)`).exec(body)?.[1];
  return {
    type: named || "JSON request",
    example: sampleValue(named || "", structIndex),
  };
}

function extractResponseContract(
  endpoint,
  body,
  structIndex,
  returnTypeIndex,
  ownerReturnTypeIndex,
  handlerFieldTypes,
) {
  const responseTypeOverrides = {
    ConfigureCloudInstance: "domain.CloudOperationRun",
    DestroyCloudInstance: "domain.CloudOperationRun",
    StartCloudInstance: "domain.CloudOperationRun",
    StopCloudInstance: "domain.CloudOperationRun",
    ValidateCloudInstance: "domain.CloudOperationRun",
    ProvisionCloudInstance: "domain.CloudOperationRun",
    StartCloudProvisioning: "domain.CloudOperationRun",
    ValidateMachineConfiguration: "domain.MachineConfigurationValidation",
    ImportSSHKey: "sshkey.Key",
    ListSSHKeys: "[]sshkey.Key",
    ListPlanningAlgorithms: "[]ports.SchedulerDescriptor",
    ImportPlan: "domain.SchedulePlan",
  };
  if (endpoint.path === "/")
    return { mediaType: "text/plain", type: "plain text", example: "ok" };
  if (endpoint.path === "/akoflow-api/preflight/") {
    const check = { available: true, message: "service is available" };
    return {
      mediaType: "application/json",
      type: "preflight checks",
      example: { server: check, docker: check, buildkit: check },
    };
  }
  if (endpoint.handler === "StreamConsoleSession")
    return {
      mediaType: "application/websocket",
      type: "bidirectional terminal byte stream",
      example: null,
    };
  if (endpoint.handler === "ExportArchiveInstance")
    return { mediaType: "application/zip", type: "ZIP archive", example: null };
  const streamedContentType = /Content-Type",\s*"([^"]+)"/.exec(body)?.[1];
  if (streamedContentType && /(?:io\.Copy|w\.Write)\(/.test(body))
    return {
      mediaType: streamedContentType,
      type: streamedContentType.includes("text/")
        ? "text stream"
        : "binary stream",
      example: streamedContentType.includes("text/")
        ? "streamed content"
        : null,
    };
  if (/Content-Type",\s*"application\/octet-stream"/.test(body))
    return {
      mediaType: "application/octet-stream",
      type: "binary stream",
      example: null,
    };
  if (/Content-Type",\s*"text\/event-stream"/.test(body))
    return {
      mediaType: "text/event-stream",
      type: "event stream",
      example: "data: {...}",
    };
  if (endpoint.successStatuses.some((status) => status.startsWith("204")))
    return { mediaType: null, type: "empty response", example: null };
  const explicitMap = [
    ...body.matchAll(
      /writeJSON\(w,\s*http\.Status[A-Za-z]+,\s*map\[string\](?:any|string)\s*\{([^}]*)\}/g,
    ),
  ].at(-1);
  if (explicitMap) {
    const keys = [...explicitMap[1].matchAll(/"([^"]+)"\s*:/g)].map(
      (match) => match[1],
    );
    return {
      mediaType: "application/json",
      type: "JSON object",
      example: Object.fromEntries(keys.map((key) => [key, "string"])),
    };
  }

  const responseVariable =
    /(?:writeItem|writeList|writeJSON)\(w,\s*(?:http\.Status[A-Za-z]+,\s*)?([A-Za-z0-9_]+)/g;
  const variable = [...body.matchAll(responseVariable)].at(-1)?.[1];
  if (variable) {
    const assignedMap = new RegExp(
      `${variable}\\s*:=\\s*map\\[string\\]any\\s*\\{([\\s\\S]*?)\\n\\s*\\}`,
    ).exec(body);
    if (assignedMap) {
      const keys = [...assignedMap[1].matchAll(/"([^"]+)"\s*:/g)].map(
        (match) => match[1],
      );
      return {
        mediaType: "application/json",
        type: "JSON object",
        example: Object.fromEntries(
          keys.map((key) => [
            key,
            /(?:s|activities|handles|events)$/i.test(key) ? [] : {},
          ]),
        ),
      };
    }
  }
  const declaredType = variable
    ? new RegExp(`var\\s+${variable}\\s+([^\\s]+)`).exec(body)?.[1]
    : null;
  const producer = variable
    ? new RegExp(
        `${variable}\\s*(?:,[^:=\\n]+)?\\s*:=\\s*(?:[A-Za-z0-9_]+\\.)*([A-Z][A-Za-z0-9_]*)\\(`,
      ).exec(body)?.[1]
    : null;
  const ownedProducer = variable
    ? new RegExp(
        `${variable}\\s*(?:,[^:=\\n]+)?\\s*:=\\s*h\\.([a-z][A-Za-z0-9_]*)\\.([A-Z][A-Za-z0-9_]*)\\(`,
      ).exec(body)
    : null;
  const owner = ownedProducer ? handlerFieldTypes.get(ownedProducer[1]) : null;
  const ownerType = owner
    ? ownerReturnTypeIndex.get(`${owner}.${ownedProducer[2]}`)
    : null;
  const candidates = producer ? [...(returnTypeIndex.get(producer) || [])] : [];
  const inferredType =
    responseTypeOverrides[endpoint.handler] ||
    declaredType ||
    ownerType ||
    (candidates.length === 1 ? candidates[0] : null);
  return {
    mediaType: "application/json",
    type: inferredType || "JSON object",
    example: inferredType ? sampleValue(inferredType, structIndex) : {},
  };
}

function endpointDescription(endpoint) {
  const action = endpoint.title;
  return `${action.charAt(0).toLowerCase()}${action.slice(1)} through the AkôFlow ${groupMetadata[endpoint.group][1]} API.`;
}

function endpointDocument(endpoint, position) {
  const relativePath = endpointPath(endpoint.path);
  const title = endpoint.title;
  const params = extractParameters(endpoint.path);
  const body = Boolean(endpoint.request);
  const requestSection = endpoint.request
    ? `\n## Request body\n\nContent-Type: \`application/json\` or \`application/yaml\`\n\nContract: \`${endpoint.request.type}\`\n\n\`\`\`json\n${JSON.stringify(endpoint.request.example, null, 2)}\n\`\`\`\n`
    : `\n## Request body\n\nThis endpoint does not accept a request body.\n`;
  const responseBody =
    endpoint.response.example === null
      ? endpoint.response.type === "empty response"
        ? "No response body."
        : `Returns a ${endpoint.response.type}; it is not JSON.`
      : `\`\`\`${endpoint.response.mediaType === "application/json" ? "json" : "text"}\n${typeof endpoint.response.example === "string" ? endpoint.response.example : JSON.stringify(endpoint.response.example, null, 2)}\n\`\`\``;
  const querySection =
    endpoint.queryParameters.length === 0
      ? ""
      : `
## Query parameters

${endpoint.queryParameters.map((parameter) => `- \`${parameter}\`: ${queryDescriptions[parameter] || "Optional query value consumed by this handler."}`).join("\n")}
`;
  return `---
title: ${JSON.stringify(title)}
sidebar_label: ${JSON.stringify(`${endpoint.method} ${relativePath}`)}
sidebar_position: ${position}
hide_title: true
hide_table_of_contents: true
custom_edit_url: null
description: ${JSON.stringify(endpoint.description)}
---

import ApiEndpoint from '@site/src/components/ApiEndpoint';

<ApiEndpoint
  method=${JSON.stringify(endpoint.method)}
  path=${JSON.stringify(endpoint.path)}
  handler=${JSON.stringify(endpoint.handler)}
  group=${JSON.stringify(endpoint.group)}
  pathParams={${JSON.stringify(params)}}
  queryParams={${JSON.stringify(endpoint.queryParameters)}}
  successStatuses={${JSON.stringify(endpoint.successStatuses)}}
  requestExample={${JSON.stringify(endpoint.request ? JSON.stringify(endpoint.request.example, null, 2) : null)}}
  requestType=${JSON.stringify(endpoint.request?.type || "No request body")}
  responseExample={${JSON.stringify(endpoint.response.example === null ? null : typeof endpoint.response.example === "string" ? endpoint.response.example : JSON.stringify(endpoint.response.example, null, 2))}}
  responseType=${JSON.stringify(endpoint.response.type)}
  responseMediaType={${JSON.stringify(endpoint.response.mediaType)}}
  hasRequestBody={${body}}
/>

This endpoint ${endpoint.description}
${requestSection}

## Successful response

${endpoint.successStatuses.map((status) => `- **${status}**`).join("\n")}

Media type: ${endpoint.response.mediaType ? `\`${endpoint.response.mediaType}\`` : "none"}

Contract: \`${endpoint.response.type}\`

${responseBody}

### Error response

\`\`\`json
{
  "error": "error description"
}
\`\`\`
${querySection}
## Related guide

See the [${endpoint.group} guide](${groupMetadata[endpoint.group][0]}) for the corresponding Desktop workflow, concepts, and authored request examples.

:::info Generated from the daemon router
This page is generated from \`internal/api/httpserver/httpserver.go\`, the registered handler, and JSON-tagged Go structs. Method, path, request fields, response kind, query parameters, media type, and successful status codes stay synchronized with the implementation.
:::
`;
}

const source = await readFile(routerFile, "utf8");
const handlerSource = await readFile(handlerFile, "utf8");
const goSources = await collectGoSources(sourceRoot);
const structIndex = buildStructIndex(goSources);
const returnTypeIndex = buildReturnTypeIndex(goSources);
const ownerReturnTypeIndex = buildOwnerReturnTypeIndex(goSources);
const handlerFieldTypes = extractHandlerFieldTypes(handlerSource);
const routePattern =
  /mux\.HandleFunc\("(GET|POST|PUT|PATCH|DELETE) ([^" ]+)",\s*(?:http_config\.KernelHandler\()?(?:workflowEngine\.)?([A-Za-z0-9_]+)/g;
const endpoints = [];
for (const match of source.matchAll(routePattern)) {
  const [, method, routePath, handler] = match;
  const group = groupFor(routePath);
  const handlerBody = extractFunctionBody(
    `${handlerSource}\n${source}`,
    handler,
  );
  const title =
    routeTitles[`${method} ${routePath}`] || humanizeHandler(handler);
  const endpoint = { method, path: routePath, handler, group, title };
  const baseEndpoint = {
    ...endpoint,
    description: endpointDescription(endpoint),
    queryParameters: extractQueryParameters(handlerBody),
    successStatuses: extractSuccessStatuses(handlerBody),
    guide: groupMetadata[group][0],
  };
  endpoints.push({
    ...baseEndpoint,
    request: extractRequestContract(handlerBody, structIndex),
    response: extractResponseContract(
      baseEndpoint,
      handlerBody,
      structIndex,
      returnTypeIndex,
      ownerReturnTypeIndex,
      handlerFieldTypes,
    ),
  });
}

if (endpoints.length === 0) {
  throw new Error(`No API routes found in ${routerFile}`);
}

endpoints.sort(
  (left, right) =>
    left.group.localeCompare(right.group) ||
    left.path.localeCompare(right.path) ||
    methodOrder[left.method] - methodOrder[right.method],
);

await rm(outputDirectory, { recursive: true, force: true });
await mkdir(outputDirectory, { recursive: true });
await mkdir(manifestDirectory, { recursive: true });

const groupPositions = new Map();
for (const [index, group] of [
  ...new Set(endpoints.map((endpoint) => endpoint.group)),
].entries()) {
  groupPositions.set(group, index + 1);
  const groupDirectory = path.join(
    outputDirectory,
    group.toLowerCase().replaceAll(" ", "-"),
  );
  await mkdir(groupDirectory, { recursive: true });
  await writeFile(
    path.join(groupDirectory, "_category_.json"),
    `${JSON.stringify({ label: group, position: index + 1, collapsed: true }, null, 2)}\n`,
  );
}

const positions = new Map();
for (const endpoint of endpoints) {
  const currentPosition = (positions.get(endpoint.group) || 0) + 1;
  positions.set(endpoint.group, currentPosition);
  const groupDirectory = path.join(
    outputDirectory,
    endpoint.group.toLowerCase().replaceAll(" ", "-"),
  );
  await writeFile(
    path.join(groupDirectory, `${slugFor(endpoint.method, endpoint.path)}.mdx`),
    endpointDocument(endpoint, currentPosition),
  );
}

await writeFile(
  manifestFile,
  `${JSON.stringify({ source: path.relative(docsDirectory, routerFile), generatedAt: new Date().toISOString(), endpoints }, null, 2)}\n`,
);

console.log(
  `Generated ${endpoints.length} API endpoint pages from ${path.relative(docsDirectory, routerFile)}`,
);
