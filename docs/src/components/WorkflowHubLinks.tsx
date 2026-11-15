import React from "react";
import CodeBlock from "@theme/CodeBlock";
import publications from "../data/showcase-hub.json";
import styles from "./WorkflowHubLinks.module.css";

type Publication = {
  examplePath: string;
  url: string;
  uri: string;
  publicationId: string;
  version: number;
  experimentId: string;
  sourceRef?: string;
  releaseTag?: string;
  activityCount?: number;
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
  const sourceRef = publication.sourceRef ?? "showcase-hub-v1";
  const releaseTag = publication.releaseTag ?? "showcase-hub-v1";
  return (
    <section
      className={styles.panel}
      aria-label="Live workflow and results from AkôFlow Hub"
    >
      <header className={styles.header}>
        <div>
          <span className={styles.eyebrow}>PUBLISHED ON AKÔFLOW HUB</span>
          <h2>Explore this workflow and its measured results</h2>
          <p>
            The interactive DAG, version and execution summary below come
            directly from the public Hub publication.
          </p>
        </div>
        <a
          className={styles.open}
          href={publication.url}
          target="_blank"
          rel="noopener noreferrer"
        >
          Open full experiment ↗
        </a>
      </header>
      <iframe
        className={styles.widget}
        src={`https://hub.akoflow.com/embed/${publication.publicationId}/${publication.version}`}
        title={`AkôFlow Hub workflow DAG and results: ${examplePath}, version ${publication.version}`}
        loading="lazy"
        referrerPolicy="strict-origin-when-cross-origin"
        sandbox="allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox allow-downloads"
        allow="clipboard-write; fullscreen"
        allowFullScreen
      />
      <details className={styles.downloads}>
        <summary>
          Import this exact version and download the research data
        </summary>
        <p>
          In Desktop, choose <strong>Create workflow → Import from Hub</strong>{" "}
          and paste this immutable URI:
        </p>
        <CodeBlock language="text">{publication.uri}</CodeBlock>
        <nav aria-label="Workflow data downloads">
          <a
            href={`https://hub.akoflow.com/api/v1/publications/${publication.publicationId}/versions/${publication.version}/yaml`}
          >
            Workflow YAML · v{publication.version}
          </a>
          <a
            href={`https://hub.akoflow.com/api/v1/publications/${publication.publicationId}/experiments/${publication.experimentId}`}
          >
            Native experiment capture
          </a>
          <a
            href={`https://github.com/UFFeScience/akoflow-examples/tree/${sourceRef}/${canonicalPath}`}
          >
            Versioned source bundle
          </a>
          <a
            href={`https://github.com/UFFeScience/akoflow-examples/releases/download/${releaseTag}/${publication.url.split("/").pop()}-results.tar.gz`}
          >
            Validated output files
          </a>
        </nav>
      </details>
    </section>
  );
}
