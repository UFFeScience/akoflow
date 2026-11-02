import React, { type ReactNode } from "react";
import Link from "@docusaurus/Link";
import styles from "./WorkflowShowcase.module.css";
export default function WorkflowPatternDetails({
  inputs,
  outputs,
  evidence,
}: {
  inputs: string[];
  outputs: string[];
  evidence: string[];
}) {
  return (
    <div className={styles.detailGrid}>
      <section>
        <h2>Inputs</h2>
        <ul>
          {inputs.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </section>
      <section>
        <h2>Outputs</h2>
        <ul>
          {outputs.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </section>
      <section>
        <h2>Provenance to preserve</h2>
        <ul>
          {evidence.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </section>
    </div>
  );
}
