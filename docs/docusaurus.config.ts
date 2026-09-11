import { themes as prismThemes } from "prism-react-renderer";
import type { Config } from "@docusaurus/types";
import type * as Preset from "@docusaurus/preset-classic";

const config: Config = {
  title: "AkôFlow",
  tagline: "Open Source Engine for Containerized Scientific Workflows",
  favicon: "img/favicon.ico",

  url: "https://uffescience.github.io",
  baseUrl: "/akoflow/",

  organizationName: "UFFeScience",
  projectName: "akoflow",

  onBrokenLinks: "warn",
  onBrokenMarkdownLinks: "warn",

  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },

  presets: [
    [
      "classic",
      {
        debug: false,
        docs: {
          sidebarPath: "./sidebars.ts",
          editUrl: "https://github.com/UFFeScience/akoflow/tree/main/docs/",
          routeBasePath: "docs",
        },
        blog: false,
        theme: {
          customCss: "./src/css/custom.css",
        },
      } satisfies Preset.Options,
    ],
  ],

  themes: [
    [
      require.resolve("@easyops-cn/docusaurus-search-local"),
      {
        hashed: true,
        indexDocs: true,
        indexPages: true,
        docsRouteBasePath: "/docs",
        language: ["en"],
        searchBarShortcutHint: true,
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
        ],
      },
    ],
  ],

  themeConfig: {
    image: "img/akoflow-social-card.png",
    colorMode: {
      defaultMode: "light",
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: "AkôFlow Docs",
      hideOnScroll: false,
      logo: {
        alt: "AkôFlow application logo",
        src: "img/icon_akoflow.png",
        srcDark: "img/icon_akoflow.png",
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
            { label: "Modules", to: "/docs/modules" },
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
              label: "Docker Hub",
              href: "https://hub.docker.com/u/akoflow",
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
