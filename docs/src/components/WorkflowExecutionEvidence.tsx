import React from "react";
import publications from "../data/showcase-hub.json";
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
  const publication = publications.find(
    (item) => item.examplePath === examplePath,
  );
  const complete = contract.validationStatus === "passed";
  const daemonComplete = contract.daemon?.status === "completed";
  const activities = contract.stages.flat();
  const captured = activities.filter((name) => contract.screenshots?.[name]);

  return (
    <section>
      <h2>Execution evidence</h2>
      {publication ? (
        <p>
          Latest validated Docker execution: {publication.activityCount} /{" "}
          {publication.activityCount} activities completed.{" "}
          <a href={`${publication.url}#publication-results`}>
            Open the measured run, logs, metrics and output downloads in Hub
          </a>
          . The documentation screenshots below are historical reference
          captures; they are separate from the latest Hub execution.
        </p>
      ) : daemonComplete ? (
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
            Read the historical documentation execution record
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
            The reference activity graph from the documentation capture. Click
            to see the complete image.
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
      {!complete && !publication && (
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
