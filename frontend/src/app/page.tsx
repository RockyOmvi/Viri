'use client'

import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useState } from 'react'
import StatCard from '@/components/StatCard'
import StatusBadge from '@/components/StatusBadge'
import HashLink from '@/components/HashLink'
import { useStatus, useRecentBlocks, useRecentTxs, search, formatWei } from '@/lib/api'
import type { Block, Tx } from '@/types'

function timeAgo(ts: number): string {
  const seconds = Math.floor(Date.now() / 1000 - ts)
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  return `${hours}h ago`
}

function formatValue(val: string): string {
  return formatWei(val, 4)
}

export default function Dashboard() {
  const router = useRouter()
  const { data: status } = useStatus()
  const { data: blocks } = useRecentBlocks(10)
  const { data: txs } = useRecentTxs(10)

  return (
    <>
      <MobileSearch router={router} />

      <section className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-gutter">
        <StatCard
          label="Block Height"
          value={status?.blockHeight?.toLocaleString() || '—'}
          subtext="~2.5s avg"
          accentColor="border-l-primary"
          icon="view_in_ar"
          live
        />
        <StatCard
          label="Peers"
          value={status?.peers?.toString() || '—'}
          subtext="Healthy"
          accentColor="border-l-secondary"
          icon="hub"
        />
        <StatCard
          label="Gas Price"
          value={status?.gasPrice ? `${parseInt(status.gasPrice) / 1e9} Gwei` : '—'}
          subtext="Low network congestion"
          accentColor="border-l-tertiary"
          icon="local_gas_station"
        />
        <StatCard
          label="Chain ID"
          value={status?.chainId?.toString() || '—'}
          subtext={status?.networkName || 'Viri Testnet'}
          accentColor="border-l-outline"
          icon="link"
        />
      </section>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-gutter">
        <section className="bg-surface-container border border-outline-variant rounded-lg overflow-hidden flex flex-col h-full">
          <div className="p-4 border-b border-outline-variant flex justify-between items-center bg-surface-container-high">
            <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface flex items-center gap-2">
              Recent Blocks
            </h2>
            <Link
              href="/blocks"
              className="bg-surface border border-outline-variant hover:bg-surface-container-highest text-on-surface font-label-caps text-label-caps px-3 py-1 rounded transition-colors"
            >
              View All
            </Link>
          </div>
          <div className="overflow-x-auto flex-grow">
            <table className="w-full text-left border-collapse">
              <thead className="bg-surface-container-low font-label-caps text-label-caps text-on-surface-variant sticky top-0">
                <tr>
                  <th className="p-3 border-b border-outline-variant">Block</th>
                  <th className="p-3 border-b border-outline-variant">Hash</th>
                  <th className="p-3 border-b border-outline-variant">Txs</th>
                  <th className="p-3 border-b border-outline-variant">Proposer</th>
                  <th className="p-3 border-b border-outline-variant text-right">Age</th>
                </tr>
              </thead>
              <tbody className="font-mono-sm text-mono-sm divide-y divide-outline-variant">
                {blocks?.slice(0, 5).map((block: Block) => (
                  <tr key={block.hash} className="hover:bg-surface-container-highest transition-colors">
                    <td className="p-3"><HashLink hash={block.number.toString()} type="block" /></td>
                    <td className="p-3 text-on-surface"><HashLink hash={block.hash} type="block" /></td>
                    <td className="p-3 text-on-surface">{block.transactionsCount}</td>
                    <td className="p-3 text-secondary"><HashLink hash={block.proposer} type="address" /></td>
                    <td className="p-3 text-on-surface-variant text-right">{timeAgo(block.timestamp)}</td>
                  </tr>
                ))}
                {(!blocks || blocks.length === 0) && (
                  <tr><td colSpan={5} className="p-3 text-on-surface-variant text-center">No blocks found</td></tr>
                )}
              </tbody>
            </table>
          </div>
        </section>

        <section className="bg-surface-container border border-outline-variant rounded-lg overflow-hidden flex flex-col h-full">
          <div className="p-4 border-b border-outline-variant flex justify-between items-center bg-surface-container-high">
            <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface flex items-center gap-2">
              Recent Transactions
            </h2>
          </div>
          <div className="overflow-x-auto flex-grow">
            <table className="w-full text-left border-collapse">
              <thead className="bg-surface-container-low font-label-caps text-label-caps text-on-surface-variant sticky top-0">
                <tr>
                  <th className="p-3 border-b border-outline-variant">Tx Hash</th>
                  <th className="p-3 border-b border-outline-variant">From</th>
                  <th className="p-3 border-b border-outline-variant">To</th>
                  <th className="p-3 border-b border-outline-variant">Value</th>
                  <th className="p-3 border-b border-outline-variant text-right">Status</th>
                </tr>
              </thead>
              <tbody className="font-mono-sm text-mono-sm divide-y divide-outline-variant">
                {txs?.slice(0, 5).map((tx: Tx) => (
                  <tr key={tx.hash} className="hover:bg-surface-container-highest transition-colors">
                    <td className="p-3"><HashLink hash={tx.hash} type="tx" /></td>
                    <td className="p-3 text-on-surface"><HashLink hash={tx.from} type="address" /></td>
                    <td className="p-3 text-secondary">
                      {tx.to ? <HashLink hash={tx.to} type="address" /> : <span className="text-on-surface-variant">Contract</span>}
                    </td>
                    <td className="p-3 text-on-surface">{formatValue(tx.value)} VIRI</td>
                    <td className="p-3 text-right"><StatusBadge status={tx.status} /></td>
                  </tr>
                ))}
                {(!txs || txs.length === 0) && (
                  <tr><td colSpan={5} className="p-3 text-on-surface-variant text-center">No transactions found</td></tr>
                )}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </>
  )
}

function MobileSearch({ router }: { router: ReturnType<typeof useRouter> }) {
  const [query, setQuery] = useState('')
  const [searching, setSearching] = useState(false)
  const [error, setError] = useState('')

  const handleSearch = async () => {
    if (!query.trim()) return
    setSearching(true)
    setError('')
    try {
      const result = await search(query)
      if (result === 'block') router.push(`/blocks/${query}`)
      else if (result === 'tx') router.push(`/tx/${query}`)
      else if (result === 'address') router.push(`/address/${query}`)
      else setError('Not found')
    } catch { setError('Search failed') }
    finally { setSearching(false) }
  }

  return (
    <div className="lg:hidden w-full bg-surface-container border border-outline-variant rounded-lg p-2 flex items-center focus-within:border-primary transition-colors">
      <svg className="w-5 h-5 text-on-surface-variant mr-2 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
      </svg>
      <input
        className="bg-transparent border-none outline-none text-body-md font-body-md text-on-surface placeholder-on-surface-variant w-full focus:ring-0 p-0"
        placeholder="Search by Address / Txn Hash / Block"
        value={query}
        onChange={e => { setQuery(e.target.value); setError('') }}
        onKeyDown={e => { if (e.key === 'Enter') handleSearch() }}
        type="text"
      />
      {searching && <span className="text-on-surface-variant shrink-0">...</span>}
      {error && <span className="text-error shrink-0 ml-1">{error}</span>}
    </div>
  )
}
