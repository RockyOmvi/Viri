package ledger

import (
	"sort"
	"sync"

	"github.com/viri-chain/viri/internal/layer1/state"
)

type TxStatus uint8

const (
	TxStatusPending TxStatus = iota
	TxStatusProcessing
	TxStatusConfirmed
	TxStatusDropped
	TxStatusReplaced
)

type TxPoolConfig struct {
	MaxTransactions int
	MaxGas          uint64
	MinGasPrice     uint64
	MaxAccountTxs   int
}

func DefaultTxPoolConfig() *TxPoolConfig {
	return &TxPoolConfig{
		MaxTransactions: 10_000,
		MaxGas:          30_000_000,
		MinGasPrice:     1,
		MaxAccountTxs:   100,
	}
}

type TxPool struct {
	mu         sync.RWMutex
	config     *TxPoolConfig
	pending    map[string]*Transaction
	queued     map[string][]*Transaction
	byGasPrice []*Transaction
	state      *state.StateManager
	txCounter  int
}

func NewTxPool(config *TxPoolConfig, stateMgr *state.StateManager) *TxPool {
	if config == nil {
		config = DefaultTxPoolConfig()
	}

	pool := &TxPool{
		config:     config,
		pending:    make(map[string]*Transaction),
		queued:     make(map[string][]*Transaction),
		byGasPrice: make([]*Transaction, 0),
		state:      stateMgr,
	}

	return pool
}

func (p *TxPool) Add(tx *Transaction) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.pending) >= p.config.MaxTransactions {
		return ErrTxPoolFull
	}

	txHash := string(tx.Hash)
	if _, exists := p.pending[txHash]; exists {
		return ErrDuplicateTx
	}

	if tx.GasPrice < p.config.MinGasPrice {
		return ErrGasPriceTooLow
	}

	sender := tx.SenderAddress()
	accountTxs := p.countAccountTxs(string(sender))
	if accountTxs >= p.config.MaxAccountTxs {
		return ErrAccountTxLimit
	}

	if p.state != nil {
		nonce, err := p.state.GetNonce(sender)
		if err == nil && tx.Nonce < nonce {
			return ErrNonceTooLow
		}
	}

	p.pending[txHash] = tx
	p.byGasPrice = append(p.byGasPrice, tx)
	sort.Slice(p.byGasPrice, func(i, j int) bool {
		return p.byGasPrice[i].GasPrice > p.byGasPrice[j].GasPrice
	})

	p.txCounter++
	return nil
}

func (p *TxPool) GetPending() []*Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	txs := make([]*Transaction, 0, len(p.pending))
	for _, tx := range p.pending {
		txs = append(txs, tx)
	}

	sort.Slice(txs, func(i, j int) bool {
		return txs[i].GasPrice > txs[j].GasPrice
	})

	return txs
}

func (p *TxPool) GetPendingByAccount(account []byte) []*Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var txs []*Transaction
	for _, tx := range p.pending {
		if string(tx.SenderAddress()) == string(account) {
			txs = append(txs, tx)
		}
	}

	sort.Slice(txs, func(i, j int) bool {
		return txs[i].Nonce < txs[j].Nonce
	})

	return txs
}

func (p *TxPool) Remove(txHash string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.removeLocked(txHash)
}

func (p *TxPool) RemoveConfirmed(txs []*Transaction) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, tx := range txs {
		p.removeLocked(string(tx.Hash))
	}
}

func (p *TxPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.pending)
}

func (p *TxPool) GasUsed() uint64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var total uint64
	for _, tx := range p.pending {
		total += tx.GasLimit
	}
	return total
}

func (p *TxPool) Has(txHash string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, exists := p.pending[txHash]
	return exists
}

func (p *TxPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.pending = make(map[string]*Transaction)
	p.queued = make(map[string][]*Transaction)
	p.byGasPrice = make([]*Transaction, 0)
}

func (p *TxPool) Stats() TxPoolStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return TxPoolStats{
		PendingCount:    len(p.pending),
		QueuedCount:     p.countQueued(),
		TotalGas:        p.gasUsedLocked(),
		UniqueAccounts:  p.uniqueAccountsLocked(),
		TotalProcessed:  p.txCounter,
	}
}

func (p *TxPool) removeLocked(txHash string) {
	if _, exists := p.pending[txHash]; exists {
		delete(p.pending, txHash)

		for i, t := range p.byGasPrice {
			if string(t.Hash) == txHash {
				p.byGasPrice = append(p.byGasPrice[:i], p.byGasPrice[i+1:]...)
				break
			}
		}
	}
}

func (p *TxPool) countAccountTxs(account string) int {
	count := 0
	for _, tx := range p.pending {
		if string(tx.SenderAddress()) == account {
			count++
		}
	}
	return count
}

func (p *TxPool) countQueued() int {
	count := 0
	for _, txs := range p.queued {
		count += len(txs)
	}
	return count
}

func (p *TxPool) gasUsedLocked() uint64 {
	var total uint64
	for _, tx := range p.pending {
		total += tx.GasLimit
	}
	return total
}

func (p *TxPool) uniqueAccountsLocked() int {
	accounts := make(map[string]bool)
	for _, tx := range p.pending {
		accounts[string(tx.From)] = true
	}
	return len(accounts)
}

type TxPoolStats struct {
	PendingCount   int
	QueuedCount    int
	TotalGas       uint64
	UniqueAccounts int
	TotalProcessed int
}
