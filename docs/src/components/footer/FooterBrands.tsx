import React from "react";
import Link from "@docusaurus/Link";
import useBaseUrl from "@docusaurus/useBaseUrl";
import styles from "./FooterBrands.module.css";

export default function FooterBrands() {
  const akoflowLogo = useBaseUrl("/img/brand/akoflow-logo.png");
  const escienceLogo = useBaseUrl("/img/brand/uffescience-logo.jpg");
  const instituteLogo = useBaseUrl("/img/brand/ic-uff-logo.png");

  return (
    <nav className={styles.brands} aria-label="AkôFlow and institutions">
      <Link to="/" className={styles.product} aria-label="AkôFlow home">
        <img src={akoflowLogo} alt="" width={56} height={56} loading="lazy" />
        <span>
          <strong>AkôFlow</strong>
          <span>Scientific workflows</span>
        </span>
      </Link>
      <div className={styles.institutions}>
        <a href="https://uffescience.ic.uff.br/" className={styles.institution}>
          <span className={styles.logoSurface}>
            <img
              src={escienceLogo}
              alt="UFF eScience Research Group"
              width={160}
              height={120}
              loading="lazy"
            />
          </span>
          <span>UFF eScience</span>
        </a>
        <a href="https://www.ic.uff.br/" className={styles.institution}>
          <span className={styles.logoSurface}>
            <img
              src={instituteLogo}
              alt="Instituto de Computação — UFF"
              width={160}
              height={120}
              loading="lazy"
            />
          </span>
          <span>Instituto de Computação · UFF</span>
        </a>
      </div>
    </nav>
  );
}
