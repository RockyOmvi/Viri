package consensus

import (
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/pkg/audit"
)

type debugBlockProducer struct {
	blockchain *ledger.Blockchain
	key        *crypto.PrivateKey
}

func (p *debugBlockProducer) CreateBlock(proposer []byte, height uint64) ([]byte, []byte, error) {
	txs := []*ledger.Transaction{}
	prevHash := p.blockchain.LatestBlock().Hash()
	block, err := ledger.NewBlock(height, prevHash, txs, proposer, p.key)
	if err != nil {
		return nil, nil, err
	}
	return block.Hash(), proposer, nil
}

func (p *debugBlockProducer) ValidateBlock(blockData []byte, blockHash []byte, height uint64) error {
	return nil
}

func (p *debugBlockProducer) CommitBlock(blockHash []byte, height uint64) error {
	txs := []*ledger.Transaction{}
	tip := p.blockchain.LatestBlock()
	prevHash := tip.Hash()
	block, err := ledger.NewBlock(height, prevHash, txs, p.key.PubKey().Address(), p.key)
	if err != nil {
		return err
	}
	return p.blockchain.AddBlock(block)
}

func (p *debugBlockProducer) GetBlockHash(height uint64) ([]byte, error) {
	block, err := p.blockchain.GetBlock(height)
	if err != nil {
		return nil, err
	}
	return block.Hash(), nil
}

func (p *debugBlockProducer) GetBlockData(height uint64) ([]byte, error) {
	block, err := p.blockchain.GetBlock(height)
	if err != nil {
		return nil, err
	}
	return ledger.SerializeBlock(block)
}

func (p *debugBlockProducer) RotateKey() error {
	return nil
}

func (p *debugBlockProducer) Sign(data []byte) (*crypto.Signature, error) {
	return p.key.Sign(data)
}

func (p *debugBlockProducer) VerifySign(pubKey []byte, data []byte, sig *crypto.Signature) bool {
	pub, err := crypto.PubKeyFromBytes(pubKey)
	if err != nil {
		return false
	}
	return pub.Verify(data, sig)
}

func (p *debugBlockProducer) GetValidatorAddress() []byte {
	return p.key.PubKey().Address()
}

func (p *debugBlockProducer) GetValidatorPublicKey() []byte {
	return p.key.PubKey().Bytes()
}

func (p *debugBlockProducer) GetChainHeight() uint64 {
	return p.blockchain.Height()
}

func TestSingleNodeBlockProduction(t *testing.T) {
	key, _ := crypto.GenerateKey()
	staking := NewStakingModule(24*time.Hour, 0.01)
	staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)

	activeValidators := staking.GetActiveValidators()
	validators := make([]*Validator, 0, len(activeValidators))
	for _, sv := range activeValidators {
		validators = append(validators, &Validator{
			Address:   sv.Address,
			PublicKey: sv.PublicKey,
			Stake:     sv.Stake,
			IsActive:  true,
		})
	}

	vs := NewValidatorSet(validators, 1)
	genesis := ledger.TestGenesis()
	bc, err := ledger.NewBlockchain(genesis)
	if err != nil {
		t.Fatal(err)
	}
	bp := &debugBlockProducer{blockchain: bc, key: key}

	config := DefaultConsensusConfig()
	config.BlockTime = 100 * time.Millisecond
	config.ViewTimeout = 200 * time.Millisecond
	config.MinValidators = 1

	auditLog := &noopAuditLogger2{}
	engine := NewHotStuffEngine(config, vs, bp, staking, nil, auditLog)

	proposer, err := vs.GetProposer(bc.Height())
	if err != nil {
		t.Fatalf("GetProposer failed: %v", err)
	}
	t.Logf("Validator address: %x", key.PubKey().Address())
	t.Logf("Validator set size: %d", vs.Size())
	t.Logf("Blockchain height: %d", bc.Height())
	t.Logf("Proposer for height %d: %x", bc.Height(), proposer.Address)
	t.Logf("Is proposer: %v", string(bp.GetValidatorAddress()) == string(proposer.Address))

	engine.SetBroadcast(func(msg *ConsensusMessage) {
		t.Logf("Broadcast: type=%v height=%d", msg.Type, msg.Height)
		engine.HandleMessage(msg)
	})

	if err := engine.Start(bc.Height() + 1); err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)

	t.Logf("Blockchain height after 500ms: %d", bc.Height())
	
	time.Sleep(1500 * time.Millisecond)

	t.Logf("Final blockchain height: %d", bc.Height())
	engine.Stop()

	if bc.Height() == 0 {
		t.Fatal("expected blocks to be produced")
	}
}

type noopAuditLogger2 struct{}

func (a *noopAuditLogger2) LogProposal(height, view uint64, proposer, blockHash string)                     {}
func (a *noopAuditLogger2) LogVote(height, view uint64, phase, validator, blockHash string)                 {}
func (a *noopAuditLogger2) LogViewChange(oldView, newView uint64, reason string)                            {}
func (a *noopAuditLogger2) LogFinalize(height uint64, hash, proposer, finalityProof string)                 {}
func (a *noopAuditLogger2) LogTimeout(height, view, timeoutCount uint64)                                    {}
func (a *noopAuditLogger2) LogSync(status string, height, target uint64, progress float64)                  {}
func (a *noopAuditLogger2) LogValidator(action, validator string, stake uint64, reason string)              {}
func (a *noopAuditLogger2) VerifyAuditChain() error                                                         { return nil }
func (a *noopAuditLogger2) GetEntry(seq uint64) (*audit.AuditEntry, error)                                  { return nil, nil }
func (a *noopAuditLogger2) ExportAuditLog(from, to uint64) ([]*audit.AuditEntry, error)                     { return nil, nil }
func (a *noopAuditLogger2) Close() error                                                                    { return nil }
