'use client'

import { useParams } from 'next/navigation'
import Link from 'next/link'
import { useState } from 'react'
import { useAccount, useAddressTxs } from '@/lib/api'
import StatusBadge from '@/components/StatusBadge'
import HashLink from '@/components/HashLink'
import type { IndexedTx } from '@/types'

function formatWei(hex: string): string {
  const num = parseInt(hex, 16)
  if (isNaN(num)) return '0'
  return (num / 1e18).toFixed(6)
}

function shortenHash(h: string): string {
  if (!h || h.length < 10) return h || ''
  return `${h.slice(0, 8)}...${h.slice(-6)}`
}

export default function AddressPage() {
  const params = useParams()
  const address = params.address as string
  const { data: account, isLoading, error } = useAccount(address)
  const [txPage, setTxPage] = useState(1)
  const { data: txHistory } = useAddressTxs(address, txPage)

  const isContract = account?.code && account.code !== '0x'

  const detailRow = (label: string, value: string, mono = false) => (
    <div className="flex flex-col sm:flex-row sm:items-center py-3 border-b border-outline-variant last:border-0">
      <span className="font-label-caps text-label-caps text-on-surface-variant w-40 shrink-0">{label}</span>
      <span className={`${mono ? 'font-mono-sm text-mono-sm' : 'font-body-sm text-body-sm'} text-on-surface break-all`}>
        {value}
      </span>
    </div>
  )

  if (isLoading) return <div className="text-on-surface-variant text-center py-8">Loading address...</div>
  if (error) return <div className="text-error text-center py-8">Error: {error.message}</div>
  if (!account) return <div className="text-on-surface-variant text-center py-8">Address not found</div>

  return (
    <>
      <h1 className="font-headline-xl text-headline-xl text-on-surface">Address</h1>

      <section className="bg-surface-container border border-outline-variant rounded-lg p-6">
        {detailRow('Address', account.address, true)}
        {detailRow('Balance', `${formatWei(account.balance)} VIRI`)}
        {detailRow('Nonce', account.nonce.toString())}
        {detailRow('Type', isContract ? 'Contract' : 'Externally Owned Account')}
        {isContract && detailRow('Code Hash', account.code, true)}
      </section>

      {/* Contract Read/Write */}
      {isContract && (
        <section className="bg-surface-container border border-outline-variant rounded-lg p-6">
          <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface mb-4">Contract Interaction</h2>
          <ContractInteraction address={address} />
        </section>
      )}

      {/* Transaction History */}
      <section className="bg-surface-container border border-outline-variant rounded-lg overflow-hidden">
        <div className="p-4 border-b border-outline-variant bg-surface-container-high">
          <h2 className="font-headline-lg text-headline-lg font-bold text-on-surface">
            Transactions ({txHistory?.total?.toLocaleString() || '...'})
          </h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead className="bg-surface-container-low font-label-caps text-label-caps text-on-surface-variant sticky top-0">
              <tr>
                <th className="p-3 border-b border-outline-variant">Tx Hash</th>
                <th className="p-3 border-b border-outline-variant">Block</th>
                <th className="p-3 border-b border-outline-variant">From</th>
                <th className="p-3 border-b border-outline-variant">To</th>
                <th className="p-3 border-b border-outline-variant">Value</th>
                <th className="p-3 border-b border-outline-variant text-right">Status</th>
              </tr>
            </thead>
            <tbody className="font-mono-sm text-mono-sm divide-y divide-outline-variant">
              {txHistory?.transactions?.map((tx: IndexedTx) => (
                <tr key={tx.hash} className="hover:bg-surface-container-highest transition-colors">
                  <td className="p-3"><HashLink hash={tx.hash} type="tx" /></td>
                  <td className="p-3"><HashLink hash={tx.block_number.toString()} type="block" /></td>
                  <td className="p-3 text-on-surface">{shortenHash(tx.from)}</td>
                  <td className="p-3 text-secondary">{tx.to ? shortenHash(tx.to) : <span className="text-on-surface-variant">Contract</span>}</td>
                  <td className="p-3 text-on-surface">{formatWei(tx.value)} VIRI</td>
                  <td className="p-3 text-right"><StatusBadge status={(tx.status || 'pending') as 'success' | 'pending' | 'failed'} /></td>
                </tr>
              ))}
              {(!txHistory?.transactions || txHistory.transactions.length === 0) && (
                <tr><td colSpan={6} className="p-3 text-on-surface-variant text-center">No transactions found</td></tr>
              )}
            </tbody>
          </table>
        </div>
        {txHistory && txHistory.pages > 1 && (
          <div className="flex justify-center gap-4 p-4 border-t border-outline-variant">
            <button
              onClick={() => setTxPage(p => Math.max(1, p - 1))}
              disabled={txPage <= 1}
              className="bg-surface border border-outline-variant hover:bg-surface-container-highest text-on-surface font-label-caps text-label-caps px-4 py-2 rounded transition-colors disabled:opacity-50"
            >
              Previous
            </button>
            <span className="font-body-sm text-body-sm text-on-surface-variant self-center">Page {txPage} of {txHistory.pages}</span>
            <button
              onClick={() => setTxPage(p => p + 1)}
              disabled={txPage >= txHistory.pages}
              className="bg-surface border border-outline-variant hover:bg-surface-container-highest text-on-surface font-label-caps text-label-caps px-4 py-2 rounded transition-colors disabled:opacity-50"
            >
              Next
            </button>
          </div>
        )}
      </section>
    </>
  )
}

