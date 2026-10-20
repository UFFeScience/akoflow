import WorkflowLocalRun from "./WorkflowLocalRun";
import WorkflowPatternDetails from "./WorkflowPatternDetails";
import React from "react";
import { findShowcase } from "../data/showcase-contracts";
import WorkflowExecutionEvidence from "./WorkflowExecutionEvidence";
import WorkflowDag from "./WorkflowDag";
import styles from "./WorkflowShowcase.module.css";

type DagStage = string | string[];

export default function AIWorkflowPattern({
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
  activities?: Array<{ name: string; responsibility: string }>;
  inputs: string[];
  outputs: string[];
  evidence: string[];
  execution: string;
}) {
  const documentedActivities =
    activities ??
    stages.flatMap((stage) =>
      (Array.isArray(stage) ? stage : [stage]).map((name) => ({
        name: name.toLowerCase().replaceAll(" ", "-"),
        responsibility: `Execute the ${name} stage and publish its declared outputs for downstream activities.`,
      })),
    );
  const contract = findShowcase(title);
  const examplePath = contract?.examplePath;
  const exampleRepository = `https://github.com/UFFeScience/akoflow-examples/tree/${contract?.sourceCommit ?? "main"}/${examplePath}`;

  return (
    <>
      <p className={styles.patternLead}>{summary}</p>
      {examplePath && <WorkflowLocalRun examplePath={examplePath} />}
      <WorkflowDag title={title} stages={stages} />
      <h2>Activity responsibilities</h2>
      <div className={styles.activityTable}>
        <table>
          <thead>
            <tr>
              <th>Activity</th>
              <th>Responsibility</th>
            </tr>
          </thead>
          <tbody>
            {documentedActivities.map((activity) => (
              <tr key={activity.name}>
                <td>
                  <code>{activity.name}</code>
                </td>
                <td>{activity.responsibility}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <WorkflowPatternDetails
        inputs={inputs}
        outputs={outputs}
        evidence={evidence}
      />
      {examplePath && (
        <section className={styles.exampleBundle}>
          <h2>Source bundle</h2>
          <p>
            The executable workflow and its implementation files are maintained
            in the separate AkôFlow examples repository. The documentation
            intentionally keeps code, container recipes, generators, and
            validation scripts out of the narrative page.
          </p>
          <a href={exampleRepository}>Open the complete source bundle ↗</a>
        </section>
      )}
      {examplePath && (
        <WorkflowExecutionEvidence title={title} examplePath={examplePath} />
      )}
      <h2>Execution considerations</h2>
      <p>{execution}</p>
      <p className={styles.patternNote}>
        <strong>AkôFlow boundary:</strong> the engine schedules, deploys,
        executes, transfers data, and records evidence. The ML or agent
        framework remains an implementation choice inside each activity.
      </p>
    </>
  );
}
