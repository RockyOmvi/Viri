import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Viri Blockchain',
  tagline: 'L1+L2+L3 modular blockchain with HotStuff BFT consensus',
  favicon: 'img/favicon.ico',

  url: 'https://viri-chain.github.io',
  baseUrl: '/',

  organizationName: 'viri-chain',
  projectName: 'viri',

  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

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
          editUrl: 'https://github.com/viri-chain/viri/edit/main/website/',
        },
        blog: {
          showReadingTime: true,
          editUrl: 'https://github.com/viri-chain/viri/edit/main/website/blog/',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/viri-social-card.jpg',
    navbar: {
      title: 'Viri',
      logo: {
        alt: 'Viri Logo',
        src: 'img/logo.svg',
      },
      items: [
        {type: 'docSidebar', sidebarId: 'docsSidebar', position: 'left', label: 'Docs'},
        {to: '/blog', label: 'Blog', position: 'left'},
        {
          href: 'https://github.com/viri-chain/viri',
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
            {label: 'Introduction', to: '/docs/intro'},
            {label: 'Architecture', to: '/docs/architecture/overview'},
            {label: 'API Reference', to: '/docs/api/json-rpc'},
          ],
        },
        {
          title: 'Community',
          items: [
            {label: 'GitHub', href: 'https://github.com/viri-chain/viri'},
          ],
        },
        {
          title: 'More',
          items: [
            {label: 'Blog', to: '/blog'},
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Viri Chain. Built with Docusaurus.`,
    },
    prism: {
      theme: require('prism-react-renderer').themes.github,
      darkTheme: require('prism-react-renderer').themes.dracula,
      additionalLanguages: ['go', 'rust', 'toml', 'yaml', 'json', 'bash', 'solidity'],
    },
  } satisfies Preset.ThemeConfig,

};

export default config;
