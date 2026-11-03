import React from "react";
import Markdown, { defaultUrlTransform } from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";
import rehypeSanitize from "rehype-sanitize";
import type { Release } from "../../lib/releases.mjs";
import ReleaseImage from "./ReleaseImage";
import ReleaseParagraph from "./ReleaseParagraph";
import ReleaseLink from "./ReleaseLink";
import styles from "./Changelog.module.css";

export default function ReleaseNotes({ release }: { release: Release }) {
  function transformUrl(value: string, key: string) {
    const safe = defaultUrlTransform(value);
    if (!safe) return "";
    if (
      safe.startsWith("https://") ||
      safe.startsWith("http://") ||
      safe.startsWith("//") ||
      safe.startsWith("mailto:")
    )
      return safe;
    if (safe.startsWith("#")) return release.url + safe;
    if (safe.startsWith("/docs/") || safe.startsWith("/img/"))
      return "https://akoflow.com" + safe;
    const tag = encodeURIComponent(release.tag);
    const base =
      key === "src"
        ? `https://raw.githubusercontent.com/UFFeScience/akoflow/${tag}/`
        : `https://github.com/UFFeScience/akoflow/blob/${tag}/`;
    return new URL(safe, base).href;
  }
  const trimmed = release.body.trimStart();
  const firstLine = trimmed.split("\n", 1)[0];
  const firstHeading = firstLine.replace(/^#{1,6}\s+/, "").trim();
  const body =
    firstHeading === release.name || firstHeading === release.tag
      ? trimmed.slice(firstLine.length).trimStart()
      : release.body;
  if (!release.body.trim())
    return <p>See the GitHub release for its full details.</p>;
  return (
    <div className={`markdown ${styles.notes}`}>
      <Markdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeRaw, rehypeSanitize]}
        urlTransform={transformUrl}
        components={{
          a: ReleaseLink,
          img: ReleaseImage,
          p: ReleaseParagraph,
          h1: "h3",
          h2: "h3",
          h3: "h4",
        }}
      >
        {body}
      </Markdown>
    </div>
  );
}
