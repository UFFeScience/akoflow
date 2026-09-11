import React from "react";
import styles from "./InfrastructureWalkthrough.module.css";

export function ConnectionPath() {
  return (
    <figure className={styles.figure}>
      <div className={styles.path} role="img" aria-label="Desktop asks the daemon to open a terminal through a connection and optional bastion to the selected resource">
        <Box eyebrow="Client" title="AkôFlow Desktop" detail="Select resource" />
        <Arrow label="HTTPS / WS" />
        <Box eyebrow="Control plane" title="AkôFlow daemon" detail="Resolve runtime + connection" />
        <Arrow label="SSH + proxy" />
        <Box eyebrow="Remote" title="Login node" detail="PTY and audited session" />
      </div>
      <figcaption>The Desktop does not SSH directly. The daemon resolves the selected resource and applies its credential, host key, and proxy route.</figcaption>
    </figure>
  );
}

export function TerminalPanelGuide() {
  return (
    <figure className={styles.figure}>
      <div className={styles.window} role="img" aria-label="Illustration of the AkôFlow resource page and interactive terminal panel">
        <div className={styles.windowBar}><span /><span /><span /><b>Resources / hpc-login</b></div>
        <div className={styles.resourceRow}>
          <div><small>RESOURCE</small><strong>hpc-login</strong><em>online · ssh · interactive</em></div>
          <button type="button" tabIndex={-1}>1&nbsp; Open terminal</button>
        </div>
        <div className={styles.terminalBar}><b>2&nbsp; hpc-login</b><span>Export log</span><span>Close session</span></div>
        <pre><code>$ hostname{`\n`}login01.cluster.example{`\n`}$ sinfo -s ▌</code></pre>
      </div>
      <figcaption>Open the session from the resource, work in the persistent bottom panel, then export or close it explicitly.</figcaption>
    </figure>
  );
}

function Box({eyebrow, title, detail}: {eyebrow: string; title: string; detail: string}) {
  return <div className={styles.box}><small>{eyebrow}</small><strong>{title}</strong><span>{detail}</span></div>;
}

function Arrow({label}: {label: string}) {
  return <div className={styles.arrow}><span>{label}</span><b aria-hidden="true">→</b></div>;
}
