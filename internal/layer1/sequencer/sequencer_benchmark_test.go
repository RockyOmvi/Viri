package sequencer

import (
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/state"
)

func BenchmarkSequencerAddTransaction(b *testing.B) {
	genesis := ledger.TestGenesis()
	store := state.NewMemoryStore()
	chain, err := ledger.NewPersistentBlockchain(genesis, store)
	if err != nil {
		b.Fatalf("chain init failed: %v", err)
	}

	key, _ := crypto.GenerateKey()
	cfg := DefaultSequencerConfig()
	cfg.ProposerKey = key
	seq := NewSequencer(cfg, chain)
	_ = seq.Start()

	txKey, _ := crypto.GenerateKey()
	stateMgr, _ := state.NewStateManager(state.NewMemoryStore())
	_ = stateMgr.Initialize(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, _ := ledger.NewTransactionFromKey(uint64(i), []byte{0x02}, 1, 1000, 1, nil, genesis.ChainID, txKey)
		_ = seq.AddTransaction(tx)
	}
	seq.Stop()
}
