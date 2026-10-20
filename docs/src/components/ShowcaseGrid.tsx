import React, { type ReactNode } from "react";
import Link from "@docusaurus/Link";
import styles from "./WorkflowShowcase.module.css";
export default function ShowcaseGrid({ children }: { children: ReactNode }) {
  return <div className={styles.grid}>{children}</div>;
}
