'use client'

import { useState } from 'react'
import { useFaucetInfo, claimFaucet } from '@/lib/api'
import StatCard from '@/components/StatCard'

export default function FaucetPage() {
  const { data: info } = useFaucetInfo()
  const [address, setAddress] = useState('')
  const [claiming, setClaiming] = useState(false)
  const [result, setResult] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  const handleClaim = async () => {
    if (!address.trim()) return
    setClaiming(true)
    setResult(null)
    try {
      const res = await claimFaucet(address.trim())
      if (res.success) {
        setResult({ type: 'success', message: `10 VIRI sent! Tx: ${res.txHash || ''}` })
      } else {
        setResult({ type: 'error', message: res.error || 'Claim failed' })
      }
    } catch (e: unknown) {
      setResult({ type: 'error', message: e instanceof Error ? e.message : 'Request failed' })
    } finally {
      setClaiming(false)
    }
  }

  return (
    <>
      <header className="mb-stack-sm">
        <h1 className="font-headline-xl text-headline-xl text-on-surface">Viri Faucet</h1>
        <p className="font-body-md text-body-md text-on-surface-variant mt-2">
          Get free testnet VIRI tokens to interact with the Viri network.
        </p>
      </header>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-gutter">
        {/* Left: Input Form */}
        <div className="lg:col-span-8 flex flex-col gap-stack-md">
          <div className="bg-surface-container border border-outline-variant rounded-lg p-stack-lg">
            <label className="block font-label-caps text-label-caps text-on-surface-variant mb-stack-sm" htmlFor="wallet-address">
              Enter Wallet Address
            </label>
            <div className="relative">
              <input
                id="wallet-address"
                type="text"
                value={address}
                onChange={e => setAddress(e.target.value)}
                placeholder="0x..."
                className="w-full bg-surface border border-outline-variant rounded p-stack-md font-mono-base text-mono-base text-on-surface placeholder:text-on-surface-variant focus:border-primary focus:ring-1 focus:ring-primary outline-none transition-colors"
              />
            </div>
            <div className="mt-stack-md flex justify-between items-center">
              <p className="font-body-sm text-body-sm text-on-surface-variant">
                Tokens will be sent immediately upon request.
              </p>
              <button
                onClick={handleClaim}
                disabled={claiming || !address.trim()}
                className="bg-primary hover:bg-primary-fixed-dim text-on-primary font-label-caps text-label-caps px-stack-lg py-stack-sm rounded transition-colors flex items-center gap-2 disabled:opacity-50"
              >
                {claiming ? 'Sending...' : 'Request Tokens'}
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z" />
                </svg>
              </button>
            </div>
          </div>

          {/* Alert messages */}
          {result && (
            <div className={`border rounded p-stack-md flex items-start gap-stack-sm ${
              result.type === 'success'
                ? 'bg-[#002113] border-tertiary'
                : 'bg-error-container border-error'
            }`}>
              {result.type === 'success' ? (
                <svg className="w-5 h-5 text-tertiary mt-0.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              ) : (
                <svg className="w-5 h-5 text-error mt-0.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              )}
              <div>
                <h4 className={`font-label-caps text-label-caps mb-1 ${result.type === 'success' ? 'text-tertiary' : 'text-error'}`}>
                  {result.type === 'success' ? 'Transaction Successful' : 'Request Failed'}
                </h4>
                <p className={`font-body-sm text-body-sm opacity-80 ${result.type === 'success' ? 'text-tertiary-fixed' : 'text-on-error-container'}`}>
                  {result.message}
                </p>
              </div>
            </div>
          )}
        </div>

        {/* Right: Stats */}
        <div className="lg:col-span-4 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-1 gap-stack-md h-fit">
          <StatCard
            label="Per Claim"
            value={`${info?.perClaim || 10} VIRI`}
            accentColor="border-l-primary"
          />
          <StatCard
            label="Daily Limit"
            value={`${info?.dailyLimit || 50} VIRI`}
            accentColor="border-l-secondary"
          />
          <StatCard
            label="Cooldown"
            value={`${info?.cooldownHours || 24} Hours`}
            accentColor="border-l-tertiary"
          />
          <div className="bg-surface-container border border-outline-variant border-t-4 border-t-outline-variant rounded-lg p-stack-md relative overflow-hidden group">
            <div className="flex justify-between items-start mb-stack-sm relative z-10">
              <span className="font-label-caps text-label-caps text-on-surface-variant">Faucet Balance</span>
            </div>
            <div className="font-mono-base text-mono-base text-on-surface relative z-10 truncate">
              {info?.balance?.toLocaleString() || '—'} VIRI
            </div>
            <div className="w-full bg-surface-bright h-1.5 rounded-full mt-stack-sm overflow-hidden">
              <div className="bg-primary h-full rounded-full" style={{ width: '85%' }}></div>
            </div>
          </div>
        </div>
      </div>
    </>
  )
}
