import React from "react";
import useBaseUrl from "@docusaurus/useBaseUrl";
import ExpandableImage from "./ExpandableImage";
import styles from "./WorkflowExecutionEvidence.module.css";

export default function WorkflowActivityEvidence({
  name,
  dependencies,
  screenshot,
  resultUrl,
}: {
  name: string;
  dependencies: string[];
  screenshot: string;
  resultUrl: string;
}) {
  const image = useBaseUrl(screenshot);
  const result = useBaseUrl(resultUrl);
  return (
    <section className={styles.activity}>
      <h3>
        <code>{name}</code>
      </h3>
      <p className={styles.dependencies}>
        After:{" "}
        {dependencies.length ? dependencies.join(", ") : "workflow start"}.
      </p>
      <ExpandableImage
        src={image}
        alt={`Execution record for activity ${name}`}
        loading="lazy"
      />
      <p>
        <a href={result}>Read the measured local activity result</a>
      </p>
    </section>
  );
}
