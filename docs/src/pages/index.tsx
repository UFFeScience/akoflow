import React from "react";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import Layout from "@theme/Layout";
import BackgroundGraph from "../components/BackgroundGraph";
import WorkflowCatalog from "../components/WorkflowCatalog";
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

const documentationSections = [
  {
    title: "Tutorials",
    description: "Install Desktop, make a first run, connect infrastructure, and follow complete examples.",
    links: [
      ["Getting started", "/docs/getting-started"],
      ["Installation", "/docs/installation"],
      ["Workflow showcase", "/docs/showcase/"],
    ],
  },
  {
    title: "How-to guides",
    description: "Task-focused instructions for workflows, environments, artifacts, and operations.",
    links: [
      ["Workflows", "/docs/guides/workflows/definitions"],
      ["Infrastructure", "/docs/guides/infrastructure/environments"],
      ["Data and evidence", "/docs/guides/data/artifacts"],
      ["Operations", "/docs/guides/operations/instance-management"],
    ],
  },
  {
    title: "Explanations",
    description: "Understand planning, runtimes, network estimates, and the evidence left by a run.",
    links: [
      ["Core concepts", "/docs/concepts"],
      ["Planning and plans", "/docs/explanations/planning"],
      ["Observed timing", "/docs/explanations/observed-timing"],
    ],
  },
  {
    title: "Developing AkôFlow",
    description: "Explore the engine, runtime adapters, and module boundaries.",
    links: [
      ["Architecture", "/docs/modules"],
      ["Engine", "/docs/engine"],
      ["Runtimes", "/docs/runtimes"],
    ],
  },
  {
    title: "Reference",
    description: "Find API endpoints, payloads, environment formats, and execution states.",
    links: [
      ["API overview", "/docs/reference/api-overview"],
      ["Environment YAML", "/docs/reference/environment-yaml"],
      ["Planning and execution states", "/docs/reference/planning-and-execution-states"],
    ],
  },
  {
    title: "Contributing",
    description: "Improve the documentation and keep examples aligned with verified behavior.",
    links: [["Documentation plan", "/docs/contributing/documentation-plan"]],
  },
] as const;

export default function Home(): React.JSX.Element {
  const { siteConfig } = useDocusaurusContext();

  return (
    <Layout
      title={siteConfig.title}
      description="Define, plan, run, and inspect scientific workflows with AkôFlow."
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
          Define a scientific workflow, choose where it runs, make a plan,
          execute it, and inspect the result. Start locally;
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

        <section className={styles.directory} aria-labelledby="documentation-sections">
          <div className={styles.directoryHeading}>
            <h2 id="documentation-sections">Explore the documentation</h2>
            <p>Browse by purpose. These sections match the documentation sidebar.</p>
          </div>
          <div className={styles.sectionGrid}>
            {documentationSections.map((section) => (
              <section className={styles.sectionCard} key={section.title}>
                <h3>{section.title}</h3>
                <p>{section.description}</p>
                <ul>
                  {section.links.map(([label, href]) => (
                    <li key={href}><Link to={href}>{label} <span aria-hidden="true">→</span></Link></li>
                  ))}
                </ul>
              </section>
            ))}
          </div>
        </section>

        <section className={styles.catalogSection} aria-labelledby="example-workflows">
          <div className={styles.directoryHeading}>
            <h2 id="example-workflows">Example workflows</h2>
            <p>Preview every workflow graph by section. Select a card to open its walkthrough or pattern.</p>
          </div>
          <WorkflowCatalog />
        </section>
      </main>
    </Layout>
  );
}
