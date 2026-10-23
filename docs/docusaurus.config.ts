import { themes as prismThemes } from "prism-react-renderer";
import type { Config } from "@docusaurus/types";
import type * as Preset from "@docusaurus/preset-classic";
import { publicSiteUrl } from "./src/utils/publicSiteUrl";

const config: Config = {
  title: "AkôFlow",
  tagline: "Open Source Engine for Containerized Scientific Workflows",
  favicon: "img/brand/akoflow-logo.png",

  // The public domain proxies this deployment and rewrites its base paths.
  // Keep the build prefix required by the institutional and Pages origins.
  url: "https://uffescience.ic.uff.br",
  baseUrl: "/akoflow/",

  // Redirect legacy origins before hydration. Build the prefix from path
  // segments: the public proxy rewrites literal base-path strings in HTML.
  headTags: [
    {
      tagName: "script",
      attributes: {},
      innerHTML:
        "if (['uffescience.ic.uff.br', 'uffescience.github.io'].includes(location.hostname)) { var segments = location.pathname.split('/'); if (segments[1] === 'akoflow') { location.replace('https://akoflow.com/' + segments.slice(2).join('/') + location.search + location.hash); } }",
    },
  ],

  organizationName: "UFFeScience",
  projectName: "akoflow",

  onBrokenLinks: "warn",
  onBrokenMarkdownLinks: "warn",

  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },

  markdown: {
    mermaid: true,
  },

  presets: [
    [
      "classic",
      {
        debug: false,
        sitemap: {
          async createSitemapItems({ defaultCreateSitemapItems, ...params }) {
            const items = await defaultCreateSitemapItems(params);
            return items.map((item) => ({
              ...item,
              url: publicSiteUrl(item.url),
            }));
          },
        },
        docs: {
          sidebarPath: "./sidebars.ts",
          editUrl: "https://github.com/UFFeScience/akoflow/tree/main/docs/",
          routeBasePath: "docs",
        },
        blog: {
          blogTitle: "AkôFlow Blog",
          blogDescription:
            "Tutorials, experiments, and results from the AkôFlow project.",
          showReadingTime: true,
          postsPerPage: "ALL",
          blogSidebarCount: 0,
          onInlineTags: "ignore",
          onUntruncatedBlogPosts: "ignore",
        },
        theme: {
          customCss: "./src/css/custom.css",
        },
      } satisfies Preset.Options,
    ],
  ],

  themes: [
    "@docusaurus/theme-mermaid",
    [
      require.resolve("@easyops-cn/docusaurus-search-local"),
      {
        hashed: true,
        indexDocs: true,
        indexPages: true,
        docsRouteBasePath: "/docs",
        language: ["en"],
        searchBarShortcut: false,
        searchBarShortcutHint: false,
        searchResultLimits: 12,
      },
    ],
  ],

  plugins: [
    [
      "@docusaurus/plugin-client-redirects",
      {
        redirects: [
          { from: "/docs/examples", to: "/docs/showcase/" },
          { from: "/docs/user-guide", to: "/docs/getting-started/" },
          {
            from: "/docs/internal/api",
            to: "/docs/reference/api-overview/",
          },
          { from: "/docs/cli", to: "/docs/reference/api-overview/" },
          ...[
            "get-environments-environmentid-cloud-capacity-targets",
            "post-environments-environmentid-cloud-capacity-targets",
            "get-environments-environmentid-cloud-instances",
            "post-environments-environmentid-cloud-instances",
            "get-environments-environmentid-cloud-catalog",
            "post-environments-environmentid-cloud-catalog-refresh",
            "post-environments-environmentid-cloud-provisioning",
          ].map((slug) => ({
            from: `/docs/api/endpoints/environments/${slug}`,
            to: `/docs/api/endpoints/cloud/${slug}`,
          })),
        ],
      },
    ],
  ],

  themeConfig: {
    mermaid: {
      theme: {
        light: "base",
        dark: "base",
      },
      options: {
        look: "classic",
        themeVariables: {
          background: "#ffffff",
          primaryColor: "#ffffff",
          primaryTextColor: "#151515",
          primaryBorderColor: "#151515",
          secondaryColor: "#f2f2f2",
          secondaryTextColor: "#151515",
          secondaryBorderColor: "#151515",
          tertiaryColor: "#ffffff",
          tertiaryTextColor: "#151515",
          tertiaryBorderColor: "#151515",
          lineColor: "#151515",
          textColor: "#151515",
          mainBkg: "#ffffff",
          nodeBorder: "#151515",
          clusterBkg: "#f2f2f2",
          clusterBorder: "#151515",
          edgeLabelBackground: "#ffffff",
          fontFamily:
            "system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
          fontSize: "15px",
        },
        themeCSS: `
          .node rect,
          .node polygon,
          .node circle,
          .node ellipse,
          .stateGroup rect,
          .statediagram-state rect {
            filter: drop-shadow(0 2px 0 rgba(21, 21, 21, 0.12));
          }
          .nodeLabel,
          .stateLabel,
          .label {
            font-weight: 600;
            letter-spacing: -0.01em;
          }
          .edgeLabel,
          .edgeLabel p {
            color: #151515 !important;
            font-size: 13px;
            font-weight: 600;
          }
          .edgeLabel rect,
          .labelBkg {
            fill: #ffffff !important;
            opacity: 0.96 !important;
            rx: 4px;
            ry: 4px;
          }
          .flowchart-link,
          .transition {
            stroke-linecap: round;
            stroke-linejoin: round;
          }
        `,
        flowchart: {
          useMaxWidth: true,
          htmlLabels: true,
          curve: "basis",
        },
      },
    },
    image: "img/akoflow-social-card.png",
    colorMode: {
      defaultMode: "light",
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: "AkôFlow",
      hideOnScroll: false,
      logo: {
        alt: "AkôFlow application logo",
        src: "img/brand/akoflow-logo.png",
        srcDark: "img/brand/akoflow-logo.png",
      },
      items: [
        {
          to: "/docs/getting-started",
          position: "left",
          label: "Guide",
        },
        {
          to: "/docs/concepts",
          position: "left",
          label: "Concepts",
        },
        {
          to: "/docs/reference/api-overview",
          position: "left",
          label: "API Reference",
        },
        { to: "/blog", position: "left", label: "Blog" },
        {
          href: "https://github.com/UFFeScience/akoflow",
          label: "GitHub",
          position: "right",
        },
      ],
    },
    footer: {
      style: "dark",
      links: [
        {
          title: "Docs",
          items: [
            { label: "Getting Started", to: "/docs/getting-started" },
            { label: "Architecture internals", to: "/docs/modules" },
            { label: "Installation", to: "/docs/installation" },
            { label: "Downloads", to: "/docs/downloads" },
            { label: "Interface Tour", to: "/docs/guides/interface-tour" },
          ],
        },
        {
          title: "Reference",
          items: [
            { label: "Workflow Spec", to: "/docs/internal/workflow-spec" },
            { label: "API Reference", to: "/docs/reference/api-overview" },
          ],
        },
        {
          title: "Community",
          items: [
            {
              label: "GitHub",
              href: "https://github.com/UFFeScience/akoflow",
            },
            {
              label: "Releases",
              href: "https://github.com/UFFeScience/akoflow/releases",
            },
            {
              label: "IC/UFF",
              href: "http://www.ic.uff.br/",
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} AkôFlow — IC/UFF e-Science Research Group. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.oneLight,
      darkTheme: prismThemes.oneDark,
      additionalLanguages: ["bash", "yaml", "json", "docker"],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
