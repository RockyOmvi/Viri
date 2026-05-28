'use client'

import { useParams } from 'next/navigation'
import Link from 'next/link'
import StatusBadge from '@/components/StatusBadge'
import { useTxByHash, formatWei } from '@/lib/api'
import type { Log } from '@/types'

export default function TxDetailPage() {
  const params = useParams()
  const hash = params.hash as string
  const { data: tx, isLoading, error } = useTxByHash(hash)

  if (isLoading) return <div className="text-on-surface-variant text-center py-8">Loading transaction...</div>
  if (error) return <div className="text-error text-center py-8">Error: {error.message}</div>
  if (!tx) return <div className="text-on-surface-variant text-center py-8">Transaction not found</div>

  const detailRow = (label: string, value: string, mono = false) => (
    <div className="flex flex-col sm:flex-row sm:items-center py-3 border-b border-outline-variant last:border-0">
      <span className="font-label-caps text-label-caps text-on-surface-variant w-40 shrink-0">{label}</span>
      <span className={`${mono ? 'font-mono-sm text-mono-sm' : 'font-body-sm text-body-sm'} text-on-surface break-all`}>
        {value}
      </span>
    </div>
  )

  return (
    <>
      <Link href="/" className="text-primary hover:text-primary-fixed-dim font-label-caps text-label-caps mb-4 inline-block">
        &larr; Back to Dashboard
      </Link>
      <h1 className="font-headline-xl text-headline-xl text-on-surface">Transaction</h1>

      <section className="bg-surface-container border border-outline-variant rounded-lg p-6">
        {detailRow('Tx Hash', tx.hash, true)}
        {detailRow('Status', '')}
        <div className="ml-40 -mt-2 mb-2"><StatusBadge status={tx.status} /></div>
        {detailRow('Block', tx.blockNumber.toString())}
        {detailRow('Timestamp', tx.timestamp ? new Date(tx.timestamp * 1000).toLocaleString() : 'Pending')}
        {detailRow('From', tx.from, true)}
        {detailRow('To', tx.to || 'Contract Creation', true)}
        {detailRow('Value', `${formatWei(tx.value)} VIRI`)}
        {detailRow('Gas Price', `${formatWei(tx.gasPrice)} VIRI`)}
        {detailRow('Gas Limit', tx.gasLimit.toLocaleString())}
        {detailRow('Gas Used', tx.gasUsed.toLocaleString())}
        {detailRow('Nonce', tx.nonce.toString())}
        {tx.contractAddress && detailRow('Contract Address', tx.contractAddress, true)}
        {detailRow('Input', tx.input || '0x', true)}
      </section>

      {/* Event Logs */}
      {tx.logs && tx.logs.length > 0 && (
        <section className="bg-surface-container border border-outline-variant rounded-lg overflow-hidden">
          <div className="p-4 border-b border-outline-variant bg-surface-container-high">
            <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface">
              Event Logs ({tx.logs.length})
            </h2>
          </div>
          <div className="divide-y divide-outline-variant">
            {tx.logs.map((log: Log, i: number) => (
              <div key={i} className="p-4 hover:bg-surface-container-highest">
                <div className="font-label-caps text-label-caps text-on-surface-variant mb-1">Log #{i}</div>
                <div className="font-mono-sm text-mono-sm text-on-surface space-y-1">
                  <div><span className="text-on-surface-variant">Address:</span> {log.address}</div>
                  {log.topics?.map((topic, j) => (
                    <div key={j}><span className="text-on-surface-variant">Topic[{j}]:</span> {topic}</div>
                  ))}
                  <div className="break-all"><span className="text-on-surface-variant">Data:</span> {log.data}</div>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}
    </>
  )
}
