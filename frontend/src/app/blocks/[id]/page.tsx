'use client'

import { useParams } from 'next/navigation'
import Link from 'next/link'
import HashLink from '@/components/HashLink'
import StatusBadge from '@/components/StatusBadge'
import { useBlockByNumber, formatWei } from '@/lib/api'
import type { Tx } from '@/types'

function formatValue(val: string): string {
  return formatWei(val, 4)
}

export default function BlockDetailPage() {
  const params = useParams()
  const blockNum = parseInt(params.id as string)
  const { data: block, isLoading, error } = useBlockByNumber(blockNum)

  if (isLoading) return <div className="text-on-surface-variant text-center py-8">Loading block...</div>
  if (error) return <div className="text-error text-center py-8">Error loading block: {error.message}</div>
  if (!block) return <div className="text-on-surface-variant text-center py-8">Block not found</div>

  const detailRow = (label: string, value: string, mono = false) => (
    <div className="flex flex-col sm:flex-row sm:items-center py-3 border-b border-outline-variant last:border-0">
      <span className="font-label-caps text-label-caps text-on-surface-variant w-40 shrink-0">{label}</span>
      <span className={`${mono ? 'font-mono-sm text-mono-sm' : 'font-body-sm text-body-sm'} text-on-surface break-all`}>
        {value}
      </span>
    </div>
  )

  const successCount = block.transactions.filter(t => t.status === 'success').length
  const failedCount = block.transactions.filter(t => t.status === 'failed').length
  const pendingCount = block.transactions.filter(t => t.status === 'pending').length

  return (
    <>
      <Link href="/blocks" className="text-primary hover:text-primary-fixed-dim font-label-caps text-label-caps mb-4 inline-block">
        &larr; Back to Blocks
      </Link>
      <h1 className="font-headline-xl text-headline-xl text-on-surface">Block #{block.number}</h1>

      <section className="bg-surface-container border border-outline-variant rounded-lg p-6">
        {detailRow('Block Height', block.number.toString())}
        {detailRow('Timestamp', new Date(block.timestamp * 1000).toLocaleString())}
        {detailRow('Hash', block.hash, true)}
        {detailRow('Parent Hash', block.parentHash, true)}
        {detailRow('Proposer', block.proposer, true)}
        {detailRow('State Root', block.stateRoot, true)}
        {block.receiptsRoot && detailRow('Receipts Root', block.receiptsRoot, true)}
        {detailRow('Gas Used', `${block.gasUsed.toLocaleString()} / ${block.gasLimit.toLocaleString()} (${((block.gasUsed / block.gasLimit) * 100).toFixed(1)}%)`)}
        {detailRow('Size', `${block.size} bytes`)}
        {detailRow('Transactions', block.transactionsCount.toString())}
      </section>

      {/* Transaction Status Summary */}
      {block.transactions.length > 0 && (
        <section className="bg-surface-container border border-outline-variant rounded-lg p-6">
          <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface mb-4">Transaction Summary</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="bg-tertiary-container/10 border border-tertiary/20 rounded-lg p-4 text-center">
              <div className="font-headline-lg text-headline-lg text-tertiary font-bold">{successCount}</div>
              <div className="font-label-caps text-label-caps text-on-surface-variant">Success</div>
            </div>
            <div className="bg-error-container/10 border border-error/20 rounded-lg p-4 text-center">
              <div className="font-headline-lg text-headline-lg text-error font-bold">{failedCount}</div>
              <div className="font-label-caps text-label-caps text-on-surface-variant">Failed</div>
            </div>
            <div className="bg-surface-container-low border border-outline-variant rounded-lg p-4 text-center">
              <div className="font-headline-lg text-headline-lg text-on-surface font-bold">{pendingCount}</div>
              <div className="font-label-caps text-label-caps text-on-surface-variant">Pending</div>
            </div>
          </div>
        </section>
      )}

      {block.transactions && block.transactions.length > 0 && (
        <section className="bg-surface-container border border-outline-variant rounded-lg overflow-hidden">
          <div className="p-4 border-b border-outline-variant bg-surface-container-high">
            <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface">
              Transactions ({block.transactions.length})
            </h2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse">
              <thead className="bg-surface-container-low font-label-caps text-label-caps text-on-surface-variant sticky top-0">
                <tr>
                  <th className="p-3 border-b border-outline-variant">#</th>
                  <th className="p-3 border-b border-outline-variant">Hash</th>
                  <th className="p-3 border-b border-outline-variant">From</th>
                  <th className="p-3 border-b border-outline-variant">To</th>
                  <th className="p-3 border-b border-outline-variant">Value</th>
                  <th className="p-3 border-b border-outline-variant">Gas Used</th>
                  <th className="p-3 border-b border-outline-variant text-right">Status</th>
                </tr>
              </thead>
              <tbody className="font-mono-sm text-mono-sm divide-y divide-outline-variant">
                {block.transactions.map((tx: Tx, i: number) => (
                  <tr key={tx.hash} className="hover:bg-surface-container-highest transition-colors">
                    <td className="p-3 text-on-surface-variant">{i}</td>
                    <td className="p-3"><HashLink hash={tx.hash} type="tx" /></td>
                    <td className="p-3 text-on-surface"><HashLink hash={tx.from} type="address" /></td>
                    <td className="p-3 text-secondary">
                      {tx.to ? <HashLink hash={tx.to} type="address" /> : <span className="text-on-surface-variant">Contract Creation</span>}
                    </td>
                    <td className="p-3 text-on-surface">{formatValue(tx.value)} VIRI</td>
                    <td className="p-3 text-on-surface">{tx.gasUsed?.toLocaleString() || '—'}</td>
                    <td className="p-3 text-right"><StatusBadge status={tx.status} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </>
  )
}
