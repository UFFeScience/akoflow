import React, { type AnchorHTMLAttributes } from "react";
import type { ExtraProps } from "react-markdown";

export default function ReleaseLink({
  children,
  href,
  title,
  node,
}: AnchorHTMLAttributes<HTMLAnchorElement> & ExtraProps) {
  if (
    node?.children.some(
      (child) => child.type === "element" && child.tagName === "img",
    )
  )
    return <>{children}</>;
  return (
    <a href={href} title={title} target="_blank" rel="noopener noreferrer">
      {children}
    </a>
  );
}
