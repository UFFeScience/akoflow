import React from "react";
import CodeBlock from "@theme/CodeBlock";
import publications from "../data/showcase-hub.json";

type Publication = {
  examplePath: string;
  url: string;
  uri: string;
  publicationId: string;
  version: number;
};

export default function WorkflowHubLinks({
  examplePath,
}: {
  examplePath: string;
}) {
  const publication = (publications as Publication[]).find(
    (item) => item.examplePath === examplePath,
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
        executing it.
      </p>
    </section>
  );
}
