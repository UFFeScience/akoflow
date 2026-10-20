import React from "react";
import CodeBlock from "@theme/CodeBlock";
import contracts from "../data/showcase-contracts.json";

type LocalContract = {
  examplePath: string;
  sourceCommit?: string;
  localProfile?: string;
  requirements?: string[];
  artifacts?: string[];
  executionMode?: string;
};

export default function WorkflowLocalRun({
  examplePath,
}: {
  examplePath: string;
}) {
  const contract = (Object.values(contracts) as LocalContract[]).find(
    (item) => item.examplePath === examplePath,
  );
  if (!contract) return null;
  const needsDocker =
    examplePath === "rendering/blender-render-farm" ||
    examplePath.startsWith("scientific-workflows/");
  const longRunning = examplePath.startsWith("scientific-workflows/");
  const commands = [
    "git clone https://github.com/UFFeScience/akoflow-examples.git",
    "cd akoflow-examples",
    ...(contract.sourceCommit
      ? [`git checkout --detach ${contract.sourceCommit}`]
      : []),
    `cd ${examplePath}`,
    "./prepare.sh",
    "./run.sh",
  ];
  const command = commands.join(" &&\n");

  return (
    <section>
      <h2>Run this example locally</h2>
      <p>{contract.localProfile}</p>
      <p>Install Go{!longRunning && " and Python 3.12 or newer"}.</p>
      {needsDocker && (
        <p>
          Docker is also required. This command runs the workload inside its
          provided Docker image.
        </p>
      )}
      <CodeBlock language="bash">{command}</CodeBlock>
      <p>
        Run the block from the directory where you keep your repositories. If
        you already cloned the repository, enter this example's directory and
        run <code>./prepare.sh &amp;&amp; ./run.sh</code>.
      </p>
      <p>
        Preparation downloads the Go module and any declared Python packages
        once. The workload then runs locally with offline input data. Every
        meaningful step is a separate process; independent activities run in
        parallel after their dependencies complete.
      </p>
      {Boolean(contract.requirements?.length) && (
        <p>
          This example installs its Python packages in a dedicated
          <code> .venv</code>: {contract.requirements?.join(", ")}.
        </p>
      )}
      <p>
        Read <code>outputs/manifest.json</code> for the generated artifacts,
        <code> outputs/dag-validation.json</code> for dependency waves, and
        <code> outputs/stage-&lt;activity&gt;.json</code> for each activity's
        observed result. A failed stage stops the run and prevents a successful
        final manifest.
      </p>
      {Boolean(contract.artifacts?.length) && (
        <p>Expected outputs: {contract.artifacts?.join(", ")}.</p>
      )}
      <h3>Run the same activities in Desktop</h3>
      <p>
        Start your local AkôFlow daemon with Docker available and configure
        <code> AKOFLOW_API_TOKEN</code> in your shell. From this example's
        directory, run:
      </p>
      <CodeBlock language="bash">{`go run ../../tools/daemon-local -root ../.. -example ${examplePath}${longRunning ? " -wait=false" : ""}`}</CodeBlock>
      <p>
        This command validates the daemon connection, builds the declared image,
        registers the local environment and workflow, and submits a real run.
        The daemon owns execution and file transfers. Open the printed run ID in
        Desktop to inspect each activity.
      </p>
    </section>
  );
}
