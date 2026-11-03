import React from "react";
import type { Release } from "../../lib/releases.mjs";
import ReleaseNotes from "./ReleaseNotes";
import ReleaseAssets from "./ReleaseAssets";
import styles from "./Changelog.module.css";

export default function ReleaseCard({
  release,
  expanded,
}: {
  release: Release;
  expanded: boolean;
}) {
  const date = new Date(release.publishedAt);
  const published = Number.isNaN(date.getTime())
    ? ""
    : new Intl.DateTimeFormat("en", {
        year: "numeric",
        month: "long",
        day: "numeric",
        timeZone: "UTC",
      }).format(date);
  return (
    <article
      id={`release-${release.tag.replace(/[^a-z0-9-]/gi, "-")}`}
      className={styles.release}
    >
      <header className={styles.releaseHeader}>
        <div>
          <div className={styles.metadata}>
            <span>{release.tag}</span>
            {published && (
              <time dateTime={release.publishedAt}>{published}</time>
            )}
            {release.prerelease && (
              <span className={styles.badge}>Pre-release</span>
            )}
          </div>
          <h2>{release.name}</h2>
        </div>
        <a
          href={release.url}
          target="_blank"
          rel="noopener noreferrer"
          className={styles.source}
        >
          GitHub release ↗
        </a>
      </header>
      <details className={styles.releaseBody} open={expanded}>
        <summary>Release notes</summary>
        <ReleaseNotes release={release} />
      </details>
      <ReleaseAssets assets={release.assets} />
    </article>
  );
}
