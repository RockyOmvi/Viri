import useSWR from 'swr'
import type { Block, Tx, Peer, NetworkStatus, FaucetInfo, FaucetClaimResponse, AccountInfo, NodeInfo, ConsensusState, AddressTxsResponse, IndexerStatus } from '@/types'

const DEFAULT_RPC = process.env.NEXT_PUBLIC_RPC_URL || 'http://localhost:8545'
const DEFAULT_API = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8546'
const INDEXER_URL = process.env.NEXT_PUBLIC_INDEXER_URL || 'http://localhost:8547'
const API_KEY = process.env.NEXT_PUBLIC_API_KEY || ''

async function rpcCall(method: string, params: unknown[] = []): Promise<unknown> {
  const res = await fetch(DEFAULT_RPC, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(API_KEY ? { 'X-API-Key': API_KEY } : {}) },
    body: JSON.stringify({ jsonrpc: '2.0', id: Date.now(), method, params }),
  })
  const json = await res.json()
  if (json.error) throw new Error(json.error.message || json.error)
  return json.result
}

async function apiGet<T>(path: string, base?: string): Promise<T> {
  const url = (base || DEFAULT_API) + path
  const res = await fetch(url, {
    headers: { ...(API_KEY ? { 'X-API-Key': API_KEY } : {}) },
  })
  if (!res.ok) throw new Error(`API error: ${res.status}`)
  const json = await res.json()
  if (json.error) throw new Error(json.error)
  return json.data || json
}

async function apiPost<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${DEFAULT_API}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(API_KEY ? { 'X-API-Key': API_KEY } : {}) },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`API error: ${res.status}`)
  const json = await res.json()
  if (json.error) throw new Error(json.error)
  return json.data || json
}

const fetcher = (url: string) => fetch(url).then(r => r.json())

// Network Status
export function useStatus(): { data: NetworkStatus | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(`${DEFAULT_API}/api/v1/status`, fetcher, {
    refreshInterval: 5000,
  })
  return { data: data?.data || data, error, isLoading }
}

// Blocks
export function useBlocks(limit = 15): { data: Block[] | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(`${DEFAULT_RPC}?method=eth_getBlockByNumber&limit=${limit}`, async () => {
    const blockNum = await rpcCall('eth_blockNumber')
    const num = parseInt(blockNum as string, 16)
    const promises = []
    for (let i = 0; i < limit && num - i >= 0; i++) {
      promises.push(rpcCall('eth_getBlockByNumber', [`0x${(num - i).toString(16)}`, false]))
    }
    const blocks = await Promise.all(promises)
    return blocks.map(b => normalizeBlock(b as Record<string, unknown>))
  }, { refreshInterval: 5000 })
  return { data, error, isLoading }
}

export function useBlocksOffset(page = 1, limit = 15): { data: Block[] | undefined; total: number; pages: number; error: Error | undefined; isLoading: boolean } {
  const from = Math.max(0, page * limit)
  const { data, error, isLoading } = useSWR(
    `${DEFAULT_API}/api/v1/blocks?from=${from}&limit=${limit}`,
    async (url: string) => {
      const json = await fetcher(url)
      const rawBlocks = json?.blocks || json?.data?.blocks || []
      const total = json?.total || json?.data?.total || 0
      const pages = Math.ceil(total / limit) || 1
      return {
        blocks: rawBlocks.map((b: Record<string, unknown>) => normalizeBlockFromREST(b)),
        total,
        pages,
      }
    },
    { refreshInterval: 10000 }
  )
  return {
    data: data?.blocks,
    total: data?.total || 0,
    pages: data?.pages || 1,
    error,
    isLoading,
  }
}

