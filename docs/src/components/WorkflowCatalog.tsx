import React from "react";
import Link from "@docusaurus/Link";
import styles from "./WorkflowCatalog.module.css";

type Stage = string | string[];
type Example = {title: string; href: string; stages: Stage[]; label?: string};
type Section = {title: string; description: string; examples: Example[]};

const sections: Section[] = [
  {
    title: "Simple examples",
    description: "Simulation, real execution, and adapter fixtures. Each page states its prerequisites and validation scope.",
    examples: [
      {title: "Edge to cloud simulation", href: "/docs/showcase/edge-cloud-simulation", stages: ["prepare · edge", "analyze · cloud", "summarize · edge"], label: "SimGrid"},
      {title: "Kubernetes real execution", href: "/docs/showcase/kubernetes-real-execution", stages: ["prepare · Kind", "process · Kind"], label: "Real run"},
      {title: "Hybrid local or Kind to GCP", href: "/docs/showcase/hybrid-cloud-transfer", stages: ["produce · local / Kind", "handoff.txt", "consume · GCP"], label: "Real run"},
      {title: "Montage 58: GCP to local", href: "/docs/showcase/montage-cloud-local", stages: [["12 mProject · GCP e2-medium"], "projected FITS", "46 activities · local", "mosaic-color.png"], label: "Real workflow"},
      {title: "Local direct execution", href: "/docs/showcase/local-direct-execution", stages: ["write report · local", "result.txt"], label: "Real run"},
      {title: "SLURM batch fixture", href: "/docs/showcase/slurm-local-fixture", stages: ["write report", "sbatch fixture", "result.txt"], label: "Local fixture"},
      {title: "30 GB network fan-out", href: "/docs/showcase/network-fanout", stages: ["producer", ["worker 1", "worker 2", "worker 3"], "consumer"], label: "SimGrid"},
      {title: "100 activities on 50 cores", href: "/docs/showcase/parallel-50-core", stages: ["100 activities", ["core 1", "cores 2–49", "core 50"]], label: "SimGrid"},
    ],
  },
  {
    title: "Machine Learning",
    description: "Training, tuning, inference, evaluation, and model delivery patterns. These are DAG designs, not verified executable bundles.",
    examples: [
      {title: "Training with MLflow", href: "/docs/showcase/ai/machine-learning/training-with-mlflow", stages: ["ingest data", "validate", "train", "evaluate", ["register model", "log MLflow run"]]},
      {title: "Hyperparameter search", href: "/docs/showcase/ai/machine-learning/hyperparameter-search", stages: ["prepare dataset", ["trial 1", "trial 2", "trial N"], "compare trials", "register best"]},
      {title: "LoRA fine-tuning", href: "/docs/showcase/ai/machine-learning/lora-fine-tuning", stages: ["fetch base model", "prepare examples", "fine-tune adapters", "evaluate", ["publish adapter", "record lineage"]]},
      {title: "Batch inference", href: "/docs/showcase/ai/machine-learning/batch-inference", stages: ["load model", "partition input", ["infer shard A", "infer shard B", "infer shard N"], "merge predictions", "publish"]},
      {title: "Comparative model evaluation", href: "/docs/showcase/ai/machine-learning/model-comparison", stages: ["freeze evaluation set", ["evaluate A", "evaluate B", "evaluate C"], "compare metrics", "approve candidate"]},
      {title: "Model conversion and quantization", href: "/docs/showcase/ai/machine-learning/model-conversion", stages: ["load model", ["export ONNX", "export TorchScript"], ["quantize CPU", "quantize GPU"], "validate outputs", "publish variants"]},
      {title: "Computer vision training", href: "/docs/showcase/ai/machine-learning/computer-vision", stages: ["collect images", "import labels", "augment", "train detector", "evaluate", "package model"]},
      {title: "Anomaly detection", href: "/docs/showcase/ai/machine-learning/anomaly-detection", stages: ["collect signals", "build features", "train detector", "score validation", "choose threshold", "publish"]},
      {title: "Model ensemble", href: "/docs/showcase/ai/machine-learning/model-ensemble", stages: ["prepare input", ["predict A", "predict B", "predict C"], "combine", "evaluate", "publish"]},
      {title: "Model distillation", href: "/docs/showcase/ai/machine-learning/model-distillation", stages: ["load teacher", "soft labels", "train student", ["quality", "latency"], "publish"]},
      {title: "Inference benchmark", href: "/docs/showcase/ai/machine-learning/inference-benchmark", stages: ["freeze inputs", ["CPU", "GPU", "edge"], "normalize", "compare cost and latency"]},
      {title: "Infrastructure selection for training", href: "/docs/showcase/ai/machine-learning/infrastructure-selection", stages: ["profile workload", ["local GPU", "HPC", "cloud"], "compare plans", "select scope", "train"]},
    ],
  },
  {
    title: "Generative AI",
    description: "Data preparation and evaluation patterns for retrieval, generation, audio, and documents.",
    examples: [
      {title: "RAG index construction and evaluation", href: "/docs/showcase/ai/generative-ai/rag-indexing", stages: ["collect documents", "clean", "chunk", "embed", "build index", ["retrieval tests", "answer tests"], "publish"]},
      {title: "Synthetic dataset generation", href: "/docs/showcase/ai/generative-ai/synthetic-dataset", stages: ["define schema", ["batch A", "batch B", "batch N"], "filter", "deduplicate", "quality review", "publish"]},
      {title: "Audio transcription and summarization", href: "/docs/showcase/ai/generative-ai/audio-transcription", stages: ["ingest audio", "segment", ["segment 1", "segment N"], "merge transcript", "summarize", "publish"]},
      {title: "OCR and document extraction", href: "/docs/showcase/ai/generative-ai/document-extraction", stages: ["ingest documents", ["OCR", "native text"], "classify", "extract fields", "validate schema", "publish"]},
    ],
  },
  {
    title: "Agentic workflows",
    description: "Bounded agent execution with explicit review, policy, and evidence stages.",
    examples: [
      {title: "Encapsulated agent task", href: "/docs/showcase/ai/agentic-workflows/bounded-agent", stages: ["prepare objective", "agent loop", "validate output", ["publish result", "store trace"]]},
      {title: "Supervised tool loop", href: "/docs/showcase/ai/agentic-workflows/supervised-tool-loop", stages: ["build context", "propose action", "policy check", "human approval", "execute tool", "verify"]},
      {title: "Multi-agent review", href: "/docs/showcase/ai/agentic-workflows/multi-agent-review", stages: ["shared context", ["research", "code", "domain"], "critic review", "synthesize", "publish evidence"]},
    ],
  },
  {
    title: "Scientific AI",
    description: "Simulation, analysis, surrogate models, and experiment feedback loops.",
    examples: [
      {title: "Scientific pipeline with AI interpretation", href: "/docs/showcase/ai/scientific-ai/ai-assisted-analysis", stages: ["prepare experiment", "simulation", "measurements", ["statistics", "AI interpretation"], "validate", "report"]},
      {title: "Surrogate model construction", href: "/docs/showcase/ai/scientific-ai/surrogate-model", stages: ["design samples", ["simulation A", "simulation N"], "assemble dataset", "train surrogate", "validate", "publish"]},
      {title: "Model-guided experiment cycle", href: "/docs/showcase/ai/scientific-ai/experiment-feedback-loop", stages: ["observations", "update model", "propose experiments", "safety check", "run experiments", "record observations"]},
    ],
  },
];

