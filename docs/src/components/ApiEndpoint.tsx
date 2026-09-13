import React, { useState } from "react";

type ApiEndpointProps = {
  title: string;
  method: string;
  path: string;
  handler: string;
  group: string;
  pathParams?: string[];
  queryParams?: string[];
  queryDescriptions?: Record<string, string>;
  successStatuses?: string[];
  requestExample?: string | null;
  requestExampleVerified?: boolean;
  responseExample?: string | null;
  responseType?: string;
  responseMediaType?: string | null;
  hasRequestBody?: boolean;
  requestMediaType?: string | null;
  requestFileName?: string | null;
  requestMultipartField?: string | null;
};

function commandFor(
  method: string,
  endpointPath: string,
  hasRequestBody: boolean,
  requestMediaType: string | null,
  requestFileName: string | null,
  requestMultipartField: string | null,
) {
  const lines = [
    `curl --fail-with-body \\`,
    `  -H "Authorization: Bearer \${AKOFLOW_API_TOKEN}" \\`,
  ];
  if (hasRequestBody) {
    if (requestMultipartField) {
      lines.push(`  -F "${requestMultipartField}=@${requestFileName || "upload.bin"}" \\`);
    } else {
      lines.push(`  -H "Content-Type: ${requestMediaType || "application/json"}" \\`);
      lines.push(`  --data-binary @${requestFileName || "request.json"} \\`);
    }
  }
  if (method !== "GET") lines.push(`  -X ${method} \\`);
  lines.push(
    endpointPath === "/"
      ? `  "\${AKOFLOW_API_URL%/akoflow-api}/"`
      : `  "\${AKOFLOW_API_URL}${endpointPath.replace("/akoflow-api", "")}"`,
  );
  return lines.join("\n");
}

export default function ApiEndpoint({
  title,
  method,
  path,
  handler,
  group,
  pathParams = [],
  queryParams = [],
  queryDescriptions = {},
  successStatuses = ["200 OK"],
  requestExample = null,
  requestExampleVerified = false,
  responseExample = null,
  responseType = "JSON object",
  responseMediaType = "application/json",
  hasRequestBody = false,
  requestMediaType = null,
  requestFileName = null,
  requestMultipartField = null,
}: ApiEndpointProps) {
  const [copied, setCopied] = useState(false);
  const displayPath = path.replace("/akoflow-api", "") || "/";
  const command = commandFor(method, path, hasRequestBody, requestMediaType, requestFileName, requestMultipartField);
  const commandIsTemplate = hasRequestBody || pathParams.length > 0;
  const requestHeading = requestExample
    ? requestExampleVerified ? "Example request" : "Request field shape"
    : "Request body";

  async function copyCommand() {
    await navigator.clipboard.writeText(command);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  }

  return (
    <div className="akoflow-api-reference">
      <main className="akoflow-api-main">
        <span className="akoflow-api-eyebrow">{group} endpoint</span>
        <h1>{title}</h1>
        <div className="akoflow-endpoint-signature">
          <span
            className={`akoflow-method akoflow-method-${method.toLowerCase()}`}
          >
            {method}
          </span>
          <code>{displayPath}</code>
        </div>

        {pathParams.length > 0 && (
          <section className="akoflow-api-section">
            <div className="akoflow-api-section-heading">
              <h2>Path parameters</h2>
            </div>
            <dl className="akoflow-api-parameters">
              {pathParams.map((parameter) => (
                <div key={parameter}>
                  <dt>
                    <code>{parameter}</code>
                  </dt>
                  <dd>Identifier used by the registered route.</dd>
                </div>
              ))}
            </dl>
          </section>
        )}

        <section className="akoflow-api-section">
          <div className="akoflow-api-section-heading">
            <h2>{requestHeading}</h2>
            <code>{hasRequestBody ? requestMediaType || "application/json" : "none"}</code>
          </div>
          {requestExample ? (
            <>
              {!requestExampleVerified && <p>Field names and types are inferred. Use the checked notes below to supply valid values.</p>}
              <pre><code>{requestExample}</code></pre>
            </>
          ) : (
            <p className="akoflow-api-empty">
              {hasRequestBody ? "See the request guidance below for a valid body." : "This endpoint does not accept a request body."}
            </p>
          )}
          {handler === "SaveBuildContext" && <p>Upload a context archive as multipart field <code>context</code>. The JSON alternative records metadata only for bytes already in the artifact store.</p>}
        </section>

        {queryParams.length > 0 && (
          <section className="akoflow-api-section">
            <div className="akoflow-api-section-heading"><h2>Query parameters</h2></div>
            <dl className="akoflow-api-parameters">
              {queryParams.map((parameter) => (
                <div key={parameter}><dt><code>{parameter}</code></dt><dd>{queryDescriptions[parameter]}</dd></div>
              ))}
            </dl>
          </section>
        )}
      </main>

      <aside className="akoflow-api-examples">
        {handler === "StreamConsoleSession" ? (
          <section className="akoflow-api-example">
            <div className="akoflow-api-example-title"><span>WebSocket connection</span></div>
            <p>Connect a WebSocket client to <code>ws://&lt;daemon-host&gt;{path.replace("{sessionId}", "<sessionId>")}</code>. Use <code>wss</code> with HTTPS and replace the session ID with the one returned when you opened the console. See the <a href="/docs/guides/operations/interactive-console#stream-protocol">stream protocol guide</a> for authentication and terminal messages.</p>
          </section>
        ) : (
          <section className="akoflow-api-example">
            <div className="akoflow-api-example-title">
              <span>{commandIsTemplate ? "cURL template" : "cURL"}</span>
              <button type="button" onClick={copyCommand}>
                {copied ? "Copied" : commandIsTemplate ? "Copy template" : "Copy"}
              </button>
            </div>
            <pre><code>{command}</code></pre>
            {hasRequestBody && <p>Supply a valid <code>{requestFileName || "request.json"}</code> before running this command.</p>}
            {pathParams.length > 0 && <p>Replace the path identifiers with IDs from your instance.</p>}
          </section>
        )}

        <section className="akoflow-api-example">
          <div className="akoflow-api-example-title akoflow-api-status-tabs">
            <span>{successStatuses[0]}</span>
            <span>{handler === "StreamConsoleSession" ? "WebSocket upgrade" : responseMediaType || "No content"}</span>
          </div>
          {responseExample ? (
            <>
              {responseMediaType === "application/json" && <p>Illustrative response shape; optional fields may be absent and values vary.</p>}
              <pre><code>{responseExample}</code></pre>
            </>
          ) : (
            <p className="akoflow-api-example-empty">
              {responseType === "empty response"
                ? "No response body."
                : responseMediaType === "application/json"
                  ? "Response example unavailable; inspect the returned JSON."
                  : "Response is streamed; no inline sample."}
            </p>
          )}
          {successStatuses.length > 1 && (
            <div className="akoflow-api-response-summary">
              {successStatuses.slice(1).map((status) => <code key={status}>{status}</code>)}
            </div>
          )}
        </section>
      </aside>
    </div>
  );
}
