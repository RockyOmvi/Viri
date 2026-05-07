package consensus

import (
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

type noopInvariantBP struct {
	bc *ledger.Blockchain
	k  *crypto.PrivateKey
}

func (p *noopInvariantBP) CreateBlock(proposer []byte, height uint64) ([]byte, []byte, error) {
	block, err := ledger.NewBlock(height, nil, nil, proposer, p.k)
	if err != nil {
		return nil, nil, err
	}
	return block.Hash(), proposer, nil
}
func (p *noopInvariantBP) ValidateBlock(blockData []byte, blockHash []byte, height uint64) error { return nil }
func (p *noopInvariantBP) CommitBlock(blockHash []byte, height uint64) error                     { return nil }
func (p *noopInvariantBP) GetBlockHash(height uint64) ([]byte, error)                            { return nil, nil }
func (p *noopInvariantBP) GetBlockData(height uint64) ([]byte, error)                            { return nil, nil }
func (p *noopInvariantBP) RotateKey() error                                                      { return nil }
func (p *noopInvariantBP) Sign(data []byte) (*crypto.Signature, error)                           { return p.k.Sign(data) }
func (p *noopInvariantBP) VerifySign(pubKey []byte, data []byte, sig *crypto.Signature) bool     { return true }
func (p *noopInvariantBP) GetValidatorAddress() []byte                                           { return p.k.PubKey().Address() }
func (p *noopInvariantBP) GetValidatorPublicKey() []byte                                         { return p.k.PubKey().Bytes() }
func (p *noopInvariantBP) GetChainHeight() uint64                                                { return p.bc.Height() }

func TestVerifyInvariantsPass(t *testing.T) {
	key, _ := crypto.GenerateKey()
	genesis := ledger.TestGenesis()
	bc, _ := ledger.NewBlockchain(genesis)
	bp := &noopInvariantBP{bc: bc, k: key}

	staking := NewStakingModule(24*time.Hour, 0.01)
	staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)

	active := staking.GetActiveValidators()
	vals := make([]*Validator, 0, len(active))
	for _, sv := range active {
		vals = append(vals, &Validator{
			Address:   sv.Address,
			PublicKey: sv.PublicKey,
			Stake:     sv.Stake,
			IsActive:  true,
		})
	}
	vs := NewValidatorSet(vals, 1)

	config := DefaultConsensusConfig()
	config.ProtocolVersion = 1
	engine := NewHotStuffEngine(config, vs, bp, staking, nil, nil)

	if err := engine.Start(1); err != nil {
		t.Fatal(err)
	}
	defer engine.Stop()

	viols := engine.verifyInvariants()
	if len(viols) > 0 {
		for _, v := range viols {
			t.Errorf("invariant violation: [%s] %s", v.check, v.detail)
		}
	}
}

func TestVerifyInvariantsDetectCorruption(t *testing.T) {
	key, _ := crypto.GenerateKey()
	genesis := ledger.TestGenesis()
	bc, _ := ledger.NewBlockchain(genesis)
	bp := &noopInvariantBP{bc: bc, k: key}

	staking := NewStakingModule(24*time.Hour, 0.01)
	staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)

	active := staking.GetActiveValidators()
	vals := make([]*Validator, 0, len(active))
	for _, sv := range active {
		vals = append(vals, &Validator{
			Address:   sv.Address,
			PublicKey: sv.PublicKey,
			Stake:     sv.Stake,
			IsActive:  true,
		})
	}
	vs := NewValidatorSet(vals, 1)

	config := DefaultConsensusConfig()
	engine := NewHotStuffEngine(config, vs, bp, staking, nil, nil)

	if err := engine.Start(1); err != nil {
		t.Fatal(err)
	}
	defer engine.Stop()

	engine.mu.Lock()
	engine.state.Height = 0
	viols := engine.verifyInvariants()
	engine.mu.Unlock()

	if len(viols) == 0 {
		t.Fatal("expected invariant violations for height=0, got none")
	}

	found := false
	for _, v := range viols {
		if v.check == "height_nonzero" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected height_nonzero violation, got: %v", viols)
	}
}

func TestVerifyInvariantsDetectPhaseCorruption(t *testing.T) {
	key, _ := crypto.GenerateKey()
	genesis := ledger.TestGenesis()
	bc, _ := ledger.NewBlockchain(genesis)
	bp := &noopInvariantBP{bc: bc, k: key}

	staking := NewStakingModule(24*time.Hour, 0.01)
	staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)

	active := staking.GetActiveValidators()
	vals := make([]*Validator, 0, len(active))
	for _, sv := range active {
		vals = append(vals, &Validator{
			Address:   sv.Address,
			PublicKey: sv.PublicKey,
			Stake:     sv.Stake,
			IsActive:  true,
		})
	}
	vs := NewValidatorSet(vals, 1)

	config := DefaultConsensusConfig()
	engine := NewHotStuffEngine(config, vs, bp, staking, nil, nil)

	if err := engine.Start(1); err != nil {
		t.Fatal(err)
	}
	defer engine.Stop()

	engine.mu.Lock()
	engine.state.Phase = Phase(99)
	viols := engine.verifyInvariants()
	engine.mu.Unlock()

	found := false
	for _, v := range viols {
		if v.check == "phase_valid" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected phase_valid violation, got: %v", viols)
	}
}

func TestVerifyInvariantsDetectStaleTimeouts(t *testing.T) {
	key, _ := crypto.GenerateKey()
	genesis := ledger.TestGenesis()
	bc, _ := ledger.NewBlockchain(genesis)
	bp := &noopInvariantBP{bc: bc, k: key}

	staking := NewStakingModule(24*time.Hour, 0.01)
	staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)

	active := staking.GetActiveValidators()
	vals := make([]*Validator, 0, len(active))
	for _, sv := range active {
		vals = append(vals, &Validator{
			Address:   sv.Address,
			PublicKey: sv.PublicKey,
			Stake:     sv.Stake,
			IsActive:  true,
		})
	}
	vs := NewValidatorSet(vals, 1)

	config := DefaultConsensusConfig()
	engine := NewHotStuffEngine(config, vs, bp, staking, nil, nil)

	if err := engine.Start(1); err != nil {
		t.Fatal(err)
	}
	defer engine.Stop()

	engine.mu.Lock()
	engine.state.View = 5
	engine.curView.Store(5)
	engine.timeouts[0] = map[string]bool{"stale": true}
	engine.timeouts[2] = map[string]bool{"stale": true}
	viols := engine.verifyInvariants()
	engine.mu.Unlock()

	found := false
	for _, v := range viols {
		if v.check == "stale_timeouts" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected stale_timeouts violation, got: %v", viols)
	}
}