function ContractInteraction({ address }: { address: string }) {
  const [tab, setTab] = useState<'read' | 'write'>('read')
  const [func, setFunc] = useState('')
  const [args, setArgs] = useState('')
  const [result, setResult] = useState('')

  const funcs = tab === 'read'
    ? ['balanceOf(address)', 'totalSupply()', 'decimals()', 'symbol()', 'name()', 'owner()']
    : ['transfer(address,uint256)', 'approve(address,uint256)']

  return (
    <div>
      <div className="flex gap-2 mb-4">
        <button
          onClick={() => setTab('read')}
          className={`px-4 py-2 rounded font-label-caps text-label-caps transition-colors ${tab === 'read' ? 'bg-primary text-on-primary' : 'bg-surface text-on-surface border border-outline-variant'}`}
        >
          Read Contract
        </button>
        <button
          onClick={() => setTab('write')}
          className={`px-4 py-2 rounded font-label-caps text-label-caps transition-colors ${tab === 'write' ? 'bg-primary text-on-primary' : 'bg-surface text-on-surface border border-outline-variant'}`}
        >
          Write Contract
        </button>
      </div>

      <div className="flex gap-2 mb-4 flex-wrap">
        {funcs.map(f => (
          <button
            key={f}
            onClick={() => setFunc(f)}
            className={`font-mono-sm text-mono-sm px-2 py-1 rounded border transition-colors ${func === f ? 'bg-primary-fixed-dim text-on-primary border-primary' : 'bg-surface text-on-surface-variant border-outline-variant hover:border-primary'}`}
          >
            {f}
          </button>
        ))}
      </div>

      <div className="flex gap-2 items-start">
        <div className="flex-1">
          <input
            type="text"
            value={func}
            readOnly
            placeholder="function"
            className="w-full bg-surface border border-outline-variant rounded p-2 font-mono-sm text-mono-sm text-on-surface mb-2"
          />
          <input
            type="text"
            value={args}
            onChange={e => setArgs(e.target.value)}
            placeholder="arg1, arg2, ..."
            className="w-full bg-surface border border-outline-variant rounded p-2 font-mono-sm text-mono-sm text-on-surface"
          />
        </div>
        <div className="flex flex-col gap-2">
          <button
            onClick={() => setResult('Query submitted (requires backend eth_call)')}
            className="bg-primary hover:bg-primary-fixed-dim text-on-primary font-label-caps text-label-caps px-4 py-2 rounded transition-colors"
          >
            {tab === 'read' ? 'Query' : 'Write'}
          </button>
        </div>
      </div>

      {result && (
        <div className="mt-4 bg-surface-container-low border border-outline-variant rounded p-3">
          <div className="font-label-caps text-label-caps text-on-surface-variant mb-1">Result</div>
          <div className="font-mono-sm text-mono-sm text-on-surface break-all">{result}</div>
        </div>
      )}
    </div>
  )
}
