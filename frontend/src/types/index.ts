export interface Block {
  number: number
  hash: string
  parentHash: string
  timestamp: number
  transactions: Tx[]
  transactionsCount: number
  proposer: string
  gasUsed: number
  gasLimit: number
  size: number
  stateRoot: string
  receiptsRoot?: string
}

export interface Tx {
  hash: string
  from: string
  to: string
  value: string
  gasPrice: string
  gasLimit: number
  gasUsed: number
  nonce: number
  status: 'success' | 'pending' | 'failed'
  timestamp: number
  blockNumber: number
  input: string
  logs?: Log[]
  contractAddress?: string
  transactionIndex?: number
  blockHash?: string
}

export interface Log {
  address: string
  topics: string[]
  data: string
}

export interface Peer {
  id: string
  address: string
  status: string
  latency: number
  peer_id?: string
}

export interface NetworkStatus {
  blockHeight: number
  peers: number
  gasPrice: string
  chainId: number
  networkName: string
  avgBlockTime: string
  uptime: number
}

export interface FaucetInfo {
  perClaim: number
  dailyLimit: number
  cooldownHours: number
  balance: number
}

export interface FaucetClaimResponse {
  success: boolean
  txHash?: string
  error?: string
}

export interface AccountInfo {
  address: string
  balance: string
  nonce: number
  code: string
}

export interface ApiResponse<T> {
  data?: T
  error?: string
}

export interface NodeInfo {
  version: string
  chain_id: number
  peer_id: string
  full_peer_id: string
  multiaddr: string
  peers: number
  height: number
  listening: boolean
  validator: boolean
}

export interface ConsensusState {
  height: number
  view?: number
  round?: number
  phase?: string
  proposer?: string
  validators?: number
  locked_block?: string
  committed_block?: string
}

export interface IndexedTx {
  _id?: string
  hash: string
  block_number: number
  block_hash: string
  from: string
  to: string
  value: string
  gas_price: string
  gas_limit: number
  gas_used: number
  nonce: number
  status: string
  input: string
  index: number
  imported_at?: string
}

export interface IndexerStatus {
  synced_block: number
  total_blocks: number
  total_txs: number
  status: string
  updated_at: string
}

export interface AddressTxsResponse {
  transactions: IndexedTx[]
  total: number
  page: number
  limit: number
  pages: number
}
