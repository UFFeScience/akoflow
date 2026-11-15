import React from "react";
import CodeBlock from "@theme/CodeBlock";
import publications from "../data/showcase-hub.json";

type Publication = {
  examplePath: string;
  url: string;
  uri: string;
  publicationId: string;
  version: number;
  experimentId: string;
};

export default function WorkflowHubLinks({
  examplePath,
}: {
  examplePath: string;
}) {
  const canonicalPath =
    examplePath === "connections/montage-cloud-local-profile"
      ? "scientific-workflows/montage-050"
      : examplePath;
  const publication = (publications as Publication[]).find(
    (item) => item.examplePath === canonicalPath,
  );
  if (!publication) return null;
  return (
    <section aria-label="Published workflow and execution results">
      <h2>Import this workflow from AkôFlow Hub</h2>
      <p>
        <a href={publication.url}>
          Open the public README and measured execution results
        </a>
        . The Hub includes the execution capture, activity metrics, logs and
        version history. Reading public results does not require an account.
      </p>
      <p>
        In Desktop, choose Create workflow → Import from Hub and paste this
        version-specific URI:
      </p>
      <CodeBlock language="text">{publication.uri}</CodeBlock>
      <p>
        <a
          href={`https://hub.akoflow.com/api/v1/publications/${publication.publicationId}/versions/${publication.version}/yaml`}
        >
          Download the exact workflow YAML (v{publication.version})
        </a>
        . Configure the runtime prerequisites below before planning and
        executing it.{" "}
        <a
          href={`https://github.com/UFFeScience/akoflow-examples/tree/showcase-hub-v1/${canonicalPath}`}
        >
          Open the versioned source bundle
        </a>
        .
      </p>
      <p>
        <a
          href={`https://hub.akoflow.com/api/v1/publications/${publication.publicationId}/experiments/${publication.experimentId}`}
        >
          Download the native experiment capture
        </a>
        {" · "}
        <a
          href={`https://github.com/UFFeScience/akoflow-examples/releases/download/showcase-hub-v1/${publication.url.split("/").pop()}-results.tar.gz`}
        >
          Download the validated output files
        </a>
      </p>
    </section>
  );
}
