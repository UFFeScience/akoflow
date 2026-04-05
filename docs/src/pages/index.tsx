import React, { useState } from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import BackgroundGraph from '../components/BackgroundGraph';
import styles from './index.module.css';

function CopyIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="9" y="9" width="13" height="13" rx="2" />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12" />
    </svg>
  );
}

function DownloadIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
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
      className={`${styles.copyBtn} ${copied ? styles.copied : ''}`}
      onClick={handle}
      title="Copy"
    >
      {copied ? <CheckIcon /> : <CopyIcon />}
    </button>
  );
}

type Platform = 'docker' | 'desktop';

const platforms: { id: Platform; label: string; recommended?: boolean }[] = [
  { id: 'docker',  label: 'Docker', },
  { id: 'desktop', label: 'Desktop' },
];

function DockerCard() {
  return (
    <div className={styles.installCard}>
      <div className={styles.installCardHeader}>
        <span className={styles.installCardTitle}>Docker</span>
        <span className={styles.installCardBadge}>Recommended</span>
      </div>
      <p className={styles.installCardDesc}>
        The fastest way to get AkôFlow running. One command installs and starts everything.
      </p>
      <div className={styles.cmdRow}>
        <span className={styles.cmdText}>curl -fsSL https://akoflow.com/run | bash</span>
        <CopyBtn text="curl -fsSL https://akoflow.com/run | bash" />
      </div>
    </div>
  );
}

function DesktopCard() {
  return (
    <div className={styles.installCard}>
      <div className={styles.installCardHeader}>
        <span className={styles.installCardTitle}>Desktop App</span>
      </div>
      <p className={styles.installCardDesc}>
        Native app for macOS and Linux. Includes a visual workflow editor and runtime manager.
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

const cards: Record<Platform, React.ReactNode> = {
  docker:  <DockerCard />,
  desktop: <DesktopCard />,
};

export default function Home(): React.JSX.Element {
  const { siteConfig } = useDocusaurusContext();
  const [platform, setPlatform] = useState<Platform>('docker');

  return (
    <Layout title={siteConfig.title} description="One Workflow. Multiple Platforms.">
      <main className={styles.page}>
        <BackgroundGraph />

        <img src="/akoflow/img/icon_akoflow.png" alt="AkôFlow" className={styles.logo} />

        <p className={styles.eyebrow}>
          Open Source &middot;{' '}
          <a href="http://www.ic.uff.br/" target="_blank" rel="noopener noreferrer">
            IC/UFF
          </a>{' '}
          e-Science Research Group
        </p>

        <h1 className={styles.headline}>
          One Workflow.<br />Multiple Platforms.
        </h1>

        <p className={styles.subheadline}>
          AkôFlow orchestrates container-based scientific workflows
          across heterogeneous environments — from your laptop to the cloud.
        </p>

        <div className={styles.platformSection}>
          <div className={styles.platformTabs}>
            {platforms.map((p) => (
              <button
                key={p.id}
                className={`${styles.tabBtn} ${platform === p.id ? styles.tabBtnActive : ''}`}
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
            Runs at <code>http://localhost:8080</code> &middot;{' '}
            <Link to="/docs/installation">Full installation guide</Link>
          </p>
        </div>

        <div className={styles.divider} />

        <div className={styles.actions}>
          <Link to="/docs/getting-started" className={styles.btnPrimary}>
            Get Started
          </Link>
          <Link to="/docs/user-guide" className={styles.btnGhost}>
            User Guide
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
