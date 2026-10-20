import React from "react";
import useBaseUrl from "@docusaurus/useBaseUrl";
import contracts from "../data/showcase-contracts.json";

type Contract = {
  title: string;
  examplePath: string;
  dependencies: Record<string, string[]>;
};

export default function WorkflowExecutionEvidence({
  title,
  examplePath,
}: {
  title: string;
  examplePath: string;
}) {
  const evidenceBase = useBaseUrl(`/showcase-validation/${examplePath}`);
  const contract = (Object.values(contracts) as Contract[]).find(
    (item) => item.examplePath === examplePath,
  );
  if (!contract) return null;
  return (
    <section>
      <h2>Activity contract validation</h2>
      <p>
        This bundle is a contract fixture. Each activity runs separately;
        independent activities are launched concurrently. Its generated
        artifacts verify orchestration contracts, not model training or an
        external MLflow service.
      </p>
      <p>
        Earlier Desktop screenshots describe an older workflow revision and are
        excluded until new daemon executions are captured. Local fixture
        validation does not verify Desktop execution.
      </p>
      <table>
        <thead>
          <tr>
            <th>Activity</th>
            <th>Dependencies</th>
            <th>Local evidence</th>
          </tr>
        </thead>
        <tbody>
          {Object.entries(contract.dependencies).map(([name, dependencies]) => (
            <tr key={name}>
              <td>
                <code>{name}</code>
              </td>
              <td>{dependencies.length ? dependencies.join(", ") : "None"}</td>
              <td>
                <a href={`${evidenceBase}/stage-${name}.json`}>
                  Activity result
                </a>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <p>
        <a href={`${evidenceBase}/dag-validation.json`}>
          {title}: dependency waves and completed activities
        </a>
      </p>
    </section>
  );
}