export function useBlockByNumber(number: number): { data: Block | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(
    `${DEFAULT_RPC}?method=eth_getBlockByNumber&number=${number}`,
    async () => {
      const [block, receipts] = await Promise.all([
        rpcCall('eth_getBlockByNumber', [`0x${number.toString(16)}`, true]),
        rpcCall('eth_getBlockReceipts', [`0x${number.toString(16)}`]).catch(() => null),
      ])
      const normalized = normalizeBlock(block as Record<string, unknown>)

      if (receipts) {
        const receiptList = receipts as Record<string, unknown>[]
        normalized.transactions = normalized.transactions.map((tx, i) => {
          const r = receiptList[i] || {}
          const txReceipt = r as Record<string, unknown>
          const status = txReceipt.status === '0x1' ? 'success' : txReceipt.status === '0x0' ? 'failed' : tx.status
          const rawLogs = (txReceipt.logs || []) as Record<string, unknown>[]
          return {
            ...tx,
            status,
            gasUsed: txReceipt.gasUsed ? parseInt(txReceipt.gasUsed as string, 16) : tx.gasUsed,
            contractAddress: (txReceipt.contractAddress as string) || '',
            logs: rawLogs.map(l => ({
              address: l.address as string,
              topics: (l.topics as string[]) || [],
              data: l.data as string,
            })),
          }
        })
      }

      return normalized
    }
  )
  return { data, error, isLoading }
}

export function useRecentBlocks(count = 10): { data: Block[] | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(`${DEFAULT_API}/api/v1/blocks?limit=${count}`, fetcher, {
    refreshInterval: 5000,
  })
  const raw = data?.blocks || data?.data?.blocks || data
  const blocks = Array.isArray(raw) ? raw.map((b: Record<string, unknown>) => normalizeBlockFromREST(b)) : undefined
  return { data: blocks, error, isLoading }
}

// Transactions
export function useTxByHash(hash: string): { data: Tx | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(
    hash ? `${DEFAULT_RPC}?method=eth_getTransactionByHash&hash=${hash}` : null,
    async () => {
      const [tx, receipt] = await Promise.all([
        rpcCall('eth_getTransactionByHash', [hash]),
        rpcCall('eth_getTransactionReceipt', [hash]).catch(() => null),
      ])
      if (!tx) throw new Error('Transaction not found')
      return normalizeTx(tx as Record<string, unknown>, receipt as Record<string, unknown> || {})
    }
  )
  return { data, error, isLoading }
}

export function useRecentTxs(count = 10): { data: Tx[] | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(`${DEFAULT_API}/api/v1/transactions?limit=${count}`, fetcher, {
    refreshInterval: 5000,
  })
  const txs = data?.transactions || data?.data?.transactions
  return { data: txs, error, isLoading }
}

// Account
export function useAccount(address: string): { data: AccountInfo | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(
    address ? `${DEFAULT_RPC}?method=eth_getBalance&address=${address}` : null,
    async () => {
      const [balance, nonce, code] = await Promise.all([
        rpcCall('eth_getBalance', [address, 'latest']),
        rpcCall('eth_getTransactionCount', [address, 'latest']),
        rpcCall('eth_getCode', [address, 'latest']),
      ])
      return {
        address,
        balance: (balance as string),
        nonce: parseInt(nonce as string, 16),
        code: (code as string),
      }
    }
  )
  return { data, error, isLoading }
}

// Address transaction history (from indexer)
export function useAddressTxs(address: string, page = 1, limit = 20): { data: AddressTxsResponse | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(
    address ? `${INDEXER_URL}/api/v1/address/${address}?page=${page}&limit=${limit}` : null,
    fetcher,
    { refreshInterval: 15000 }
  )
  return { data, error, isLoading }
}

// Peers
export function usePeers(): { data: Peer[] | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(`${DEFAULT_API}/api/v1/peers`, fetcher, {
    refreshInterval: 10000,
  })
  const peers = data?.peers || data?.data?.peers
  return { data: peers, error, isLoading }
}

// Faucet
export function useFaucetInfo(): { data: FaucetInfo | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(`${DEFAULT_API}/api/v1/faucet/info`, fetcher, {
    refreshInterval: 30000,
  })
  return { data: data?.data || data, error, isLoading }
}

export async function claimFaucet(address: string): Promise<FaucetClaimResponse> {
  return apiPost<FaucetClaimResponse>('/api/v1/faucet/claim', { address })
}

// Node Info
export function useNodeInfo(): { data: NodeInfo | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(
    'viri_nodeInfo',
    async () => {
      const result = await rpcCall('viri_nodeInfo')
      return result as NodeInfo
    },
    { refreshInterval: 15000 }
  )
  return { data, error, isLoading }
}

// Consensus State
export function useConsensusState(): { data: ConsensusState | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(
    'viri_getConsensusState',
    async () => {
      const result = await rpcCall('viri_getConsensusState')
      return result as ConsensusState
    },
    { refreshInterval: 10000 }
  )
  return { data, error, isLoading }
}

