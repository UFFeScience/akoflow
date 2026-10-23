import React from "react";
import Head from "@docusaurus/Head";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import { useLocation } from "@docusaurus/router";
import { PageMetadata, useThemeConfig } from "@docusaurus/theme-common";
import {
  DEFAULT_SEARCH_TAG,
  useAlternatePageUtils,
} from "@docusaurus/theme-common/internal";
import SearchMetadata from "@theme/SearchMetadata";
import { publicSiteUrl } from "@site/src/utils/publicSiteUrl";

export default function SiteMetadata() {
  const { i18n } = useDocusaurusContext();
  const { metadata, image } = useThemeConfig();
  const { pathname } = useLocation();
  const alternatePages = useAlternatePageUtils();
  const canonical = publicSiteUrl(pathname);
  const currentLang = i18n.localeConfigs[i18n.currentLocale]!.htmlLang;

  return (
    <>
      <Head>
        <body />
        <meta name="twitter:card" content="summary_large_image" />
        <link rel="canonical" href={canonical} />
        <meta property="og:url" content={canonical} />
        <meta property="og:locale" content={currentLang.replaceAll("-", "_")} />
        {Object.entries(i18n.localeConfigs).map(([locale, { htmlLang }]) => (
          <link
            key={locale}
            rel="alternate"
            hrefLang={htmlLang}
            href={publicSiteUrl(
              alternatePages.createUrl({ locale, fullyQualified: true }),
            )}
          />
        ))}
        <link
          rel="alternate"
          hrefLang="x-default"
          href={publicSiteUrl(
            alternatePages.createUrl({
              locale: i18n.defaultLocale,
              fullyQualified: true,
            }),
          )}
        />
      </Head>
      {image && <PageMetadata image={publicSiteUrl(image)} />}
      <SearchMetadata tag={DEFAULT_SEARCH_TAG} locale={i18n.currentLocale} />
      <Head>
        {metadata.map((item, index) => (
          <meta key={index} {...item} />
        ))}
      </Head>
    </>
  );
}
