'use client'

import Link from 'next/link'
import HashLink from '@/components/HashLink'
import { useBlocksOffset } from '@/lib/api'
import type { Block } from '@/types'
import { useState } from 'react'

function timeAgo(ts: number): string {
  const now = Math.floor(Date.now() / 1000)
  const seconds = now - ts
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  return `${hours}h ago`
}

export default function BlocksPage() {
  const [page, setPage] = useState(1)
  const limit = 15
  const { data: blocks, total, pages, isLoading } = useBlocksOffset(page, limit)

  return (
    <>
      <h1 className="font-headline-xl text-headline-xl text-on-surface">Blocks</h1>
      <p className="font-body-md text-body-md text-on-surface-variant">
        Explore blocks on the Viri Testnet &mdash; {total.toLocaleString()} total
      </p>

      <section className="bg-surface-container border border-outline-variant rounded-lg overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead className="bg-surface-container-low font-label-caps text-label-caps text-on-surface-variant sticky top-0">
              <tr>
                <th className="p-4 border-b border-outline-variant">Block</th>
                <th className="p-4 border-b border-outline-variant">Hash</th>
                <th className="p-4 border-b border-outline-variant">Txs</th>
                <th className="p-4 border-b border-outline-variant">Proposer</th>
                <th className="p-4 border-b border-outline-variant">Gas Used</th>
                <th className="p-4 border-b border-outline-variant text-right">Age</th>
              </tr>
            </thead>
            <tbody className="font-mono-sm text-mono-sm divide-y divide-outline-variant">
              {blocks?.map((block: Block) => (
                <tr key={block.hash} className="hover:bg-surface-container-highest transition-colors">
                  <td className="p-4">
                    <Link href={`/blocks/${block.number}`} className="text-primary hover:text-primary-fixed-dim">
                      {block.number}
                    </Link>
                  </td>
                  <td className="p-4 text-on-surface"><HashLink hash={block.hash} type="block" /></td>
                  <td className="p-4 text-on-surface">{block.transactionsCount}</td>
                  <td className="p-4 text-secondary"><HashLink hash={block.proposer} type="address" /></td>
                  <td className="p-4 text-on-surface">{block.gasLimit ? `${((block.gasUsed / block.gasLimit) * 100).toFixed(1)}%` : '—'}</td>
                  <td className="p-4 text-on-surface-variant text-right">{block.timestamp ? timeAgo(block.timestamp) : '—'}</td>
                </tr>
              ))}
              {isLoading && (
                <tr><td colSpan={6} className="p-4 text-on-surface-variant text-center">Loading...</td></tr>
              )}
              {(!blocks || blocks.length === 0) && !isLoading && (
                <tr><td colSpan={6} className="p-4 text-on-surface-variant text-center">No blocks found</td></tr>
              )}
            </tbody>
          </table>
        </div>
        {pages > 1 && (
          <div className="flex justify-center gap-4 p-4 border-t border-outline-variant">
            <button
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page <= 1}
              className="bg-surface border border-outline-variant hover:bg-surface-container-highest text-on-surface font-label-caps text-label-caps px-4 py-2 rounded transition-colors disabled:opacity-50"
            >
              Previous
            </button>
            <span className="font-body-sm text-body-sm text-on-surface-variant self-center">Page {page} of {pages}</span>
            <button
              onClick={() => setPage(p => p + 1)}
              disabled={page >= pages}
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
