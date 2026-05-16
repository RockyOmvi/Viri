'use client'

import StatCard from '@/components/StatCard'
import HashLink from '@/components/HashLink'
import { useConsensusState, useStatus, usePeers } from '@/lib/api'

export default function ValidatorsPage() {
  const { data: consensus } = useConsensusState()
  const { data: status } = useStatus()
  const { data: peers } = usePeers()

  const validatorCount = consensus?.validators || status?.peers || 0

  return (
    <>
      <h1 className="font-headline-xl text-headline-xl text-on-surface">Validators</h1>
      <p className="font-body-md text-body-md text-on-surface-variant">
        Active validators securing the Viri Testnet via HotStuff BFT consensus
      </p>

      <section className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-gutter">
        <StatCard label="Active Validators" value={validatorCount.toString()} accentColor="border-l-primary" icon="verified" />
        <StatCard label="Chain Height" value={consensus?.height?.toLocaleString() || status?.blockHeight?.toLocaleString() || '—'} accentColor="border-l-secondary" />
        <StatCard label="Current View" value={consensus?.view?.toString() || '—'} accentColor="border-l-tertiary" />
        <StatCard label="Current Phase" value={consensus?.phase || '—'} subtext="HotStuff 4-phase" accentColor="border-l-outline" />
      </section>

      {consensus?.proposer && (
        <section className="bg-surface-container border border-outline-variant rounded-lg p-6">
          <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface mb-4">Current Leader</h2>
          <div className="font-mono-sm text-mono-sm text-on-surface flex items-center gap-2">
            <span className="w-3 h-3 bg-tertiary rounded-full animate-pulse"></span>
            <HashLink hash={consensus.proposer} type="address" />
          </div>
        </section>
      )}

      <section className="bg-surface-container border border-outline-variant rounded-lg overflow-hidden">
        <div className="p-4 border-b border-outline-variant bg-surface-container-high">
          <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface">Known Peers</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead className="bg-surface-container-low font-label-caps text-label-caps text-on-surface-variant sticky top-0">
              <tr>
                <th className="p-3 border-b border-outline-variant">Peer ID</th>
                <th className="p-3 border-b border-outline-variant">Multiaddr</th>
                <th className="p-3 border-b border-outline-variant">Status</th>
              </tr>
            </thead>
            <tbody className="font-mono-sm text-mono-sm divide-y divide-outline-variant">
              {peers?.map((peer) => {
                const peerId = peer.peer_id || peer.id
                return (
                  <tr key={peerId} className="hover:bg-surface-container-highest transition-colors">
                    <td className="p-3 text-primary">{peerId?.slice(0, 30)}...</td>
                    <td className="p-3 text-on-surface break-all">{peer.address || '—'}</td>
                    <td className="p-3">
                      <span className="bg-tertiary-container/20 text-tertiary border border-tertiary/30 px-2 py-0.5 rounded font-label-caps text-label-caps">
                        {peer.status || 'connected'}
                      </span>
                    </td>
                  </tr>
                )
              })}
              {(!peers || peers.length === 0) && (
                <tr><td colSpan={3} className="p-3 text-on-surface-variant text-center">No peers found</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </>
  )
}