// Indexer Status
export function useIndexerStatus(): { data: IndexerStatus | undefined; error: Error | undefined; isLoading: boolean } {
  const { data, error, isLoading } = useSWR(`${INDEXER_URL}/api/v1/status`, fetcher, {
    refreshInterval: 15000,
  })
  return { data, error, isLoading }
}

// Search - tries block number, tx hash, address
export async function search(query: string): Promise<'block' | 'tx' | 'address' | null> {
  const q = query.trim()
  if (!q) return null

  if (/^\d+$/.test(q)) {
    try {
      const block = await rpcCall('eth_getBlockByNumber', [`0x${parseInt(q).toString(16)}`, false])
      if (block) return 'block'
    } catch { /* not a block */ }
  }

  if (/^0x[0-9a-fA-F]{64}$/.test(q)) {
    try {
      const tx = await rpcCall('eth_getTransactionByHash', [q])
      if (tx) return 'tx'
    } catch { /* not a tx */ }
    try {
      const block = await rpcCall('eth_getBlockByHash', [q])
      if (block) return 'block'
    } catch { /* not a block hash */ }
  }

  if (/^0x[0-9a-fA-F]{40}$/.test(q)) {
    return 'address'
  }

  return null
}

// Helpers
function normalizeBlock(raw: Record<string, unknown>): Block {
  const txs = (raw.transactions as unknown[]) || []
  return {
    number: parseInt(raw.number as string, 16),
    hash: raw.hash as string,
    parentHash: raw.parentHash as string,
    timestamp: parseInt(raw.timestamp as string, 16),
    transactions: txs.map(t => normalizeTx(t as Record<string, unknown>, {})),
    transactionsCount: txs.length,
    proposer: (raw.miner || raw.author || '') as string,
    gasUsed: parseInt(raw.gasUsed as string, 16),
    gasLimit: parseInt(raw.gasLimit as string, 16),
    size: parseInt(raw.size as string, 16),
    stateRoot: raw.stateRoot as string,
    receiptsRoot: raw.receiptsRoot as string,
  }
}

function normalizeBlockFromREST(raw: Record<string, unknown>): Block {
  return {
    number: parseInt(raw.number as string) || parseInt(raw.height as string) || 0,
    hash: raw.hash as string || '',
    parentHash: raw.parentHash as string || '',
    timestamp: typeof raw.timestamp === 'number' ? (raw.timestamp as number) : parseInt(raw.timestamp as string) || 0,
    transactions: [],
    transactionsCount: (raw.tx_count as number) || (raw.txCount as number) || parseInt(raw.transactionsCount as string) || 0,
    proposer: (raw.miner || raw.proposer || '') as string,
    gasUsed: parseInt(raw.gasUsed as string) || 0,
    gasLimit: parseInt(raw.gasLimit as string) || 0,
    size: parseInt(raw.size as string) || 0,
    stateRoot: raw.stateRoot as string || '',
  }
}

function normalizeTx(tx: Record<string, unknown>, receipt: Record<string, unknown>): Tx {
  const rawLogs = (receipt.logs || []) as Record<string, unknown>[]
  const status = receipt.status === undefined ? 'pending'
    : receipt.status === '0x1' ? 'success' : 'failed'
  return {
    hash: tx.hash as string,
    from: tx.from as string,
    to: tx.to as string || '',
    value: tx.value as string,
    gasPrice: tx.gasPrice as string,
    gasLimit: parseInt(tx.gas as string, 16),
    gasUsed: receipt.gasUsed ? parseInt(receipt.gasUsed as string, 16) : 0,
    nonce: parseInt(tx.nonce as string, 16),
    status,
    timestamp: 0,
    blockNumber: parseInt(tx.blockNumber as string, 16),
    input: tx.input as string,
    transactionIndex: receipt.transactionIndex ? parseInt(receipt.transactionIndex as string, 16) : undefined,
    blockHash: tx.blockHash as string,
    contractAddress: receipt.contractAddress as string || '',
    logs: rawLogs.map(l => ({
      address: l.address as string,
      topics: (l.topics as string[]) || [],
      data: l.data as string,
    })),
  }
}
