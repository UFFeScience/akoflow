import React, { useRef } from "react";
import styles from "./ExpandableImage.module.css";
import contracts from "../data/showcase-contracts.json";
import Mermaid from "@theme/Mermaid";

type Stage = string | string[];

export default function WorkflowDag({
  title,
  stages,
}: {
  title: string;
  stages: Stage[];
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  function expandDiagram() {
    dialog.current?.showModal();
  }
  function closeDiagram() {
    dialog.current?.close();
  }
  const contract = Object.values(contracts).find(
    (item) => item.title === title,
  );
  const groups = (contract?.stages ?? stages).map((stage) =>
    Array.isArray(stage) ? stage : [stage],
  );
  const lines = [
    "flowchart LR",
    "classDef activity fill:#ffffff,stroke:#151515,color:#151515,rx:8,ry:8",
    "linkStyle default stroke:#151515,stroke-width:2px,color:#151515",
  ];
  groups.forEach((group, index) => {
    group.forEach((name, node) => {
      const label = name.replaceAll('"', "&quot;");
      lines.push(`n${index}_${node}["${label}"]:::activity`);
      if (index > 0) {
        groups[index - 1].forEach((_, previous) => {
          lines.push(`n${index - 1}_${previous} --> n${index}_${node}`);
        });
      }
    });
  });
  return (
    <figure aria-label={`${title} activity DAG`}>
      <button
        type="button"
        className={styles.thumbnail}
        onClick={expandDiagram}
        aria-label={`Enlarge DAG: ${title}`}
      >
        <Mermaid value={lines.join("\n")} />
      </button>
      <dialog ref={dialog} className={styles.dialog}>
        <button
          type="button"
          className={styles.close}
          onClick={closeDiagram}
          autoFocus
        >
          Close ×
        </button>
        <div className={styles.largeDiagram}>
          <Mermaid value={lines.join("\n")} />
        </div>
      </dialog>
      <figcaption>
        {title}. Nodes in the same column can run in parallel once their
        dependencies complete.
      </figcaption>
    </figure>
  );
}
