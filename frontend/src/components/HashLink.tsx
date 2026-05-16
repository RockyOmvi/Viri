import Link from 'next/link'

interface HashLinkProps {
  hash: string
  type: 'block' | 'tx' | 'address'
  truncate?: boolean
}

export default function HashLink({ hash, type, truncate = true }: HashLinkProps) {
  const href = type === 'block' ? `/blocks/${hash}`
    : type === 'tx' ? `/tx/${hash}`
    : `/address/${hash}`

  const display = truncate ? `${hash.slice(0, 10)}...${hash.slice(-6)}` : hash

  return (
    <Link href={href} className="text-primary hover:text-primary-fixed-dim font-mono-sm text-mono-sm transition-colors">
      {display}
    </Link>
  )
}
