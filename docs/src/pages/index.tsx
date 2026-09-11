import React, { useState } from "react";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import Layout from "@theme/Layout";
import BackgroundGraph from "../components/BackgroundGraph";
import styles from "./index.module.css";

function CopyIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect x="9" y="9" width="13" height="13" rx="2" />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <polyline points="20 6 9 17 4 12" />
    </svg>
  );
}

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

function CopyBtn({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  function handle() {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }
  return (
    <button
      className={`${styles.copyBtn} ${copied ? styles.copied : ""}`}
      onClick={handle}
      title="Copy"
    >
      {copied ? <CheckIcon /> : <CopyIcon />}
    </button>
  );
}

type Platform = "desktop" | "compose" | "api";

const platforms: { id: Platform; label: string; recommended?: boolean }[] = [
  { id: "desktop", label: "Desktop", recommended: true },
  { id: "compose", label: "Docker Compose" },
  { id: "api", label: "API" },
];

function ComposeCard() {
  return (
    <div className={styles.installCard}>
      <div className={styles.installCardHeader}>
        <span className={styles.installCardTitle}>Versioned daemon stack</span>
      </div>
      <p className={styles.installCardDesc}>
        Operate the AkôFlow daemon and BuildKit images directly from the release
        bundle.
      </p>
      <div className={styles.cmdRow}>
        <span className={styles.cmdText}>
          docker compose --env-file releases/.env -f releases/compose.yaml up -d
        </span>
        <CopyBtn text="docker compose --env-file releases/.env -f releases/compose.yaml up -d" />
      </div>
      <Link to="/docs/installation" className={styles.docsLink}>
        Deployment instructions
      </Link>
    </div>
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
        Native client for macOS, Windows, and Linux. It starts the matching
        daemon and BuildKit services through Docker.
      </p>
      <a
        href="https://github.com/UFFeScience/akoflow/releases"
        target="_blank"
        rel="noopener noreferrer"
        className={styles.downloadBtn}
      >
        <DownloadIcon />
        Download
      </a>
      <Link to="/docs/installation" className={styles.docsLink}>
        Installation guide
      </Link>
    </div>
  );
}

function ApiCard() {
  return (
    <div className={styles.installCard}>
      <div className={styles.installCardHeader}>
        <span className={styles.installCardTitle}>
          Automate through the HTTP API
        </span>
      </div>
      <p className={styles.installCardDesc}>
        Use the same operations as Desktop from scripts, experiment pipelines,
        and integrations.
      </p>
      <div className={styles.cmdRow}>
        <span className={styles.cmdText}>
          curl -H &quot;Authorization: Bearer $AKOFLOW_API_TOKEN&quot;
          $AKOFLOW_API_URL/environments/
        </span>
        <CopyBtn
          text={
            'curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" "$AKOFLOW_API_URL/environments/"'
          }
        />
      </div>
      <Link to="/docs/reference/api-overview" className={styles.docsLink}>
        API reference
      </Link>
    </div>
  );
}

const cards: Record<Platform, React.ReactNode> = {
  desktop: <DesktopCard />,
  compose: <ComposeCard />,
  api: <ApiCard />,
};

export default function Home(): React.JSX.Element {
  const { siteConfig } = useDocusaurusContext();
  const [platform, setPlatform] = useState<Platform>("desktop");

  return (
    <Layout
      title={siteConfig.title}
      description="One Workflow. Multiple Platforms."
    >
      <main className={styles.page}>
        <BackgroundGraph />

        <img
          src="/akoflow/img/icon_akoflow.png"
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
          Model, compare, and execute container-based scientific workflows
          across local, simulated, cloud, Kubernetes, and HPC environments.
        </p>

        <div className={styles.platformSection}>
          <div className={styles.platformTabs}>
            {platforms.map((p) => (
              <button
                key={p.id}
                className={`${styles.tabBtn} ${platform === p.id ? styles.tabBtnActive : ""}`}
                onClick={() => setPlatform(p.id)}
              >
                {p.label}
                {p.recommended && platform !== p.id && (
                  <span className={styles.tabRecommended}>rec</span>
                )}
              </button>
            ))}
          </div>

          {cards[platform]}

          <p className={styles.hint}>
            Docker selects an available loopback API port &middot;{" "}
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
