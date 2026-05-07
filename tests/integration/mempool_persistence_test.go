package integration

import (
	"math/big"
	"os"
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/state"
)

func setupMempoolTest(t *testing.T) (*ledger.MempoolPersister, *ledger.TxPool, *state.StateManager, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "mempool-test")
	if err != nil {
		t.Fatal(err)
	}

	db, err := state.NewBadgerStore(dir)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatal(err)
	}

	sm, err := state.NewStateManager(db)
	if err != nil {
		db.Close()
		os.RemoveAll(dir)
		t.Fatal(err)
	}

	sm.Initialize(new(big.Int).SetUint64(1000000000))

	pool := ledger.NewTxPool(nil, sm)
	persister := ledger.NewMempoolPersister(dir)

	cleanup := func() {
		db.Close()
		os.RemoveAll(dir)
	}

	return persister, pool, sm, cleanup
}

func generateTestTx(t *testing.T, key *crypto.PrivateKey, nonce uint64, toAddr []byte) *ledger.Transaction {
	t.Helper()
	tx, err := ledger.NewTransactionFromKey(nonce, toAddr, 1000, 100000, 1, nil, key)
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

func TestSaveLoadRoundtrip(t *testing.T) {
	persister, pool, sm, cleanup := setupMempoolTest(t)
	defer cleanup()

	key1, _ := crypto.GenerateKey()
	key2, _ := crypto.GenerateKey()
	sm.CreateAccount(key1.PubKey().Address(), 0, big.NewInt(1000000))
	sm.CreateAccount(key2.PubKey().Address(), 0, big.NewInt(1000000))

	for i := 0; i < 5; i++ {
		tx := generateTestTx(t, key1, uint64(i+1), key2.PubKey().Address())
		pool.Add(tx)
	}

	if err := persister.Save(pool); err != nil {
		t.Fatalf("failed to save mempool: %v", err)
	}

	loaded, err := persister.Load()
	if err != nil {
		t.Fatalf("failed to load mempool: %v", err)
	}

	if len(loaded) != 5 {
		t.Fatalf("expected 5 transactions, got %d", len(loaded))
	}

	for i, tx := range loaded {
		if tx.Nonce < 1 || tx.Nonce > 5 {
			t.Errorf("transaction %d has unexpected nonce %d (expected 1-5)", i, tx.Nonce)
		}
	}

	nonces := make(map[uint64]bool)
	for _, tx := range loaded {
		nonces[tx.Nonce] = true
	}
	for i := 1; i <= 5; i++ {
		if !nonces[uint64(i)] {
			t.Errorf("missing nonce %d in loaded transactions", i)
		}
	}
}

func TestPersistenceSurvivesRestart(t *testing.T) {
	dir, _ := os.MkdirTemp("", "mempool-restart-test")
	defer os.RemoveAll(dir)

	db, err := state.NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sm, _ := state.NewStateManager(db)
	sm.Initialize(new(big.Int).SetUint64(1000000000))
	pool := ledger.NewTxPool(nil, sm)
	persister := ledger.NewMempoolPersister(dir)

	key1, _ := crypto.GenerateKey()
	key2, _ := crypto.GenerateKey()
	sm.CreateAccount(key1.PubKey().Address(), 0, big.NewInt(1000000))
	sm.CreateAccount(key2.PubKey().Address(), 0, big.NewInt(1000000))

	tx := generateTestTx(t, key1, 1, key2.PubKey().Address())
	pool.Add(tx)

	persister.Save(pool)
	db.Close()

	db2, _ := state.NewBadgerStore(dir)
	_, _ = state.NewStateManager(db2)
	persister2 := ledger.NewMempoolPersister(dir)

	loaded, err := persister2.Load()
	if err != nil {
		t.Fatalf("failed to load after restart: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(loaded))
	}

	db2.Close()
}

func TestCorruptedFileDetection(t *testing.T) {
	dir, _ := os.MkdirTemp("", "mempool-corrupt-test")
	defer os.RemoveAll(dir)

	persister := ledger.NewMempoolPersister(dir)

	corruptData := []byte("{invalid json data")
	os.WriteFile(dir+"/mempool.json", corruptData, 0644)

	_, err := persister.Load()
	if err == nil {
		t.Error("expected error for corrupted file")
	}
}

func TestEmptyMempoolSaveLoad(t *testing.T) {
	persister, pool, _, cleanup := setupMempoolTest(t)
	defer cleanup()

	if err := persister.Save(pool); err != nil {
		t.Fatalf("save should not fail for empty pool: %v", err)
	}

	loaded, err := persister.Load()
	if err != nil {
		t.Fatalf("load should not fail: %v", err)
	}

	if loaded != nil && len(loaded) != 0 {
		t.Errorf("expected empty or nil, got %d transactions", len(loaded))
	}
}

func TestLargeMempoolSaveLoad(t *testing.T) {
	persister, pool, sm, cleanup := setupMempoolTest(t)
	defer cleanup()

	key1, _ := crypto.GenerateKey()
	key2, _ := crypto.GenerateKey()
	sm.CreateAccount(key1.PubKey().Address(), 0, big.NewInt(100000000))
	sm.CreateAccount(key2.PubKey().Address(), 0, big.NewInt(100000000))

	numTxs := 100
	for i := 0; i < numTxs; i++ {
		tx := generateTestTx(t, key1, uint64(i+1), key2.PubKey().Address())
		pool.Add(tx)
	}

	if err := persister.Save(pool); err != nil {
		t.Fatalf("failed to save large mempool: %v", err)
	}

	loaded, err := persister.Load()
	if err != nil {
		t.Fatalf("failed to load large mempool: %v", err)
	}

	if len(loaded) != numTxs {
		t.Fatalf("expected %d transactions, got %d", numTxs, len(loaded))
	}
}

func TestSignatureVerificationOnLoadedTransactions(t *testing.T) {
	persister, pool, sm, cleanup := setupMempoolTest(t)
	defer cleanup()

	key1, _ := crypto.GenerateKey()
	key2, _ := crypto.GenerateKey()
	sm.CreateAccount(key1.PubKey().Address(), 0, big.NewInt(1000000))
	sm.CreateAccount(key2.PubKey().Address(), 0, big.NewInt(1000000))

	tx := generateTestTx(t, key1, 1, key2.PubKey().Address())
	pool.Add(tx)

	persister.Save(pool)

	loaded, _ := persister.Load()
	if len(loaded) != 1 {
		t.Fatalf("expected 1 transaction")
	}

	if !loaded[0].Verify() {
		t.Error("loaded transaction signature verification failed")
	}
}
