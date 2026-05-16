import type { Metadata } from 'next'
import './globals.css'
import Header from '@/components/Header'
import Footer from '@/components/Footer'

export const metadata: Metadata = {
  title: 'Viri Explorer',
  description: 'Viri Blockchain Testnet Explorer',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark">
      <body className="bg-background text-on-background min-h-screen flex flex-col font-body-md text-body-md">
        <Header />
        <main className="flex-grow w-full px-gutter max-w-container-max mx-auto py-margin-page space-y-stack-lg">
          {children}
        </main>
        <Footer />
      </body>
    </html>
  )
}
