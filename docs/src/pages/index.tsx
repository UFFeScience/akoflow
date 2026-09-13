import React from "react";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import Layout from "@theme/Layout";
import BackgroundGraph from "../components/BackgroundGraph";
import styles from "./index.module.css";

function DownloadIcon() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
      <polyline points="7 10 12 15 17 10" />
      <line x1="12" y1="15" x2="12" y2="3" />
    </svg>
  );
}

function DesktopCard() {
  return (
    <div className={styles.installCard}>
      <div className={styles.installCardHeader}>
        <span className={styles.installCardTitle}>Desktop App</span>
        <span className={styles.installCardBadge}>Recommended</span>
      </div>
      <p className={styles.installCardDesc}>
        Native client for macOS, Windows, and Linux, distributed through
        versioned GitHub Releases.
      </p>
      <Link to="/docs/downloads" className={styles.downloadBtn}>
        <DownloadIcon />
        Choose download
      </Link>
      <Link to="/docs/installation" className={styles.docsLink}>
        Installation guide
      </Link>
    </div>
  );
}

export default function Home(): React.JSX.Element {
  const { siteConfig } = useDocusaurusContext();

  return (
    <Layout
      title={siteConfig.title}
      description="One Workflow. Multiple Platforms."
    >
      <main className={styles.page}>
        <BackgroundGraph />

        <img
          src="/img/brand/akoflow-macos.png"
          alt="AkôFlow"
          className={styles.logo}
        />

        <p className={styles.eyebrow}>
          Open Source &middot;{" "}
          <a
            href="http://www.ic.uff.br/"
            target="_blank"
            rel="noopener noreferrer"
          >
            IC/UFF
          </a>{" "}
          e-Science Research Group
        </p>

        <h1 className={styles.headline}>
          One Workflow.
          <br />
          Multiple Platforms.
        </h1>

        <p className={styles.subheadline}>
          Define a scientific workflow, choose where it runs, compare plans,
          execute one, and inspect the results and provenance. Start locally;
          connected environments require their own setup.
        </p>

        <div className={styles.platformSection}>
          <DesktopCard />

          <p className={styles.hint}>
            Desktop starts the local AkôFlow service through Docker &middot;{" "}
            <Link to="/docs/installation">Full installation guide</Link>
          </p>
        </div>

        <div className={styles.divider} />

        <div className={styles.actions}>
          <Link to="/docs/getting-started" className={styles.btnPrimary}>
            Get Started
          </Link>
          <Link to="/docs/guides/interface-tour" className={styles.btnGhost}>
            Interface tour
          </Link>
          <a
            href="https://github.com/UFFeScience/akoflow"
            target="_blank"
            rel="noopener noreferrer"
            className={styles.btnGhost}
          >
            GitHub
          </a>
        </div>
      </main>
    </Layout>
  );
}
