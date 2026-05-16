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

type PressureCallback func(level float64)

type TxPool struct {
	mu             sync.RWMutex
	config         *TxPoolConfig
	pending        map[string]*Transaction
	queued         map[string][]*Transaction
	byGasPrice     []*Transaction
	state          *state.StateManager
	txCounter      int
	pressureCB     PressureCallback
	lastPressureLvl float64
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

	// Evict lowest-gas-price tx if pool is full (replace-by-fee)
	if len(p.pending) >= p.config.MaxTransactions {
		if len(p.byGasPrice) > 0 && tx.GasPrice > p.byGasPrice[len(p.byGasPrice)-1].GasPrice {
			evictHash := string(p.byGasPrice[len(p.byGasPrice)-1].Hash)
			p.removeLocked(evictHash)
		} else {
			return ErrTxPoolFull
		}
	}

	p.pending[txHash] = tx
	p.byGasPrice = append(p.byGasPrice, tx)
	sort.Slice(p.byGasPrice, func(i, j int) bool {
		return p.byGasPrice[i].GasPrice > p.byGasPrice[j].GasPrice
	})

	p.txCounter++
	p.notifyPressure()
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

func (p *TxPool) PressureLevel() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.config.MaxTransactions == 0 {
		return 0
	}
	return float64(len(p.pending)) / float64(p.config.MaxTransactions)
}

func (p *TxPool) SetPressureCallback(cb PressureCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pressureCB = cb
}

func (p *TxPool) Evict(count int) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	removed := 0
	for i := len(p.byGasPrice) - 1; i >= 0 && removed < count; i-- {
		hash := string(p.byGasPrice[i].Hash)
		if _, exists := p.pending[hash]; exists {
			delete(p.pending, hash)
			removed++
		}
	}
	// Rebuild sorted list
	p.rebuildByGasPrice()
	p.notifyPressure()
	return removed
}

func (p *TxPool) notifyPressure() {
	if p.pressureCB == nil {
		return
	}
	// Called with mu.Lock() held — compute pressure inline to avoid
	// RLock-acquiring-while-Lock-held deadlock on the RWMutex.
	maxTx := p.config.MaxTransactions
	var lvl float64
	if maxTx > 0 {
		lvl = float64(len(p.pending)) / float64(maxTx)
	}
	if lvl != p.lastPressureLvl {
		p.lastPressureLvl = lvl
		p.pressureCB(lvl)
	}
}

func (p *TxPool) rebuildByGasPrice() {
	p.byGasPrice = make([]*Transaction, 0, len(p.pending))
	for _, tx := range p.pending {
		p.byGasPrice = append(p.byGasPrice, tx)
	}
	sort.Slice(p.byGasPrice, func(i, j int) bool {
		return p.byGasPrice[i].GasPrice > p.byGasPrice[j].GasPrice
	})
}

type TxPoolStats struct {
	PendingCount   int
	QueuedCount    int
	TotalGas       uint64
	UniqueAccounts int
	TotalProcessed int
}
