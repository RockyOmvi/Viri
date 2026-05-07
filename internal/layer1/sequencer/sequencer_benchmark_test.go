package sequencer

import (
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/state"
)

func BenchmarkSequencerAddTransaction(b *testing.B) {
	seq := NewSequencer(DefaultSequencerConfig(), newTestChain(&testing.T{}))
	_ = seq.Start()

	key, _ := crypto.GenerateKey()
	stateMgr, _ := state.NewStateManager(state.NewMemoryStore())
	_ = stateMgr.Initialize(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, _ := ledger.NewTransactionFromKey(uint64(i), []byte{0x02}, 1, 1000, 1, nil, key)
		_ = seq.AddTransaction(tx)
	}
	seq.Stop()
	_ = time.Now()
}