function Preview({stages, title}: {stages: Stage[]; title: string}) {
  return (
    <div className={styles.preview} role="img" aria-label={`${title}: ${stages.map((stage) => Array.isArray(stage) ? stage.join(" and ") : stage).join(" then ")}`}>
      {stages.map((stage, index) => (
        <React.Fragment key={`${title}-${index}`}>
          {index > 0 && <span className={styles.arrow} aria-hidden="true">↓</span>}
          {Array.isArray(stage) ? (
            <div className={styles.parallel}>{stage.map((name) => <span key={name}>{name}</span>)}</div>
          ) : <span className={styles.node}>{stage}</span>}
        </React.Fragment>
      ))}
    </div>
  );
}

export default function WorkflowCatalog(): React.JSX.Element {
  return (
    <div className={styles.catalog}>
      {sections.map((section) => (
        <section key={section.title} className={styles.section}>
          <div className={styles.sectionHeading}>
            <h2>{section.title}</h2>
            <p>{section.description}</p>
          </div>
          <div className={styles.grid}>
            {section.examples.map((example) => (
              <Link className={styles.card} to={example.href} key={example.href} aria-label={`Open ${example.title}`}>
                <Preview stages={example.stages} title={example.title} />
                <div className={styles.cardFooter}>
                  <div>
                    {example.label && <small>{example.label}</small>}
                    <h3>{example.title}</h3>
                  </div>
                  <span aria-hidden="true">↗</span>
                </div>
              </Link>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
