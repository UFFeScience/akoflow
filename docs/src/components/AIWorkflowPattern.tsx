import WorkflowPatternDetails from "./WorkflowPatternDetails";
import React from "react";
import WorkflowExecutionEvidence from "./WorkflowExecutionEvidence";
import WorkflowDag from "./WorkflowDag";
import styles from "./WorkflowShowcase.module.css";

type DagStage = string | string[];

const runnableExamplePaths: Record<string, string> = {
  "Training with MLflow": "machine-learning/training-with-mlflow",
  "Hyperparameter search": "machine-learning/hyperparameter-search",
  "LoRA fine-tuning": "machine-learning/lora-fine-tuning",
  "Batch inference": "machine-learning/batch-inference",
  "Comparative model evaluation": "machine-learning/model-comparison",
  "Model conversion and quantization": "machine-learning/model-conversion",
  "Computer vision training": "machine-learning/computer-vision",
  "Anomaly detection": "machine-learning/anomaly-detection",
  "Model ensemble": "machine-learning/model-ensemble",
  "Model distillation": "machine-learning/model-distillation",
  "Inference benchmark": "machine-learning/inference-benchmark",
  "Infrastructure selection for training":
    "machine-learning/infrastructure-selection",
  "RAG index construction and evaluation": "generative-ai/rag-indexing",
  "Synthetic dataset generation": "generative-ai/synthetic-dataset",
  "Audio transcription and summarization": "generative-ai/audio-transcription",
  "OCR and document extraction": "generative-ai/document-extraction",
  "Encapsulated agent task": "agentic-workflows/bounded-agent",
  "Supervised tool loop": "agentic-workflows/supervised-tool-loop",
  "Multi-agent review": "agentic-workflows/multi-agent-review",
  "Scientific pipeline with AI interpretation":
    "scientific-ai/ai-assisted-analysis",
  "Surrogate model construction": "scientific-ai/surrogate-model",
  "Model-guided experiment cycle": "scientific-ai/experiment-feedback-loop",
};

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
  const examplePath = runnableExamplePaths[title];
  const exampleRepository = `https://github.com/UFFeScience/akoflow-examples/tree/main/${examplePath}`;

  return (
    <>
      <p className={styles.patternLead}>{summary}</p>
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
