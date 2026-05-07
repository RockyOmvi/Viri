package benchmarks

import (
	"math/big"
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
		tx, _ := ledger.NewTransactionFromKey(uint64(i), []byte{0x02}, 100, 100000, 1, nil, key)
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
		tx, _ := ledger.NewTransactionFromKey(uint64(i), []byte{0x02}, 100, 100000, 1, nil, key)
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

func TestBenchmarksEnabled(t *testing.T) {
	if testing.Short() {
		return
	}
}
