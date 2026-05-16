import Link from 'next/link'

export default function Footer() {
  return (
    <footer className="bg-surface-container-low border-t border-outline-variant w-full mt-auto">
      <div className="w-full py-stack-lg px-gutter max-w-container-max mx-auto flex flex-col md:flex-row justify-between items-center gap-stack-md">
        <div className="flex items-center gap-2">
          <span className="font-headline-lg text-headline-lg font-bold text-on-surface">Viri</span>
          <span className="text-body-sm font-body-sm text-on-surface-variant">
            &copy; 2024 Viri Testnet. Built for the decentralized future.
          </span>
        </div>
        <nav className="flex gap-4">
          <Link href="#" className="text-on-surface-variant hover:text-tertiary transition-colors font-label-caps text-label-caps">
            Documentation
          </Link>
          <Link href="#" className="text-on-surface-variant hover:text-tertiary transition-colors font-label-caps text-label-caps">
            GitHub
          </Link>
          <Link href="#" className="text-on-surface-variant hover:text-tertiary transition-colors font-label-caps text-label-caps">
            Status
          </Link>
          <Link href="#" className="text-on-surface-variant hover:text-tertiary transition-colors font-label-caps text-label-caps">
            Privacy Policy
          </Link>
        </nav>
      </div>
    </footer>
  )
}
