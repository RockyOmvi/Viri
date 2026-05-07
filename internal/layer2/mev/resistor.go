package mev

import (
	"sort"
	"sync"
	"time"
)

type TxOrdering uint8

const (
	TxOrderingFIFO TxOrdering = iota
	TxOrderingGasPrice
	TxOrderingMEVOptimized
)

type PendingTx struct {
	Hash      []byte
	Sender    []byte
	Recipient []byte
	Amount    uint64
	GasPrice  uint64
	Timestamp time.Time
	Data      []byte
}

type MEVResistor struct {
	mu         sync.Mutex
	pending    []*PendingTx
	ordering   TxOrdering
	batchSize  int
	batchDelay time.Duration
	lastBatch  time.Time
}

func NewMEVResistor(ordering TxOrdering, batchSize int, batchDelay time.Duration) *MEVResistor {
	return &MEVResistor{
		ordering:   ordering,
		batchSize:  batchSize,
		batchDelay: batchDelay,
		lastBatch:  time.Now(),
	}
}

func (m *MEVResistor) AddTx(tx *PendingTx) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pending = append(m.pending, tx)
}

func (m *MEVResistor) GetBatch() []*PendingTx {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.pending) == 0 {
		return nil
	}

	now := time.Now()
	if len(m.pending) < m.batchSize && now.Sub(m.lastBatch) < m.batchDelay {
		return nil
	}

	batch := m.pending
	if len(batch) > m.batchSize {
		batch = batch[:m.batchSize]
	}

	m.orderBatch(batch)

	m.pending = m.pending[len(batch):]
	m.lastBatch = now

	return batch
}

func (m *MEVResistor) orderBatch(batch []*PendingTx) {
	switch m.ordering {
	case TxOrderingGasPrice:
		sort.Slice(batch, func(i, j int) bool {
			return batch[i].GasPrice > batch[j].GasPrice
		})
	case TxOrderingMEVOptimized:
		sort.Slice(batch, func(i, j int) bool {
			return batch[i].GasPrice*batch[i].Amount > batch[j].GasPrice*batch[j].Amount
		})
	case TxOrderingFIFO:
	default:
	}
}

func (m *MEVResistor) PendingCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pending)
}

func (m *MEVResistor) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pending = m.pending[:0]
}
