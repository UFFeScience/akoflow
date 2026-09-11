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
};

function commandFor(
  method: string,
  endpointPath: string,
  hasRequestBody: boolean,
) {
  const lines = [
    `curl --fail-with-body \\`,
    `  -H "Authorization: Bearer \${AKOFLOW_API_TOKEN}" \\`,
  ];
  if (hasRequestBody) {
    lines.push(`  -H "Content-Type: application/json" \\`);
    lines.push(`  --data-binary @request.json \\`);
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
}: ApiEndpointProps) {
  const [copied, setCopied] = useState(false);
  const displayPath = path.replace("/akoflow-api", "") || "/";
  const description = handler.replace(/([a-z0-9])([A-Z])/g, "$1 $2");
  const command = commandFor(method, path, hasRequestBody);

  async function copyCommand() {
    await navigator.clipboard.writeText(command);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  }

  return (
    <div className="akoflow-api-reference">
      <main>
        <span className="akoflow-api-eyebrow">{group} endpoint</span>
        <h1>{description}</h1>
        <div className="akoflow-endpoint-signature">
          <span
            className={`akoflow-method akoflow-method-${method.toLowerCase()}`}
          >
            {method}
          </span>
          <code>{displayPath}</code>
        </div>

        <h2>Request</h2>
        <p>
          Send this request to the AkôFlow daemon. When a token is configured,
          all endpoints except the public bootstrap reads require bearer
          authentication.
        </p>

        {pathParams.length > 0 && (
          <>
            <h3>Path parameters</h3>
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
          </>
        )}

        <h3>Body</h3>
        <p>
          <code>{requestType}</code>
        </p>
        {hasRequestBody && requestExample ? (
          <pre className="akoflow-api-payload">
            <code>{requestExample}</code>
          </pre>
        ) : (
          <p>No request body.</p>
        )}

        <h2>Response</h2>
        <p>
          <code>{responseMediaType || "no content"}</code> · {responseType}
        </p>
        {responseExample ? (
          <pre className="akoflow-api-payload">
            <code>{responseExample}</code>
          </pre>
        ) : (
          <p>No JSON response body.</p>
        )}
      </main>

      <aside className="akoflow-api-example">
        <div className="akoflow-api-example-title">
          <span>cURL</span>
          <button type="button" onClick={copyCommand}>
            {copied ? "Copied" : "Copy"}
          </button>
        </div>
        <pre>
          <code>{command}</code>
        </pre>
        <div className="akoflow-api-example-title">Endpoint</div>
        <pre>
          <code>{`${method} ${path}`}</code>
        </pre>
        <div className="akoflow-api-example-title">Response</div>
        <div className="akoflow-api-response-summary">
          {successStatuses.map((status) => (
            <code key={status}>{status}</code>
          ))}
          <span>
            {queryParams.length > 0
              ? `${queryParams.length} query parameter${queryParams.length === 1 ? "" : "s"}`
              : "No documented query parameters"}
          </span>
        </div>
        {responseExample && (
          <pre>
            <code>{responseExample}</code>
          </pre>
        )}
      </aside>
    </div>
  );
}
