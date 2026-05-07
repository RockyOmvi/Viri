package consensus

import (
	"bytes"
	"crypto/sha256"
	"sync"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/logging"
)

type mockBlockProducer struct {
	mu          sync.Mutex
	blocks      map[uint64][]byte
	blockHashes map[uint64][]byte
	key         *crypto.PrivateKey
	address     []byte
	pubKey      []byte
	height      uint64
}

func newMockBlockProducer() *mockBlockProducer {
	key, _ := crypto.GenerateKey()
	return &mockBlockProducer{
		blocks:      make(map[uint64][]byte),
		blockHashes: make(map[uint64][]byte),
		key:         key,
		address:     key.PubKey().Address(),
		pubKey:      key.PubKey().Bytes(),
		height:      1,
	}
}

func (m *mockBlockProducer) CreateBlock(proposer []byte, height uint64) ([]byte, []byte, error) {
	data := []byte("block-" + string(proposer))
	hash := sha256.Sum256(data)

	m.mu.Lock()
	m.blocks[height] = data
	m.blockHashes[height] = hash[:]
	m.mu.Unlock()

	return data, hash[:], nil
}

func (m *mockBlockProducer) ValidateBlock(blockData []byte, blockHash []byte, height uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

func (m *mockBlockProducer) CommitBlock(blockHash []byte, height uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocks[height] = append(m.blocks[height], blockHash...)
	return nil
}

func (m *mockBlockProducer) GetBlockHash(height uint64) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.blockHashes[height], nil
}

func (m *mockBlockProducer) GetBlockData(height uint64) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.blocks[height], nil
}

func (m *mockBlockProducer) RotateKey() error {
	return nil
}

func (m *mockBlockProducer) Sign(data []byte) (*crypto.Signature, error) {
	return m.key.Sign(data)
}

func (m *mockBlockProducer) VerifySign(pubKey []byte, data []byte, sig *crypto.Signature) bool {
	return true
}

func (m *mockBlockProducer) GetValidatorAddress() []byte {
	return m.address
}

func (m *mockBlockProducer) GetValidatorPublicKey() []byte {
	return m.pubKey
}

func (m *mockBlockProducer) GetChainHeight() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.height
}

func TestHotStuffEngineStart(t *testing.T) {
	validators := []*Validator{
		{Address: []byte{0x01}, PublicKey: []byte("pk1"), Stake: 100, IsActive: true},
		{Address: []byte{0x02}, PublicKey: []byte("pk2"), Stake: 100, IsActive: true},
		{Address: []byte{0x03}, PublicKey: []byte("pk3"), Stake: 100, IsActive: true},
		{Address: []byte{0x04}, PublicKey: []byte("pk4"), Stake: 100, IsActive: true},
	}

	vs := NewValidatorSet(validators, 1)
	bp := newMockBlockProducer()
	sm := NewStakingModule(24*time.Hour, 0.01)
	log := logging.NewLogger("test", logging.ParseLogLevel("info"), "info")

	config := DefaultConsensusConfig()
	config.BlockTime = 100 * time.Millisecond
	config.ViewTimeout = 200 * time.Millisecond

	engine := NewHotStuffEngine(config, vs, bp, sm, log, nil)

	err := engine.Start(1)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if engine.Height() != 1 {
		t.Errorf("Expected height 1, got %d", engine.Height())
	}

	if engine.Phase() != PhasePrepare {
		t.Errorf("Expected PhasePrepare, got %v", engine.Phase())
	}

	engine.Stop()
}

func TestHotStuffEnginePropose(t *testing.T) {
	addr := []byte{0x01}
	validators := []*Validator{
		{Address: addr, PublicKey: []byte("pk1"), Stake: 300, IsActive: true},
		{Address: []byte{0x02}, PublicKey: []byte("pk2"), Stake: 100, IsActive: true},
	}

	vs := NewValidatorSet(validators, 1)
	bp := newMockBlockProducer()
	sm := NewStakingModule(24*time.Hour, 0.01)
	log := logging.NewLogger("test", logging.ParseLogLevel("info"), "info")

	config := DefaultConsensusConfig()
	config.BlockTime = 100 * time.Millisecond
	config.ViewTimeout = 200 * time.Millisecond

	engine := NewHotStuffEngine(config, vs, bp, sm, log, nil)
	engine.Start(1)

	if !engine.IsLeader() {
		t.Log("This node is not the leader, skipping propose test")
		engine.Stop()
		return
	}

	err := engine.Propose()
	if err != nil {
		t.Fatalf("Propose failed: %v", err)
	}

	if engine.Phase() != PhasePrepare {
		t.Errorf("Expected PhasePrepare after propose, got %v", engine.Phase())
	}

	engine.Stop()
}

