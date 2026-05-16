'use client'

import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import { useState } from 'react'
import { search } from '@/lib/api'

const navItems = [
  { href: '/blocks', label: 'Blocks' },
  { href: '/validators', label: 'Validators' },
  { href: '/network', label: 'Network' },
  { href: '/faucet', label: 'Faucet' },
]

export default function Header() {
  const pathname = usePathname()

  return (
    <header className="bg-surface border-b border-outline-variant w-full sticky top-0 z-50">
      <div className="flex justify-between items-center w-full px-gutter max-w-container-max mx-auto h-16">
        <div className="flex items-center gap-stack-lg">
          <Link href="/" className="text-headline-lg font-headline-lg font-bold text-on-surface">
            Viri Explorer
          </Link>
          <nav className="hidden md:flex gap-stack-md">
            {navItems.map(item => {
              const isActive = pathname?.startsWith(item.href)
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`font-label-caps text-label-caps px-3 py-2 rounded transition-colors ${
                    isActive
                      ? 'text-primary border-b-2 border-primary pb-1'
                      : 'text-on-surface-variant hover:text-on-surface hover:bg-surface-container-high'
                  }`}
                >
                  {item.label}
                </Link>
              )
            })}
          </nav>
        </div>
        <div className="flex items-center gap-stack-md">
          <SearchBar />
          <span className="font-label-caps text-label-caps text-on-surface-variant border border-outline-variant rounded px-2 py-1 hidden sm:block">
            Viri Testnet
          </span>
        </div>
      </div>
    </header>
  )
}

function SearchBar() {
  const router = useRouter()
  const [query, setQuery] = useState('')
  const [searching, setSearching] = useState(false)
  const [error, setError] = useState('')

  const handleSearch = async (e: React.FormEvent | React.KeyboardEvent) => {
    e.preventDefault()
    const q = query.trim()
    if (!q) return

    setSearching(true)
    setError('')

    try {
      const result = await search(q)
      if (result === 'block') {
        router.push(`/blocks/${q}`)
      } else if (result === 'tx') {
        router.push(`/tx/${q}`)
      } else if (result === 'address') {
        router.push(`/address/${q}`)
      } else {
        setError('Not found')
      }
    } catch {
      setError('Search failed')
    } finally {
      setSearching(false)
    }
  }

  return (
    <form onSubmit={handleSearch} className="hidden lg:flex items-center bg-surface-container border border-outline-variant rounded-full px-4 py-2 focus-within:border-primary transition-colors">
      <svg className="w-4 h-4 text-on-surface-variant mr-2 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
      </svg>
      <input
        className="bg-transparent border-none outline-none text-body-sm font-body-sm text-on-surface placeholder-on-surface-variant w-64 focus:ring-0 p-0"
        placeholder="Search by Address / Txn Hash / Block"
        value={query}
        onChange={e => { setQuery(e.target.value); setError('') }}
        onKeyDown={e => { if (e.key === 'Enter') handleSearch(e) }}
        type="text"
      />
      {searching && <span className="text-on-surface-variant text-sm ml-2">...</span>}
      {error && <span className="text-error text-sm ml-2">{error}</span>}
    </form>
  )
}
