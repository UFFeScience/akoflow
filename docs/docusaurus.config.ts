import { themes as prismThemes } from 'prism-react-renderer';
import type { Config } from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'AkôFlow',
  tagline: 'Open Source Engine for Containerized Scientific Workflows',
  favicon: 'img/favicon.ico',

  url: 'https://akoflow.com',
  baseUrl: '/',

  organizationName: 'UFFeScience',
  projectName: 'akoflow',

  onBrokenLinks: 'warn',
  onBrokenMarkdownLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/UFFeScience/akoflow/tree/main/docs/',
          routeBasePath: 'docs',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/akoflow-social-card.png',
    colorMode: {
      defaultMode: 'light',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'AkôFlow',
      logo: {
        alt: 'AkôFlow Logo',
        src: 'img/icon_akoflow.png',
        srcDark: 'img/icon_akoflow.png',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Documentation',
        },
        {
          href: 'https://github.com/UFFeScience/akoflow',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            { label: 'Getting Started', to: '/docs/getting-started' },
            { label: 'Installation', to: '/docs/installation' },
            { label: 'User Guide', to: '/docs/user-guide' },
          ],
        },
        {
          title: 'Reference',
          items: [
            { label: 'Workflow Spec', to: '/docs/internal/workflow-spec' },
            { label: 'API Reference', to: '/docs/internal/api' },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/UFFeScience/akoflow',
            },
            {
              label: 'IC/UFF',
              href: 'http://www.ic.uff.br/',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} AkôFlow — IC/UFF e-Science Research Group. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.oneLight,
      darkTheme: prismThemes.oneDark,
      additionalLanguages: ['bash', 'yaml', 'json', 'docker'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
