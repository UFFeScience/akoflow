import React from "react";
import type { ReleaseAsset } from "../../lib/releases.mjs";
import styles from "./Changelog.module.css";

function assetSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MiB`;
}

export default function ReleaseAssets({ assets }: { assets: ReleaseAsset[] }) {
  if (!assets.length) return null;
  return (
    <details className={styles.assets}>
      <summary>Downloads · {assets.length} files</summary>
      <ul>
        {assets.map((asset) => (
          <li key={asset.name}>
            <a href={asset.url}>{asset.name}</a>
            <span>{assetSize(asset.size)}</span>
          </li>
        ))}
      </ul>
    </details>
  );
}
