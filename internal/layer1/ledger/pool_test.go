package ledger

import (
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func TestTxPoolAdd(t *testing.T) {
	pool := NewTxPool(nil, nil)

	key, _ := crypto.GenerateKey()
	tx, _ := NewTransactionFromKey(0, key.PubKey().Address(), 100, 1000, 10, nil, 1337, key)

	if err := pool.Add(tx); err != nil {
		t.Fatalf("Failed to add tx to pool: %v", err)
	}

	if pool.Size() != 1 {
		t.Errorf("Expected pool size 1, got %d", pool.Size())
	}

	if !pool.Has(string(tx.Hash)) {
		t.Error("Pool should contain the transaction")
	}
}

func TestTxPoolDuplicate(t *testing.T) {
	pool := NewTxPool(nil, nil)

	key, _ := crypto.GenerateKey()
	tx, _ := NewTransactionFromKey(0, key.PubKey().Address(), 100, 1000, 10, nil, 1337, key)

	if err := pool.Add(tx); err != nil {
		t.Fatalf("First add failed: %v", err)
	}

	if err := pool.Add(tx); err != ErrDuplicateTx {
		t.Errorf("Expected duplicate error, got %v", err)
	}
}

func TestTxPoolGasPriceFilter(t *testing.T) {
	config := DefaultTxPoolConfig()
	config.MinGasPrice = 100
	pool := NewTxPool(config, nil)

	key, _ := crypto.GenerateKey()
	tx, _ := NewTransactionFromKey(0, key.PubKey().Address(), 100, 1000, 50, nil, 1337, key)

	if err := pool.Add(tx); err != ErrGasPriceTooLow {
		t.Errorf("Expected gas price too low error, got %v", err)
	}
}

func TestTxPoolGasPriceOrdering(t *testing.T) {
	pool := NewTxPool(nil, nil)

	key1, _ := crypto.GenerateKey()
	key2, _ := crypto.GenerateKey()

	tx1, _ := NewTransactionFromKey(0, key1.PubKey().Address(), 100, 1000, 10, nil, 1337, key1)
	tx2, _ := NewTransactionFromKey(0, key2.PubKey().Address(), 200, 1000, 20, nil, 1337, key2)

	pool.Add(tx1)
	pool.Add(tx2)

	pending := pool.GetPending()
	if len(pending) != 2 {
		t.Fatalf("Expected 2 pending txs, got %d", len(pending))
	}

	if pending[0].GasPrice < pending[1].GasPrice {
		t.Error("Transactions should be ordered by gas price (highest first)")
	}
}

func TestTxPoolRemove(t *testing.T) {
	pool := NewTxPool(nil, nil)

	key, _ := crypto.GenerateKey()
	tx, _ := NewTransactionFromKey(0, key.PubKey().Address(), 100, 1000, 10, nil, 1337, key)

	pool.Add(tx)
	pool.Remove(string(tx.Hash))

	if pool.Size() != 0 {
		t.Errorf("Expected pool size 0 after removal, got %d", pool.Size())
	}
}

func TestTxPoolRemoveConfirmed(t *testing.T) {
	pool := NewTxPool(nil, nil)

	key1, _ := crypto.GenerateKey()
	key2, _ := crypto.GenerateKey()

	tx1, _ := NewTransactionFromKey(0, key1.PubKey().Address(), 100, 1000, 10, nil, 1337, key1)
	tx2, _ := NewTransactionFromKey(0, key2.PubKey().Address(), 200, 1000, 20, nil, 1337, key2)

	pool.Add(tx1)
	pool.Add(tx2)

	pool.RemoveConfirmed([]*Transaction{tx1})

	if pool.Size() != 1 {
		t.Errorf("Expected pool size 1, got %d", pool.Size())
	}

	if !pool.Has(string(tx2.Hash)) {
		t.Error("tx2 should still be in pool")
	}
}

func TestTxPoolClear(t *testing.T) {
	pool := NewTxPool(nil, nil)

	for i := 0; i < 10; i++ {
		key, _ := crypto.GenerateKey()
		tx, _ := NewTransactionFromKey(uint64(i), key.PubKey().Address(), 100, 1000, 10, nil, 1337, key)
		pool.Add(tx)
	}

	if pool.Size() != 10 {
		t.Fatalf("Expected 10 txs, got %d", pool.Size())
	}

	pool.Clear()

	if pool.Size() != 0 {
		t.Errorf("Expected 0 txs after clear, got %d", pool.Size())
	}
}

func TestTxPoolStats(t *testing.T) {
	pool := NewTxPool(nil, nil)

	key, _ := crypto.GenerateKey()
	tx, _ := NewTransactionFromKey(0, key.PubKey().Address(), 100, 1000, 10, nil, 1337, key)

	pool.Add(tx)

	stats := pool.Stats()

	if stats.PendingCount != 1 {
		t.Errorf("Expected pending count 1, got %d", stats.PendingCount)
	}

	if stats.TotalProcessed != 1 {
		t.Errorf("Expected total processed 1, got %d", stats.TotalProcessed)
	}
}

func TestTxPoolMaxTransactions(t *testing.T) {
	config := DefaultTxPoolConfig()
	config.MaxTransactions = 2
	pool := NewTxPool(config, nil)

	for i := 0; i < 3; i++ {
		key, _ := crypto.GenerateKey()
		tx, _ := NewTransactionFromKey(uint64(i), key.PubKey().Address(), 100, 1000, 10, nil, 1337, key)

		if i < 2 {
			if err := pool.Add(tx); err != nil {
				t.Errorf("Add %d failed: %v", i, err)
			}
		} else {
			if err := pool.Add(tx); err != ErrTxPoolFull {
				t.Errorf("Expected pool full error, got %v", err)
			}
		}
	}
}

func TestTxPoolGasUsed(t *testing.T) {
	pool := NewTxPool(nil, nil)

	key, _ := crypto.GenerateKey()
	tx, _ := NewTransactionFromKey(0, key.PubKey().Address(), 100, 5000, 10, nil, 1337, key)

	pool.Add(tx)

	if pool.GasUsed() != 5000 {
		t.Errorf("Expected gas used 5000, got %d", pool.GasUsed())
	}
}
