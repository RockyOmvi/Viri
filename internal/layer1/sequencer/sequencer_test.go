package sequencer

import (
	"math/big"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/state"
)

func newTestChain(t *testing.T) *ledger.PersistentBlockchain {
	t.Helper()
	genesis := ledger.TestGenesis()
	store := state.NewMemoryStore()
	chain, err := ledger.NewPersistentBlockchain(genesis, store)
	if err != nil {
		t.Fatalf("chain init failed: %v", err)
	}
	return chain
}

func newTestTx(t *testing.T, nonce uint64) *ledger.Transaction {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("key gen failed: %v", err)
	}
	stateMgr, _ := state.NewStateManager(state.NewMemoryStore())
	_ = stateMgr.Initialize(big.NewInt(1000000))

	tx, err := ledger.NewTransactionFromKey(nonce, []byte{0x02}, 1, 1000, 1, []byte{0x01}, key)
	if err != nil {
		t.Fatalf("tx create failed: %v", err)
	}
	return tx
}

func TestSequencerStartStop(t *testing.T) {
	seq := NewSequencer(DefaultSequencerConfig(), newTestChain(t))
	if seq.IsRunning() {
		t.Fatalf("should not be running")
	}

	if err := seq.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if !seq.IsRunning() {
		t.Fatalf("should be running")
	}

	if err := seq.Start(); err == nil {
		t.Fatalf("expected already running error")
	}

	seq.Stop()
	if seq.IsRunning() {
		t.Fatalf("should be stopped")
	}
}

func TestSequencerAddTransaction(t *testing.T) {
	seq := NewSequencer(DefaultSequencerConfig(), newTestChain(t))
	if err := seq.AddTransaction(newTestTx(t, 1)); err == nil {
		t.Fatalf("expected error when not running")
	}

	_ = seq.Start()
	if err := seq.AddTransaction(newTestTx(t, 1)); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if seq.PendingCount() != 1 {
		t.Fatalf("expected 1 pending, got %d", seq.PendingCount())
	}
	seq.Stop()
}

func TestSequencerBatching(t *testing.T) {
	config := DefaultSequencerConfig()
	config.BatchSize = 2
	config.BatchTimeout = 10 * time.Millisecond
	seq := NewSequencer(config, newTestChain(t))
	_ = seq.Start()

	_ = seq.AddTransaction(newTestTx(t, 1))
	_ = seq.AddTransaction(newTestTx(t, 2))

	seq.mu.Lock()
	seq.createBlock()
	seq.mu.Unlock()

	if seq.PendingCount() != 0 {
		t.Fatalf("expected pending to flush, got %d", seq.PendingCount())
	}
	seq.Stop()
}

func TestSequencerFiltersOversize(t *testing.T) {
	config := DefaultSequencerConfig()
	config.BatchSize = 2
	config.BatchTimeout = 10 * time.Millisecond
	config.MaxBlockSize = 300
	config.MaxGasPerBlock = 1500

	seq := NewSequencer(config, newTestChain(t))
	_ = seq.Start()

	_ = seq.AddTransaction(newTestTx(t, 1))
	_ = seq.AddTransaction(newTestTx(t, 2))

	seq.mu.Lock()
	seq.createBlock()
	seq.mu.Unlock()

	if seq.PendingCount() != 1 {
		t.Fatalf("expected 1 pending after filtering, got %d", seq.PendingCount())
	}
	seq.Stop()
}
