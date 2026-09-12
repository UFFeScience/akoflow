import React, { useState } from "react";

type ApiEndpointProps = {
  method: string;
  path: string;
  handler: string;
  group: string;
  pathParams?: string[];
  queryParams?: string[];
  successStatuses?: string[];
  requestExample?: string | null;
  requestType?: string;
  responseExample?: string | null;
  responseType?: string;
  responseMediaType?: string | null;
  hasRequestBody?: boolean;
  requestMediaType?: string | null;
  requestFileName?: string | null;
};

function fieldsFromExample(example: string | null) {
  if (!example) return [];
  try {
    const value = JSON.parse(example);
    if (!value || Array.isArray(value) || typeof value !== "object") return [];
    return Object.entries(value).map(([name, fieldValue]) => ({
      name,
      type: Array.isArray(fieldValue)
        ? "array"
        : fieldValue === null
          ? "null"
          : typeof fieldValue === "object"
            ? "object"
            : typeof fieldValue,
    }));
  } catch {
    return [];
  }
}

function commandFor(
  method: string,
  endpointPath: string,
  hasRequestBody: boolean,
  requestMediaType: string | null,
  requestFileName: string | null,
) {
  const lines = [
    `curl --fail-with-body \\`,
    `  -H "Authorization: Bearer \${AKOFLOW_API_TOKEN}" \\`,
  ];
  if (hasRequestBody) {
    lines.push(`  -H "Content-Type: ${requestMediaType || "application/json"}" \\`);
    lines.push(`  --data-binary @${requestFileName || "request.json"} \\`);
  }
  if (method !== "GET") lines.push(`  -X ${method} \\`);
  lines.push(
    `  "\${AKOFLOW_API_URL}${endpointPath.replace("/akoflow-api", "")}"`,
  );
  return lines.join("\n");
}

export default function ApiEndpoint({
  method,
  path,
  handler,
  group,
  pathParams = [],
  queryParams = [],
  successStatuses = ["200 OK"],
  requestExample = null,
  requestType = "No request body",
  responseExample = null,
  responseType = "JSON object",
  responseMediaType = "application/json",
  hasRequestBody = false,
  requestMediaType = null,
  requestFileName = null,
}: ApiEndpointProps) {
  const [copied, setCopied] = useState(false);
  const displayPath = path.replace("/akoflow-api", "") || "/";
  const description = handler.replace(/([a-z0-9])([A-Z])/g, "$1 $2");
  const command = commandFor(method, path, hasRequestBody, requestMediaType, requestFileName);
  const requestFields = fieldsFromExample(requestExample);

  async function copyCommand() {
    await navigator.clipboard.writeText(command);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  }

  return (
    <div className="akoflow-api-reference">
      <main className="akoflow-api-main">
        <span className="akoflow-api-eyebrow">{group} endpoint</span>
        <h1>{description}</h1>
        <p className="akoflow-api-lead">
          Send this request to the AkôFlow daemon. Authentication is required
          when an API token is configured.
        </p>
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
            <h2>Body</h2>
            <code>{hasRequestBody ? requestMediaType || "application/json" : "none"}</code>
          </div>
          {requestFields.length > 0 ? (
            <dl className="akoflow-api-fields">
              {requestFields.map((field) => (
                <div key={field.name}>
                  <dt><code>{field.name}</code> <span>{field.type}</span></dt>
                  <dd>Illustrative field from <code>{requestType}</code>; check required values in the related guide.</dd>
                </div>
              ))}
            </dl>
          ) : (
            <p className="akoflow-api-empty">
              {hasRequestBody ? <>Body contract: <code>{requestType}</code>.</> : "This endpoint does not accept a request body."}
            </p>
          )}
          {requestExample && <><p>Illustrative JSON shape; replace sample values and check the related guide for required fields.</p><pre><code>{requestExample}</code></pre></>}
          {handler === "SaveBuildContext" && <p>Alternatively, upload a file as multipart field <code>context</code>. See the Artifacts guide for the upload procedure.</p>}
        </section>

        {queryParams.length > 0 && (
          <section className="akoflow-api-section">
            <div className="akoflow-api-section-heading"><h2>Query parameters</h2></div>
            <dl className="akoflow-api-parameters">
              {queryParams.map((parameter) => (
                <div key={parameter}><dt><code>{parameter}</code></dt><dd>Optional query value consumed by this route.</dd></div>
              ))}
            </dl>
          </section>
        )}
      </main>

      <aside className="akoflow-api-examples">
        <section className="akoflow-api-example">
          <div className="akoflow-api-example-title">
            <span>cURL</span>
            <button type="button" onClick={copyCommand}>
              {copied ? "Copied" : "Copy"}
            </button>
          </div>
          <pre><code>{command}</code></pre>
          {hasRequestBody && <p>Supply a valid <code>{requestFileName || "request.json"}</code> before running this command.</p>}
        </section>

        <section className="akoflow-api-example">
          <div className="akoflow-api-example-title akoflow-api-status-tabs">
            <span>{successStatuses[0]}</span>
            <span>{responseMediaType || "No content"}</span>
          </div>
          {responseExample ? (
            <pre><code>{responseExample}</code></pre>
          ) : (
            <p className="akoflow-api-example-empty">No response body.</p>
          )}
          <div className="akoflow-api-response-summary">
            <span>{responseType}</span>
            {successStatuses.slice(1).map((status) => <code key={status}>{status}</code>)}
          </div>
        </section>
      </aside>
    </div>
  );
}
