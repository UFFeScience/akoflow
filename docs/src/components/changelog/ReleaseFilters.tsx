import React, { type ChangeEvent } from "react";
import styles from "./Changelog.module.css";

type Props = {
  query: string;
  kind: string;
  count: number;
  refreshing: boolean;
  onQueryChange: (value: string) => void;
  onKindChange: (value: string) => void;
  onRefresh: () => void;
};

export default function ReleaseFilters(props: Props) {
  function changeQuery(event: ChangeEvent<HTMLInputElement>) {
    props.onQueryChange(event.target.value);
  }
  function changeKind(event: ChangeEvent<HTMLSelectElement>) {
    props.onKindChange(event.target.value);
  }
  return (
    <div className={styles.filters}>
      <label className={styles.search}>
        <span>Find a release</span>
        <input
          value={props.query}
          onChange={changeQuery}
          placeholder="Version or release notes…"
          type="search"
        />
      </label>
      <label>
        <span>Release type</span>
        <select value={props.kind} onChange={changeKind}>
          <option value="all">All releases</option>
          <option value="stable">Stable</option>
          <option value="prerelease">Pre-releases</option>
        </select>
      </label>
      <button
        type="button"
        onClick={props.onRefresh}
        disabled={props.refreshing}
      >
        {props.refreshing ? "Checking…" : "Refresh"}
      </button>
      <p className={styles.count} role="status">
        {props.count} {props.count === 1 ? "release" : "releases"}
      </p>
    </div>
  );
}
