import React, {type ReactNode} from "react";
import Link from "@docusaurus/Link";
import styles from "./WorkflowShowcase.module.css";

type DiagramKind = "edge-cloud" | "fanout" | "kubernetes" | "local" | "parallel" | "slurm";
type DagStage = string | string[];

function Arrow() {
  return <span className={styles.arrow} aria-hidden="true">→</span>;
}

export function WorkflowDag({
  title,
  stages,
}: {
  title: string;
  stages: DagStage[];
}) {
  const description = stages
    .map((stage) => Array.isArray(stage) ? stage.join(" and ") : stage)
    .join(" then ");

  return (
    <figure className={styles.dagFigure}>
      <div className={styles.dag} role="img" aria-label={`${title}: ${description}`}>
        {stages.map((stage, index) => (
          <React.Fragment key={`${title}-${index}`}>
            {index > 0 && <Arrow />}
            {Array.isArray(stage) ? (
              <span className={styles.parallel}>
                {stage.map((node) => <i key={node}>{node}</i>)}
              </span>
            ) : (
              <span className={styles.node}>{stage}</span>
            )}
          </React.Fragment>
        ))}
      </div>
      <figcaption>{title}</figcaption>
    </figure>
  );
}

export function WorkflowPatternDetails({
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
      <section><h2>Inputs</h2><ul>{inputs.map((item) => <li key={item}>{item}</li>)}</ul></section>
      <section><h2>Outputs</h2><ul>{outputs.map((item) => <li key={item}>{item}</li>)}</ul></section>
      <section><h2>Provenance to preserve</h2><ul>{evidence.map((item) => <li key={item}>{item}</li>)}</ul></section>
    </div>
  );
}

export function AIWorkflowPattern({
  title,
  summary,
  stages,
  activities,
  inputs,
  outputs,
  evidence,
  execution,
}: {
  title: string;
  summary: string;
  stages: DagStage[];
  activities?: Array<{name: string; responsibility: string}>;
  inputs: string[];
  outputs: string[];
  evidence: string[];
  execution: string;
}) {
  const documentedActivities = activities ?? stages.flatMap((stage) =>
    (Array.isArray(stage) ? stage : [stage]).map((name) => ({
      name: name.toLowerCase().replaceAll(" ", "-"),
      responsibility: `Execute the ${name} stage and publish its declared outputs for downstream activities.`,
    })),
  );

  return (
    <>
      <p className={styles.patternLead}>{summary}</p>
      <WorkflowDag title={title} stages={stages} />
      <h2>Activity responsibilities</h2>
      <div className={styles.activityTable}>
        <table>
          <thead><tr><th>Activity</th><th>Responsibility</th></tr></thead>
          <tbody>{documentedActivities.map((activity) => (
            <tr key={activity.name}><td><code>{activity.name}</code></td><td>{activity.responsibility}</td></tr>
          ))}</tbody>
        </table>
      </div>
      <WorkflowPatternDetails inputs={inputs} outputs={outputs} evidence={evidence} />
      <h2>Execution considerations</h2>
      <p>{execution}</p>
      <p className={styles.patternNote}><strong>AkôFlow boundary:</strong> the engine schedules, deploys, executes, transfers data, and records evidence. The ML or agent framework remains an implementation choice inside each activity.</p>
    </>
  );
}

export function WorkflowDiagram({kind}: {kind: DiagramKind}) {
  if (kind === "parallel") {
    return (
      <div className={styles.fanout} role="img" aria-label="One hundred independent activities distributed across fifty cores">
        <span className={styles.node}>100 tasks</span><Arrow />
        <span className={styles.parallel}><i>core 1</i><i>core 2…49</i><i>core 50</i></span>
      </div>
    );
  }
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

  if (kind === "local") {
    return (
      <div className={styles.diagram} role="img" aria-label="One write-report activity running directly on the daemon host">
        <span className={styles.node}>write-report</span><Arrow />
        <span className={styles.target}>daemon host</span>
        <span className={styles.route}>result.txt</span>
      </div>
    );
  }

  if (kind === "slurm") {
    return (
      <div className={styles.diagram} role="img" aria-label="One write-report activity submitted through the local SLURM fixture">
        <span className={styles.node}>write-report</span><Arrow />
        <span className={styles.target}>sbatch fixture</span>
        <span className={styles.route}>result.txt</span>
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