func TestHotStuffEngineStateTransitions(t *testing.T) {
	validators := []*Validator{
		{Address: []byte{0x01}, PublicKey: []byte("pk1"), Stake: 100, IsActive: true},
		{Address: []byte{0x02}, PublicKey: []byte("pk2"), Stake: 100, IsActive: true},
		{Address: []byte{0x03}, PublicKey: []byte("pk3"), Stake: 100, IsActive: true},
	}

	vs := NewValidatorSet(validators, 1)
	bp := newMockBlockProducer()
	sm := NewStakingModule(24*time.Hour, 0.01)
	log := logging.NewLogger("test", logging.ParseLogLevel("info"), "info")

	config := DefaultConsensusConfig()
	config.BlockTime = 100 * time.Millisecond
	config.ViewTimeout = 200 * time.Millisecond

	engine := NewHotStuffEngine(config, vs, bp, sm, log, nil)
	engine.Start(1)

	heightKey := "1-0-PREPARE"
	engine.votes[heightKey] = make(map[Phase]map[string]bool)
	engine.votes[heightKey][PhasePrepare] = make(map[string]bool)

	for _, v := range validators {
		engine.votes[heightKey][PhasePrepare][string(v.Address)] = true
	}

	state := engine.GetState()
	if state == nil {
		t.Fatal("State is nil")
	}

	engine.Stop()
}

func TestConsensusConfigDefaults(t *testing.T) {
	config := DefaultConsensusConfig()

	if config.BlockTime != 3*time.Second {
		t.Errorf("Expected BlockTime 3s, got %v", config.BlockTime)
	}

	if config.ViewTimeout != 5*time.Second {
		t.Errorf("Expected ViewTimeout 5s, got %v", config.ViewTimeout)
	}

	if config.MinValidators != 4 {
		t.Errorf("Expected MinValidators 4, got %d", config.MinValidators)
	}

	if config.EpochLength != 1000 {
		t.Errorf("Expected EpochLength 1000, got %d", config.EpochLength)
	}
}

func TestPhaseString(t *testing.T) {
	tests := map[Phase]string{
		PhaseIdle:      "IDLE",
		PhasePrepare:   "PREPARE",
		PhasePreCommit: "PRECOMMIT",
		PhaseCommit:    "COMMIT",
		PhaseDecide:    "DECIDE",
	}

	for phase, expected := range tests {
		if phase.String() != expected {
			t.Errorf("Phase %v: expected %q, got %q", phase, expected, phase.String())
		}
	}
}

func TestQCEncodeDecode(t *testing.T) {
	key, _ := crypto.GenerateKey()
	sig, _ := key.Sign([]byte("data"))

	qc := &QC{
		Height:    42,
		View:      3,
		Phase:     PhaseCommit,
		BlockHash: []byte("blockhash123456789012345678901234"),
		Signatures: map[string]crypto.Signature{
			"ab": *sig,
		},
		ValidatorAddrs: []string{"ab"},
	}

	data, err := qc.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := DecodeQC(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded.Height != qc.Height {
		t.Errorf("Height mismatch: expected %d, got %d", qc.Height, decoded.Height)
	}

	if decoded.View != qc.View {
		t.Errorf("View mismatch: expected %d, got %d", qc.View, decoded.View)
	}

	if decoded.Phase != qc.Phase {
		t.Errorf("Phase mismatch: expected %v, got %v", qc.Phase, decoded.Phase)
	}

	if !bytes.Equal(decoded.BlockHash, qc.BlockHash) {
		t.Errorf("BlockHash mismatch")
	}
}
