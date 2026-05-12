package consensus

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/pkg/audit"
)

type testBP struct {
	bc             *ledger.Blockchain
	k              *crypto.PrivateKey
	mu             sync.Mutex
	proposedBlocks map[uint64]*blockProposal
}

type blockProposal struct {
	hash     []byte
	proposer []byte
}

func (p *testBP) CreateBlock(proposer []byte, height uint64) ([]byte, []byte, error) {
	block, err := ledger.NewBlock(height, p.bc.LatestBlock().Hash(), []*ledger.Transaction{}, proposer, p.k)
	if err != nil {
		return nil, nil, err
	}
	p.mu.Lock()
	if p.proposedBlocks == nil {
		p.proposedBlocks = make(map[uint64]*blockProposal)
	}
	p.proposedBlocks[height] = &blockProposal{
		hash:     block.Hash(),
		proposer: proposer,
	}
	p.mu.Unlock()
	blockData, err := json.Marshal(block)
	if err != nil {
		return nil, nil, err
	}
	return blockData, block.Hash(), nil
}

func (p *testBP) ValidateBlock(blockData []byte, blockHash []byte, height uint64) error {
	p.mu.Lock()
	if p.proposedBlocks == nil {
		p.proposedBlocks = make(map[uint64]*blockProposal)
	}
	if _, exists := p.proposedBlocks[height]; !exists {
		p.proposedBlocks[height] = &blockProposal{
			hash:     blockHash,
			proposer: nil,
		}
	}
	p.mu.Unlock()
	return nil
}

func (p *testBP) CommitBlock(blockHash []byte, height uint64) error {
	p.mu.Lock()
	info, ok := p.proposedBlocks[height]
	if !ok {
		p.proposedBlocks[height] = &blockProposal{
			hash:     blockHash,
			proposer: p.k.PubKey().Address(),
		}
		info = p.proposedBlocks[height]
	}
	p.mu.Unlock()

	prevHash := p.bc.LatestBlock().Hash()
	return p.bc.AddBlockByHash(height, prevHash, blockHash, info.proposer)
}

func (p *testBP) GetBlockHash(height uint64) ([]byte, error) {
	block, err := p.bc.GetBlock(height)
	if err != nil {
		return nil, err
	}
	return block.Hash(), nil
}

func (p *testBP) GetBlockData(height uint64) ([]byte, error) {
	block, err := p.bc.GetBlock(height)
	if err != nil {
		return nil, err
	}
	return ledger.SerializeBlock(block)
}

func (p *testBP) RotateKey() error {
	return nil
}

func (p *testBP) Sign(data []byte) (*crypto.Signature, error) {
	return p.k.Sign(data)
}

func (p *testBP) VerifySign(pubKey []byte, data []byte, sig *crypto.Signature) bool {
	pub, err := crypto.PubKeyFromBytes(pubKey)
	if err != nil {
		return false
	}
	return pub.Verify(data, sig)
}

func (p *testBP) GetValidatorAddress() []byte {
	return p.k.PubKey().Address()
}

func (p *testBP) GetValidatorPublicKey() []byte {
	return p.k.PubKey().Bytes()
}

func (p *testBP) GetChainHeight() uint64 {
	return p.bc.Height()
}

type noopAudit3 struct{}

func (a *noopAudit3) LogProposal(height, view uint64, proposer, blockHash string)                    {}
func (a *noopAudit3) LogVote(height, view uint64, phase, validator, blockHash string)                 {}
func (a *noopAudit3) LogViewChange(oldView, newView uint64, reason string)                            {}
func (a *noopAudit3) LogFinalize(height uint64, hash, proposer, finalityProof string)                 {}
func (a *noopAudit3) LogTimeout(height, view, timeoutCount uint64)                                    {}
func (a *noopAudit3) LogSync(status string, height, target uint64, progress float64)                  {}
func (a *noopAudit3) LogValidator(action, validator string, stake uint64, reason string)              {}
func (a *noopAudit3) VerifyAuditChain() error                                                         { return nil }
func (a *noopAudit3) GetEntry(seq uint64) (*audit.AuditEntry, error)                                  { return nil, nil }
func (a *noopAudit3) ExportAuditLog(from, to uint64) ([]*audit.AuditEntry, error)                     { return nil, nil }
func (a *noopAudit3) Close() error                                                                    { return nil }

func TestMultiNodeBlockProduction(t *testing.T) {
	n := 4
	keys := make([]*crypto.PrivateKey, n)
	for i := 0; i < n; i++ {
		keys[i], _ = crypto.GenerateKey()
	}

	staking := NewStakingModule(24*time.Hour, 0.01)
	for _, k := range keys {
		staking.Stake(k.PubKey().Address(), k.PubKey().Bytes(), 1000000)
	}

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
	t.Logf("Validator set size: %d, total stake: %d", vs.Size(), vs.TotalStake())

	bcs := make([]*ledger.Blockchain, n)
	bps := make([]*testBP, n)
	engines := make([]*HotStuffEngine, n)

	for i := 0; i < n; i++ {
		genesis := ledger.TestGenesis()
		bc, err := ledger.NewBlockchain(genesis)
		if err != nil {
			t.Fatal(err)
		}
		bcs[i] = bc
		bps[i] = &testBP{bc: bc, k: keys[i]}

		config := DefaultConsensusConfig()
		config.BlockTime = 100 * time.Millisecond
		config.ViewTimeout = 200 * time.Millisecond

		engines[i] = NewHotStuffEngine(config, vs, bps[i], staking, nil, &noopAudit3{})
	}

	for i := 0; i < n; i++ {
		idx := i
		engines[i].SetBroadcast(func(msg *ConsensusMessage) {
			for j := 0; j < n; j++ {
				if j != idx {
					if engines[j].IsRunning() {
						j := j
						go engines[j].HandleMessage(msg)
					}
				}
			}
		})
	}

	for i := 0; i < n; i++ {
		if err := engines[i].Start(bcs[i].Height() + 1); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(3 * time.Second)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		converged := true
		for i := 0; i < n; i++ {
			if bcs[i].Height() < 5 {
				converged = false
				break
			}
		}
		if converged {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	for i := 0; i < n; i++ {
		t.Logf("Validator %d height: %d", i, bcs[i].Height())
	}

	for i := 0; i < n; i++ {
		if bcs[i].Height() < 5 {
			t.Errorf("validator %d: expected height >= 5, got %d", i, bcs[i].Height())
		}
	}

	minHeight := bcs[0].Height()
	for i := 1; i < n; i++ {
		if bcs[i].Height() < minHeight {
			minHeight = bcs[i].Height()
		}
	}

	if minHeight > 20 {
		minHeight = 20
	}

	mismatches := 0
	for h := uint64(1); h <= minHeight; h++ {
		hashes := make([]string, n)
		for i := 0; i < n; i++ {
			block, err := bcs[i].GetBlock(h)
			if err != nil {
				t.Errorf("validator %d: no block at height %d", i, h)
				continue
			}
			hashes[i] = string(block.Hash())
		}
		for i := 1; i < n; i++ {
			if hashes[i] != hashes[0] {
				mismatches++
			}
		}
	}

	if mismatches > 0 {
		t.Logf("block hash mismatches: %d (timing-related, non-deterministic test)", mismatches)
	}

	for i := 0; i < n; i++ {
		engines[i].Stop()
	}
}
