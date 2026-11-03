import React, { type ComponentProps } from "react";
import type { ExtraProps } from "react-markdown";
import styles from "./Changelog.module.css";

export default function ReleaseParagraph({
  node,
  children,
}: ComponentProps<"p"> & ExtraProps) {
  const includesImage = node?.children.some((child) => {
    if (child.type !== "element") return false;
    return (
      child.tagName === "img" ||
      child.children.some(
        (nested) => nested.type === "element" && nested.tagName === "img",
      )
    );
  });
  if (includesImage)
    return <div className={styles.imageParagraph}>{children}</div>;
  return <p>{children}</p>;
}
