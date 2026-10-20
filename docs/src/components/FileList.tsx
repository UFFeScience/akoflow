import React, { type ReactNode } from "react";
import Link from "@docusaurus/Link";
import styles from "./WorkflowShowcase.module.css";
export default function FileList({ children }: { children: ReactNode }) {
  return <div className={styles.files}>{children}</div>;
}
