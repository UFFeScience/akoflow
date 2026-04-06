import type { SidebarsConfig } from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'category',
      label: 'Overview',
      collapsed: false,
      items: ['getting-started', 'modules', 'concepts'],
    },
    {
      type: 'category',
      label: 'Setup',
      collapsed: false,
      items: ['cli', 'installation', 'downloads'],
    },
    {
      type: 'category',
      label: 'Usage',
      collapsed: false,
      items: ['user-guide', 'examples'],
    },
    {
      type: 'category',
      label: 'Internals',
      collapsed: true,
      items: ['engine', 'runtimes'],
    },
    {
      type: 'category',
      label: 'Reference',
      collapsed: false,
      items: ['internal/workflow-spec', 'internal/api'],
    },
  ],
};

export default sidebars;
