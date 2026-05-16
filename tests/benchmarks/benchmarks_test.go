package benchmarks

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/config"
	"github.com/viri-chain/viri/internal/layer1/consensus"
	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/p2p"
	"github.com/viri-chain/viri/internal/layer1/state"
)

func BenchmarkBlockchainAddBlock(b *testing.B) {
	db := state.NewMemoryStore()
	genesis := ledger.DefaultGenesis()
	blockchain, _ := ledger.NewPersistentBlockchain(genesis, db)

	key, _ := crypto.GenerateKey()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, _ := ledger.NewTransactionFromKey(uint64(i), []byte{0x02}, 100, 100000, 1, nil, uint64(1), key)
		blockchain.TxPool().Add(tx)

		block, _ := ledger.NewBlock(uint64(i+1), blockchain.TipHash(), blockchain.TxPool().GetPending(), key.PubKey().Address(), key)
		blockchain.AddBlock(block)
	}
}

func BenchmarkTransactionPool(b *testing.B) {
	pool := ledger.NewTxPool(nil, nil)
	key, _ := crypto.GenerateKey()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, _ := ledger.NewTransactionFromKey(uint64(i), []byte{0x02}, 100, 100000, 1, nil, uint64(1), key)
		pool.Add(tx)
	}
}

func BenchmarkCryptoSign(b *testing.B) {
	key, _ := crypto.GenerateKey()
	data := []byte("test transaction data for signing benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key.Sign(data)
	}
}

func BenchmarkCryptoVerify(b *testing.B) {
	key, _ := crypto.GenerateKey()
	data := []byte("test transaction data for verification benchmark")
	sig, _ := key.Sign(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key.PubKey().Verify(data, sig)
	}
}

func BenchmarkStateAccountCreation(b *testing.B) {
	stateMgr, _ := state.NewStateManager(state.NewMemoryStore())
	stateMgr.Initialize(big.NewInt(1000000))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		addr := []byte{byte(i % 256), byte((i / 256) % 256)}
		stateMgr.CreateAccount(addr, state.AccountTypeNormal, big.NewInt(1000))
	}
}

func BenchmarkConsensusEngine(b *testing.B) {
	staking := consensus.NewStakingModule(24*time.Hour, 0.01)

	for i := 0; i < 4; i++ {
		key, _ := crypto.GenerateKey()
		staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)
	}

	activeValidators := staking.GetActiveValidators()
	validators := make([]*consensus.Validator, 0, len(activeValidators))
	for _, sv := range activeValidators {
		validators = append(validators, &consensus.Validator{
			Address:  sv.Address,
			PublicKey: sv.PublicKey,
			Stake:    sv.Stake,
			IsActive: true,
		})
	}

	vs := consensus.NewValidatorSet(validators, 1)
	config := consensus.DefaultConsensusConfig()

	engine := consensus.NewHotStuffEngine(config, vs, nil, staking, nil, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.AddReward(1000000)
	}
}

func BenchmarkP2PMessageEncode(b *testing.B) {
	msg := p2p.NewMessage(p2p.MsgBlock, []byte("test block data for encoding benchmark"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg.Encode()
	}
}

func BenchmarkConfigValidation(b *testing.B) {
	cfg := config.DefaultConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.Validate()
	}
}

func BenchmarkMerkleTree(b *testing.B) {
	leaves := make([][]byte, 1000)
	for i := range leaves {
		leaves[i] = []byte{byte(i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		crypto.NewMerkleTree(leaves)
	}
}

// BenchmarkConcurrentTransactionSubmission measures throughput under concurrent load.
func BenchmarkConcurrentTransactionSubmission(b *testing.B) {
	pool := ledger.NewTxPool(&ledger.TxPoolConfig{
		MaxTransactions: 100_000,
		MaxGas:          1_000_000_000,
		MinGasPrice:     1,
		MaxAccountTxs:   1000,
	}, nil)

	keys := make([]*crypto.PrivateKey, 32)
	for i := range keys {
		keys[i], _ = crypto.GenerateKey()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		keyIdx := 0
		nonce := uint64(0)
		for pb.Next() {
			key := keys[keyIdx%len(keys)]
			keyIdx++
			tx, err := ledger.NewTransactionFromKey(nonce, []byte{0x02}, 100, 100000, 1, nil, uint64(1), key)
			if err != nil {
				b.Fatal(err)
			}
			pool.Add(tx)
			nonce++
		}
	})
}

// BenchmarkMempoolFillEvict measures mempool behavior when full (eviction pressure).
func BenchmarkMempoolFillEvict(b *testing.B) {
	config := &ledger.TxPoolConfig{
		MaxTransactions: 1000,
		MaxGas:          1_000_000_000,
		MinGasPrice:     1,
		MaxAccountTxs:   100,
	}
	pool := ledger.NewTxPool(config, nil)
	key, _ := crypto.GenerateKey()

	// Fill pool to capacity
	for i := 0; i < config.MaxTransactions; i++ {
		tx, _ := ledger.NewTransactionFromKey(uint64(i), []byte{0x02}, 100, 100000, 1, nil, uint64(1), key)
		pool.Add(tx)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, _ := ledger.NewTransactionFromKey(uint64(config.MaxTransactions+i), []byte{0x02}, 100, 100000, 99999, nil, uint64(1), key)
		pool.Add(tx)
	}
}

// BenchmarkBlockProductionConcurrent measures block production throughput
// with concurrent transaction submission.
func BenchmarkBlockProductionConcurrent(b *testing.B) {
	db := state.NewMemoryStore()
	genesis := ledger.DefaultGenesis()
	blockchain, _ := ledger.NewPersistentBlockchain(genesis, db)
	key, _ := crypto.GenerateKey()

	// Pre-generate keys for concurrent submitters
	submitterCount := 16
	keys := make([]*crypto.PrivateKey, submitterCount)
	for i := range keys {
		keys[i], _ = crypto.GenerateKey()
	}

	var mu sync.Mutex
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for j := 0; j < submitterCount; j++ {
			wg.Add(1)
			go func(idx int, nonceOffset int) {
				defer wg.Done()
				k := keys[idx]
				tx, _ := ledger.NewTransactionFromKey(uint64(nonceOffset+idx), []byte{0x02}, 100, 100000, 1, nil, uint64(1), k)
				blockchain.TxPool().Add(tx)
			}(j, i*submitterCount)
		}
		wg.Wait()

		mu.Lock()
		block, _ := ledger.NewBlock(uint64(i+1), blockchain.TipHash(), blockchain.TxPool().GetPending(), key.PubKey().Address(), key)
		blockchain.AddBlock(block)
		mu.Unlock()
	}
}

func TestBenchmarksEnabled(t *testing.T) {
	if testing.Short() {
		return
	}
}
