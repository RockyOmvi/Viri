package execution

import (
	"sync"

	"github.com/viri-chain/viri/internal/layer1/ledger"
)

// DependencyGraph computes independent transaction batches for parallel execution.
type DependencyGraph struct {
	mu     sync.Mutex
	conflicts map[string]struct{} // accounts written by previous batch
}

// NewDependencyGraph creates a dependency tracker.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		conflicts: make(map[string]struct{}),
	}
}

// Batch represents a set of transactions that can execute in parallel.
type Batch []int // indices into the transaction list

// ComputeBatches partitions transactions into dependency-free parallel batches.
func (dg *DependencyGraph) ComputeBatches(txs []*ledger.Transaction) []Batch {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	var batches []Batch
	pending := make([]int, len(txs))
	for i := range txs {
		pending[i] = i
	}

	for len(pending) > 0 {
		var batch Batch
		var next []int
		written := make(map[string]struct{})

		for _, idx := range pending {
			tx := txs[idx]
			accounts := dg.txAccounts(tx)

			// Check conflict with accounts written in this batch
			conflict := false
			for _, acc := range accounts {
				if _, exists := written[acc]; exists {
					conflict = true
					break
				}
			}

			if conflict {
				next = append(next, idx)
			} else {
				batch = append(batch, idx)
				for _, acc := range accounts {
					written[acc] = struct{}{}
				}
			}
		}

		batches = append(batches, batch)
		pending = next
	}

	return batches
}

func (dg *DependencyGraph) txAccounts(tx *ledger.Transaction) []string {
	accounts := []string{string(tx.From)}
	if len(tx.To) > 0 {
		accounts = append(accounts, string(tx.To))
	}
	return accounts
}

// ExecuteBlockParallel executes transactions in dependency-free parallel batches.
func (e *ExecutionEngine) ExecuteBlockParallel(txs []*ledger.Transaction, blockHeight uint64,
	getAccount func([]byte) (*AccountState, error),
	setAccount func([]byte, *AccountState) error) ([]*ExecutionResult, uint64, error) {

	dg := NewDependencyGraph()
	batches := dg.ComputeBatches(txs)

	results := make([]*ExecutionResult, len(txs))
	totalGasUsed := uint64(0)

	for _, batch := range batches {
		var wg sync.WaitGroup
		errCh := make(chan error, len(batch))

		for _, idx := range batch {
			wg.Add(1)
			go func(txIdx int) {
				defer wg.Done()
				result, err := e.ExecuteTransaction(txs[txIdx], blockHeight, getAccount, setAccount)
				if err != nil {
					errCh <- err
					return
				}
				results[txIdx] = result
			}(idx)
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			if err != nil {
				return results, totalGasUsed, err
			}
		}

		for _, idx := range batch {
			if results[idx] != nil {
				totalGasUsed += results[idx].GasUsed
			}
		}
	}

	return results, totalGasUsed, nil
}
