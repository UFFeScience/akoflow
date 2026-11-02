import React, { type ReactNode } from "react";
import Link from "@docusaurus/Link";
import styles from "./WorkflowShowcase.module.css";
export default function Arrow() {
  return (
    <span className={styles.arrow} aria-hidden="true">
      →
    </span>
  );
}
