import React, { useEffect, useState } from "react";
import snapshot from "../../data/releases.generated.json";
import {
  fetchAllReleases,
  normalizeReleases,
  releasesPage,
  type ReleaseSnapshot,
} from "../../lib/releases.mjs";
import ReleaseCard from "./ReleaseCard";
import ReleaseFilters from "./ReleaseFilters";
import styles from "./Changelog.module.css";

export default function ProjectChangelog() {
  const [history, setHistory] = useState<ReleaseSnapshot>(() => ({
    updatedAt: snapshot.updatedAt,
    releases: normalizeReleases(snapshot.releases),
  }));
  const [query, setQuery] = useState("");
  const [kind, setKind] = useState("all");
  const [refreshIndex, setRefreshIndex] = useState(0);
  const [refreshing, setRefreshing] = useState(true);
  const [live, setLive] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    const timeout = setTimeout(() => controller.abort(), 15000);
    setRefreshing(true);
    fetchAllReleases(fetch, { signal: controller.signal })
      .then((releases) => {
        if (!active) return;
        setHistory({ updatedAt: new Date().toISOString(), releases });
        setLive(true);
      })
      .catch(() => {
        if (active) setLive(false);
      })
      .finally(() => {
        clearTimeout(timeout);
        if (active) setRefreshing(false);
      });
    return () => {
      active = false;
      clearTimeout(timeout);
      controller.abort();
    };
  }, [refreshIndex]);
  function refresh() {
    setRefreshIndex((value) => value + 1);
  }
  const term = query.trim().toLocaleLowerCase();
  const releases = history.releases.filter((release) => {
    const matchesType =
      kind === "all" ||
      (kind === "prerelease" ? release.prerelease : !release.prerelease);
    return (
      matchesType &&
      `${release.tag} ${release.name} ${release.body}`
        .toLocaleLowerCase()
        .includes(term)
    );
  });
  return (
    <div className={styles.changelog}>
      <div className={styles.origin}>
        <a href={releasesPage} target="_blank" rel="noopener noreferrer">
          From GitHub Releases ↗
        </a>
        <span>{live ? "Up to date" : "Published release history"}</span>
      </div>
      <ReleaseFilters
        query={query}
        kind={kind}
        count={releases.length}
        refreshing={refreshing}
        onQueryChange={setQuery}
        onKindChange={setKind}
        onRefresh={refresh}
      />
      {releases.length ? (
        releases.map((release, index) => (
          <ReleaseCard
            key={release.id}
            release={release}
            expanded={index === 0 || Boolean(term)}
          />
        ))
      ) : (
        <p>No releases match your search.</p>
      )}
    </div>
  );
}
