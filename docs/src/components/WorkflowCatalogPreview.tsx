import React from "react";
import styles from "./WorkflowCatalog.module.css";
type Stage = string | string[];
export default function WorkflowCatalogPreview({
  stages,
  title,
}: {
  stages: Stage[];
  title: string;
}) {
  return (
    <div
      className={styles.preview}
      role="img"
      aria-label={`${title}: ${stages.map((stage) => (Array.isArray(stage) ? stage.join(" and ") : stage)).join(" then ")}`}
    >
      {stages.map((stage, index) => (
        <React.Fragment key={`${title}-${index}`}>
          {index > 0 && (
            <span className={styles.arrow} aria-hidden="true">
              ↓
            </span>
          )}
          {Array.isArray(stage) ? (
            <div className={styles.parallel}>
              {stage.map((name) => (
                <span key={name}>{name}</span>
              ))}
            </div>
          ) : (
            <span className={styles.node}>{stage}</span>
          )}
        </React.Fragment>
      ))}
    </div>
  );
}
