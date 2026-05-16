import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    'intro',
    {
      type: 'category',
      label: 'Architecture',
      items: [
        'architecture/overview',
        'architecture/consensus',
        'architecture/p2p',
        'architecture/state',
        'architecture/crypto',
        'architecture/layer2',
        'architecture/layer3',
      ],
    },
    {
      type: 'category',
      label: 'Deployment',
      items: [
        'deployment/quickstart',
        'deployment/docker',
        'deployment/azure',
        'deployment/standalone',
        'deployment/testnet',
      ],
    },
    {
      type: 'category',
      label: 'API Reference',
      items: [
        'api/json-rpc',
        'api/rest',
        'api/websocket',
        'api/sdk',
      ],
    },
    {
      type: 'category',
      label: 'Node Operation',
      items: [
        'node/configuration',
        'node/validator',
        'node/faucet',
        'node/monitoring',
        'node/troubleshooting',
      ],
    },
    {
      type: 'category',
      label: 'Development',
      items: [
        'development/setup',
        'development/testing',
        'development/contributing',
        'development/smart-contracts',
      ],
    },
  ],
};

export default sidebars;
