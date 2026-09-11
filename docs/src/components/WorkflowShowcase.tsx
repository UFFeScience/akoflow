import React, {type ReactNode} from "react";
import Link from "@docusaurus/Link";
import styles from "./WorkflowShowcase.module.css";

type DiagramKind = "edge-cloud" | "fanout" | "kubernetes";

function Arrow() {
  return <span className={styles.arrow} aria-hidden="true">→</span>;
}

export function WorkflowDiagram({kind}: {kind: DiagramKind}) {
  if (kind === "fanout") {
    return (
      <div className={styles.fanout} role="img" aria-label="One producer, three parallel workers, and one consumer">
        <span className={styles.node}>t1</span><Arrow />
        <span className={styles.parallel}><i>t2</i><i>t3</i><i>t4</i></span>
        <Arrow /><span className={styles.node}>t5</span>
      </div>
    );
  }

  if (kind === "kubernetes") {
    return (
      <div className={styles.diagram} role="img" aria-label="Prepare and process activities running on Kubernetes">
        <span className={styles.node}>prepare</span><Arrow />
        <span className={styles.node}>process</span><Arrow />
        <span className={styles.target}>Kubernetes</span>
      </div>
    );
  }

  return (
    <div className={styles.diagram} role="img" aria-label="Prepare, analyze, and summarize workflow across edge and cloud resources">
      <span className={styles.node}>prepare</span><Arrow />
      <span className={styles.node}>analyze</span><Arrow />
      <span className={styles.node}>summarize</span>
      <span className={styles.route}>edge · cloud · edge</span>
    </div>
  );
}

export function ShowcaseGrid({children}: {children: ReactNode}) {
  return <div className={styles.grid}>{children}</div>;
}

export function ShowcaseCard({
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
        <div className={styles.tags}>{tags.map((tag) => <span key={tag}>{tag}</span>)}</div>
        <h2>{title}</h2>
        <p>{description}</p>
        <strong>Open walkthrough <span aria-hidden="true">→</span></strong>
      </div>
    </Link>
  );
}

export function FileList({children}: {children: ReactNode}) {
  return <div className={styles.files}>{children}</div>;
}
