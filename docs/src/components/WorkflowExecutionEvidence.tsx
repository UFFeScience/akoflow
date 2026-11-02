import React from "react";
import useBaseUrl from "@docusaurus/useBaseUrl";
import { findShowcase } from "../data/showcase-contracts";
import ExpandableImage from "./ExpandableImage";
import WorkflowActivityEvidence from "./WorkflowActivityEvidence";
import styles from "./WorkflowExecutionEvidence.module.css";

export default function WorkflowExecutionEvidence({
  title,
  examplePath,
}: {
  title: string;
  examplePath: string;
}) {
  const contract = findShowcase(examplePath);
  const evidenceBase = useBaseUrl(`/showcase/validation/${examplePath}`);
  const graphImage = useBaseUrl(contract?.screenshots?.workflow ?? "");
  if (!contract) return null;
  const complete = contract.validationStatus === "passed";
  const daemonComplete = contract.daemon?.status === "completed";
  const activities = contract.stages.flat();
  const captured = activities.filter((name) => contract.screenshots?.[name]);

  return (
    <section>
      <h2>Execution evidence</h2>
      {daemonComplete ? (
        <p>
          Verified on local Docker: {contract.daemon?.completedActivityCount}/
          {contract.daemon?.activityCount} activities completed. These captures
          belong to run <code>{contract.daemon?.runId}</code>.
        </p>
      ) : (
        <p>
          This long scientific workflow uses the original input data and native
          commands. Its pinned image and command contract are verified. A full
          run can take substantially longer than the small local examples;
          inspect the recorded daemon status before treating its outputs as
          completed.
          {contract.daemon && (
            <>
              {" "}
              Recorded status: <strong>{contract.daemon.status}</strong> for run{" "}
              <code>{contract.daemon.runId}</code>.
            </>
          )}
        </p>
      )}
      {contract.daemon && (
        <p>
          <a href={`${evidenceBase}/daemon-run.json`}>
            Read the daemon execution record
          </a>
          {complete && (
            <>
              {" "}
              ·{" "}
              <a href={`${evidenceBase}/manifest.json`}>
                Local output manifest
              </a>
            </>
          )}
        </p>
      )}
      {contract.screenshots?.workflow && (
        <figure>
          <ExpandableImage
            src={graphImage}
            alt={`${title}: observed activity DAG`}
            loading="lazy"
          />
          <figcaption>
            The observed activity graph. Click to see the complete capture.
          </figcaption>
        </figure>
      )}
      {complete && (
        <p>
          Each stage also ran through the standalone local command. Read the
          <a href={`${evidenceBase}/dag-validation.json`}>
            {" "}
            dependency and artifact validation
          </a>
          .
        </p>
      )}
      <div className={styles.grid}>
        {captured.map((name) => (
          <WorkflowActivityEvidence
            key={name}
            name={name}
            dependencies={contract.dependencies[name]}
            screenshot={contract.screenshots?.[name] ?? ""}
            resultUrl={`/showcase/validation/${examplePath}/stage-${name}.json`}
          />
        ))}
      </div>
      {!complete && (
        <p>
          <a href={`${evidenceBase}/image-contract.json`}>
            Pinned-image and 58-activity contract validation
          </a>
          . Final FITS and PNG outputs are verified by{" "}
          <code>./validate.sh</code>
          after all activities finish.
        </p>
      )}
    </section>
  );
}
