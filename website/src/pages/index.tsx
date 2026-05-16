import type {ReactNode} from 'react';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className="hero hero--primary" style={{padding: '4rem 0', textAlign: 'center'}}>
      <div className="container">
        <Heading as="h1" className="hero__title">{siteConfig.title}</Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div style={{marginTop: '2rem'}}>
          <a className="button button--secondary button--lg" href="/docs/intro" style={{marginRight: '1rem'}}>
            Get Started
          </a>
          <a className="button button--secondary button--lg" href="/docs/deployment/quickstart">
            Run a Node
          </a>
        </div>
      </div>
    </header>
  );
}

function Feature({title, description}: {title: string; description: string}) {
  return (
    <div className="col col--4" style={{padding: '2rem 1rem'}}>
      <div className="card-demo" style={{height: '100%'}}>
        <div className="card" style={{height: '100%', padding: '1.5rem'}}>
          <div className="card__header">
            <Heading as="h3">{title}</Heading>
          </div>
          <div className="card__body">
            <p>{description}</p>
          </div>
        </div>
      </div>
    </div>
  );
}

function FeaturesSection() {
  const features = [
    {title: 'HotStuff BFT Consensus', description: '4-phase HotStuff consensus with leader rotation, view change, and liveness guarantees. Fast finality and Byzantine fault tolerance.'},
    {title: 'EVM Compatibility', description: 'Full Ethereum Virtual Machine support. Deploy Solidity smart contracts, use EIP-1559 fee model, and interact via standard JSON-RPC.'},
    {title: 'Modular L1+L2+L3', description: 'Three-layer architecture: L1 for consensus/data availability, L2 for execution/AA/ZK, L3 for interoperability/bridge/governance.'},
    {title: 'libp2p Networking', description: 'Peer-to-peer networking via libp2p with Kademlia DHT, pubsub, peer management, and rate limiting.'},
    {title: 'secp256k1 Cryptography', description: 'Ethereum-compatible secp256k1 signatures, Keccak256 hashing, Merkle-Patricia Trie state, and groth16 ZK proofs.'},
    {title: 'Account Abstraction', description: 'ERC-4337 account abstraction with user operations, paymasters, and smart wallet support.'},
  ];

  return (
    <section style={{padding: '4rem 0'}}>
      <div className="container">
        <div className="row">
          {features.map((f, i) => <Feature key={i} {...f} />)}
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout title={siteConfig.title} description={siteConfig.tagline}>
      <HomepageHeader />
      <main>
        <FeaturesSection />
      </main>
    </Layout>
  );
}
