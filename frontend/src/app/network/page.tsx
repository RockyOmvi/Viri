'use client'

import StatCard from '@/components/StatCard'
import { useStatus, usePeers, useNodeInfo, useConsensusState, useIndexerStatus } from '@/lib/api'
import type { Peer, NodeInfo, ConsensusState } from '@/types'

export default function NetworkPage() {
  const { data: status } = useStatus()
  const { data: peers } = usePeers()
  const { data: nodeInfo } = useNodeInfo()
  const { data: consensus } = useConsensusState()
  const { data: indexer } = useIndexerStatus()

  return (
    <>
      <h1 className="font-headline-xl text-headline-xl text-on-surface">Network</h1>
      <p className="font-body-md text-body-md text-on-surface-variant">
        Viri Testnet node status, consensus state, and peer information
      </p>

      <section className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-gutter">
        <StatCard label="Node Uptime" value={status?.uptime ? `${Math.floor(status.uptime / 3600)}h ${Math.floor((status.uptime % 3600) / 60)}m` : '—'} accentColor="border-l-primary" />
        <StatCard label="Connected Peers" value={status?.peers?.toString() || '—'} subtext={status?.peers && status.peers > 0 ? 'Connected' : 'No peers'} accentColor="border-l-secondary" />
        <StatCard label="Latest Block" value={status?.blockHeight?.toLocaleString() || '—'} accentColor="border-l-tertiary" />
        <StatCard label="Network" value={status?.networkName || 'Viri Testnet'} accentColor="border-l-outline" />
      </section>

      {/* Node Info */}
      {nodeInfo && (
        <section className="bg-surface-container border border-outline-variant rounded-lg p-6">
          <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface mb-4">Node Info</h2>
          <NodeInfoTable info={nodeInfo} />
        </section>
      )}

      {/* Consensus State */}
      {consensus && (
        <section className="bg-surface-container border border-outline-variant rounded-lg p-6">
          <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface mb-4">Consensus State</h2>
          <ConsensusTable state={consensus} />
        </section>
      )}

      {/* Indexer Status */}
      {indexer && (
        <section className="bg-surface-container border border-outline-variant rounded-lg p-6">
          <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface mb-4">Indexer</h2>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <StatCard label="Synced Block" value={indexer.synced_block?.toLocaleString() || '—'} accentColor="border-l-primary" />
            <StatCard label="Total Blocks" value={indexer.total_blocks?.toLocaleString() || '—'} accentColor="border-l-secondary" />
            <StatCard label="Total Txs" value={indexer.total_txs?.toLocaleString() || '—'} accentColor="border-l-tertiary" />
            <StatCard label="Status" value={indexer.status || '—'} accentColor="border-l-outline" />
          </div>
        </section>
      )}

      {/* Peers Table */}
      <section className="bg-surface-container border border-outline-variant rounded-lg overflow-hidden">
        <div className="p-4 border-b border-outline-variant bg-surface-container-high">
          <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface flex items-center gap-2">
            Peers ({peers?.length || 0})
          </h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead className="bg-surface-container-low font-label-caps text-label-caps text-on-surface-variant sticky top-0">
              <tr>
                <th className="p-4 border-b border-outline-variant">Peer ID</th>
                <th className="p-4 border-b border-outline-variant">Address</th>
                <th className="p-4 border-b border-outline-variant">Status</th>
                <th className="p-4 border-b border-outline-variant text-right">Latency</th>
              </tr>
            </thead>
            <tbody className="font-mono-sm text-mono-sm divide-y divide-outline-variant">
              {peers?.map((peer: Peer) => {
                const peerId = peer.peer_id || peer.id
                return (
                  <tr key={peerId} className="hover:bg-surface-container-highest transition-colors">
                    <td className="p-4 text-primary font-mono-sm text-mono-sm">{peerId?.slice(0, 20)}...</td>
                    <td className="p-4 text-on-surface">{peer.address || '—'}</td>
                    <td className="p-4">
                      <span className="bg-tertiary-container/20 text-tertiary border border-tertiary/30 px-2 py-0.5 rounded font-label-caps text-label-caps">
                        {peer.status || 'connected'}
                      </span>
                    </td>
                    <td className="p-4 text-on-surface-variant text-right">{peer.latency ? `${peer.latency}ms` : '—'}</td>
                  </tr>
                )
              })}
              {(!peers || peers.length === 0) && (
                <tr><td colSpan={4} className="p-4 text-on-surface-variant text-center">No peers connected</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </>
  )
}

function NodeInfoTable({ info }: { info: NodeInfo }) {
  const row = (label: string, val: string) => (
    <div className="flex py-2 border-b border-outline-variant last:border-0">
      <span className="font-label-caps text-label-caps text-on-surface-variant w-40 shrink-0">{label}</span>
      <span className="font-mono-sm text-mono-sm text-on-surface break-all">{val}</span>
    </div>
  )
  return (
    <div>
      {row('Version', info.version)}
      {row('Peer ID', info.peer_id)}
      {row('Full Peer ID', info.full_peer_id)}
      {row('Multiaddr', info.multiaddr)}
      {row('Chain ID', info.chain_id?.toString())}
      {row('Height', info.height?.toString())}
      {row('Peers', info.peers?.toString())}
      {row('Listening', info.listening ? 'Yes' : 'No')}
      {row('Validator', info.validator ? 'Yes' : 'No')}
    </div>
  )
}

function ConsensusTable({ state }: { state: ConsensusState }) {
  const row = (label: string, val: string) => (
    <div className="flex py-2 border-b border-outline-variant last:border-0">
      <span className="font-label-caps text-label-caps text-on-surface-variant w-40 shrink-0">{label}</span>
      <span className="font-mono-sm text-mono-sm text-on-surface break-all">{val}</span>
    </div>
  )
  return (
    <div>
      {row('Height', state.height?.toString())}
      {state.view !== undefined && row('View', state.view.toString())}
      {state.round !== undefined && row('Round', state.round.toString())}
      {state.phase && row('Phase', state.phase)}
      {state.proposer && row('Proposer', state.proposer)}
      {state.validators !== undefined && row('Validators', state.validators.toString())}
      {state.locked_block && row('Locked Block', state.locked_block)}
      {state.committed_block && row('Committed Block', state.committed_block)}
    </div>
  )
}
