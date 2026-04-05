import type { SidebarsConfig } from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'category',
      label: 'Overview',
      collapsed: false,
      items: ['getting-started'],
    },
    {
      type: 'category',
      label: 'Setup',
      collapsed: false,
      items: ['installation'],
    },
    {
      type: 'category',
      label: 'Usage',
      collapsed: false,
      items: ['user-guide'],
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
