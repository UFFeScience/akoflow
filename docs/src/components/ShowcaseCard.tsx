import React, { type ReactNode } from "react";
import Link from "@docusaurus/Link";
import styles from "./WorkflowShowcase.module.css";
import WorkflowDiagram from "./WorkflowDiagram";
type DiagramKind =
  "edge-cloud" | "fanout" | "kubernetes" | "local" | "parallel" | "slurm";
export default function ShowcaseCard({
  title,
  description,
  href,
  kind,
  tags,
}: {
  title: string;
  description: string;
  href: string;
  kind: DiagramKind;
  tags: string[];
}) {
  return (
    <Link className={styles.card} to={href}>
      <WorkflowDiagram kind={kind} />
      <div className={styles.cardBody}>
        <div className={styles.tags}>
          {tags.map((tag) => (
            <span key={tag}>{tag}</span>
          ))}
        </div>
        <h2>{title}</h2>
        <p>{description}</p>
        <strong>
          Open walkthrough <span aria-hidden="true">→</span>
        </strong>
      </div>
    </Link>
  );
}
