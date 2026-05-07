package consensus

import (
	"bytes"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

func TestDoubleSignDetectorProposal(t *testing.T) {
	detector := NewDoubleSignDetector()

	proposer := []byte("validator1")

	_ = detector.CheckProposal(proposer, 1, 0, []byte("hash1"))

	evidence := detector.CheckProposal(proposer, 1, 0, []byte("hash2"))
	if evidence == nil {
		t.Fatal("Expected double sign evidence for conflicting proposals")
	}

	if evidence.Type != DoubleSignProposal {
		t.Errorf("Expected DoubleSignProposal, got %d", evidence.Type)
	}

	if !bytes.Equal(evidence.Evidence1, []byte("hash1")) {
		t.Errorf("Evidence1 mismatch")
	}

	if !bytes.Equal(evidence.Evidence2, []byte("hash2")) {
		t.Errorf("Evidence2 mismatch")
	}
}

func TestDoubleSignDetectorVote(t *testing.T) {
	detector := NewDoubleSignDetector()

	validator := []byte("validator1")

	_ = detector.CheckVote(validator, 1, 0, PhasePrepare, []byte("hash1"))

	evidence := detector.CheckVote(validator, 1, 0, PhasePrepare, []byte("hash2"))
	if evidence == nil {
		t.Fatal("Expected double sign evidence for conflicting votes")
	}

	if evidence.Type != DoubleSignVote {
		t.Errorf("Expected DoubleSignVote, got %d", evidence.Type)
	}
}

func TestDoubleSignDetectorNoConflict(t *testing.T) {
	detector := NewDoubleSignDetector()

	validator := []byte("validator1")

	_ = detector.CheckVote(validator, 1, 0, PhasePrepare, []byte("hash1"))

	evidence := detector.CheckVote(validator, 1, 0, PhasePrepare, []byte("hash1"))
	if evidence != nil {
		t.Error("Expected no double sign for same vote")
	}
}

func TestDoubleSignDetectorMarkSlashed(t *testing.T) {
	detector := NewDoubleSignDetector()

	proposer := []byte("validator1")

	_ = detector.CheckProposal(proposer, 1, 0, []byte("hash1"))
	evidence := detector.CheckProposal(proposer, 1, 0, []byte("hash2"))

	if evidence.IsSlashed {
		t.Error("Evidence should not be slashed initially")
	}

	detector.MarkSlashed(proposer, 1)

	evidences := detector.GetEvidence()
	if len(evidences) == 0 {
		t.Fatal("Expected evidence")
	}

	if !evidences[0].IsSlashed {
		t.Error("Evidence should be marked slashed")
	}
}

func TestDoubleSignDetectorTrimHistory(t *testing.T) {
	detector := NewDoubleSignDetector()
	detector.maxHistoryLen = 5

	for i := uint64(0); i < 10; i++ {
		_ = detector.CheckProposal([]byte("v1"), i, 0, []byte("hash"))
	}

	if len(detector.history) > 5 {
		t.Errorf("History should be trimmed, got %d entries", len(detector.history))
	}
}

func TestFinalityGadgetCheckpoint(t *testing.T) {
	fg := NewFinalityGadget(10, 0.67)

	_ = fg.CreateCheckpoint(1, []byte("hash1"), []string{"v1", "v2", "v3"})

	cps := fg.GetCheckpoints(1, 1)
	if len(cps) != 1 {
		t.Errorf("Expected 1 checkpoint, got %d", len(cps))
	}

	if !fg.IsFinalized(0) {
		t.Error("Height 0 should be finalized (genesis)")
	}
}

func TestFinalityGadgetVote(t *testing.T) {
	fg := NewFinalityGadget(10, 0.67)

	validators := []string{"v1", "v2", "v3"}
	_ = fg.CreateCheckpoint(1, []byte("hash1"), validators)

	for _, v := range validators {
		fg.AddVote(1, v)
	}

	if !fg.IsFinalized(1) {
		t.Error("Height 1 should be finalized after supermajority")
	}

	if fg.LastFinalized() != 1 {
		t.Errorf("Expected last finalized 1, got %d", fg.LastFinalized())
	}
}

func TestFinalityGadgetNotEnoughVotes(t *testing.T) {
	fg := NewFinalityGadget(10, 0.67)

	validators := []string{"v1", "v2", "v3"}
	_ = fg.CreateCheckpoint(1, []byte("hash1"), validators)

	fg.AddVote(1, "v1")

	if fg.IsFinalized(1) {
		t.Error("Height 1 should not be finalized with only 1/3 votes")
	}
}

func TestFinalityGadgetCleanup(t *testing.T) {
	fg := NewFinalityGadget(5, 0.67)

	for i := uint64(1); i <= 20; i++ {
		_ = fg.CreateCheckpoint(i, []byte("hash"), []string{"v1"})
	}

	fg.Cleanup(20)

	cps := fg.GetCheckpoints(1, 20)
	if len(cps) > 6 {
		t.Errorf("Expected ~6 checkpoints after cleanup, got %d", len(cps))
	}
}

func TestLivenessTrackerRecord(t *testing.T) {
	lt := NewLivenessTracker(10)

	lt.UpdateHeight(10)
	lt.RecordActivity([]byte("v1"), 5)

	missed := lt.GetMissed([]byte("v1"))
	if missed != 5 {
		t.Errorf("Expected 5 missed, got %d", missed)
	}
}

func TestLivenessTrackerOffline(t *testing.T) {
	lt := NewLivenessTracker(5)

	lt.UpdateHeight(20)
	lt.RecordActivity([]byte("v1"), 10)
	lt.RecordActivity([]byte("v2"), 19)

	offline := lt.GetOfflineValidators()
	if len(offline) != 1 {
		t.Errorf("Expected 1 offline validator, got %d", len(offline))
	}

	if offline[0] != "v1" {
		t.Errorf("Expected v1 offline, got %s", offline[0])
	}
}

func TestLivenessTrackerActive(t *testing.T) {
	lt := NewLivenessTracker(5)

	lt.UpdateHeight(20)
	lt.RecordActivity([]byte("v1"), 18)
	lt.RecordActivity([]byte("v2"), 19)

	offline := lt.GetOfflineValidators()
	if len(offline) != 0 {
		t.Errorf("Expected 0 offline validators, got %d", len(offline))
	}
}

func TestLivenessTrackerNeverSeen(t *testing.T) {
	lt := NewLivenessTracker(5)

	lt.UpdateHeight(10)

	missed := lt.GetMissed([]byte("v1"))
	if missed != 10 {
		t.Errorf("Expected 10 missed for never-seen validator, got %d", missed)
	}
}

func TestLivenessTrackerReset(t *testing.T) {
	lt := NewLivenessTracker(5)

	lt.UpdateHeight(20)
	lt.RecordActivity([]byte("v1"), 10)
	lt.RecordActivity([]byte("v2"), 15)

	lt.Reset()

	offline := lt.GetOfflineValidators()
	if len(offline) != 0 {
		t.Errorf("Expected 0 offline after reset, got %d", len(offline))
	}
}

func TestConsensusStateStoreSaveLoad(t *testing.T) {
	store := NewConsensusStateStore()

	state := &ConsensusState{
		Height: 42,
		View:   3,
		Phase:  PhaseCommit,
	}

	_ = store.Save(state, []string{"v1", "v2"})

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Height != 42 {
		t.Errorf("Expected height 42, got %d", loaded.Height)
	}

	if loaded.View != 3 {
		t.Errorf("Expected view 3, got %d", loaded.View)
	}

	if loaded.Phase != PhaseCommit {
		t.Errorf("Expected PhaseCommit, got %v", loaded.Phase)
	}
}

func TestConsensusStateStoreEmptyLoad(t *testing.T) {
	store := NewConsensusStateStore()

	_, err := store.Load()
	if err == nil {
		t.Error("Expected error for empty state")
	}
}

func TestPersistedStateJSON(t *testing.T) {
	state := &PersistedState{
		Height:    100,
		View:      2,
		Phase:     PhasePrepare,
		ValidatorAddrs: []string{"v1", "v2"},
	}

	data, err := state.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	restored := &PersistedState{}
	err = restored.FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if restored.Height != 100 {
		t.Errorf("Expected height 100, got %d", restored.Height)
	}
}

func TestEpochRotation(t *testing.T) {
	sm := NewStakingModule(24*time.Hour, 0.01)
	sm.Stake([]byte{0x01}, []byte("pk1"), 100)
	sm.Stake([]byte{0x02}, []byte("pk2"), 100)
	sm.Stake([]byte{0x03}, []byte("pk3"), 100)
	sm.Stake([]byte{0x04}, []byte("pk4"), 100)

	activeValidators := sm.GetActiveValidators()
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
	bp := newMockBlockProducer()

	config := DefaultConsensusConfig()
	config.EpochLength = 5
	config.BlockTime = 10 * time.Millisecond
	config.ViewTimeout = 20 * time.Millisecond

	engine := NewHotStuffEngine(config, vs, bp, sm, nil, nil)

	if engine.epochStartHeight != 1 {
		t.Errorf("Expected epoch start 1, got %d", engine.epochStartHeight)
	}

	originalEpoch := vs.Epoch()

	engine.rotateEpoch(5)

	if engine.validatorSet.Epoch() <= originalEpoch {
		t.Errorf("Expected new epoch > %d, got %d", originalEpoch, engine.validatorSet.Epoch())
	}

	if engine.epochStartHeight != 6 {
		t.Errorf("Expected epoch start 6, got %d", engine.epochStartHeight)
	}
}

func TestKeyRotationInterface(t *testing.T) {
	keys := make([]*crypto.PrivateKey, 1)
	keys[0], _ = crypto.GenerateKey()

	staking := NewStakingModule(24*time.Hour, 0.01)
	staking.Stake(keys[0].PubKey().Address(), keys[0].PubKey().Bytes(), 1000000)

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

	genesis := ledger.TestGenesis()
	bc, err := ledger.NewBlockchain(genesis)
	if err != nil {
		t.Fatal(err)
	}
	bp := &testBP{bc: bc, k: keys[0]}

	config := DefaultConsensusConfig()
	engine := NewHotStuffEngine(config, vs, bp, staking, nil, &noopAudit3{})

	if err := engine.blockProducer.RotateKey(); err != nil {
		t.Fatalf("RotateKey failed: %v", err)
	}
}

func TestRewardDistribution(t *testing.T) {
	validators := []*Validator{
		{Address: []byte{0x01}, PublicKey: []byte("pk1"), Stake: 100, IsActive: true},
		{Address: []byte{0x02}, PublicKey: []byte("pk2"), Stake: 100, IsActive: true},
	}

	vs := NewValidatorSet(validators, 1)
	bp := newMockBlockProducer()
	sm := NewStakingModule(24*time.Hour, 0.01)

	config := DefaultConsensusConfig()
	config.BlockTime = 10 * time.Millisecond
	config.ViewTimeout = 20 * time.Millisecond

	engine := NewHotStuffEngine(config, vs, bp, sm, nil, nil)

	engine.AddReward(1000)
	engine.distributeRewards(1)

	if engine.rewardPool != 0 {
		t.Errorf("Expected reward pool 0 after distribution, got %d", engine.rewardPool)
	}
}

func TestValidatorRegistration(t *testing.T) {
	validators := []*Validator{
		{Address: []byte{0x01}, PublicKey: []byte("pk1"), Stake: 100, IsActive: true},
	}

	vs := NewValidatorSet(validators, 1)
	bp := newMockBlockProducer()
	sm := NewStakingModule(24*time.Hour, 0.01)

	config := DefaultConsensusConfig()
	config.BlockTime = 10 * time.Millisecond
	config.ViewTimeout = 20 * time.Millisecond

	engine := NewHotStuffEngine(config, vs, bp, sm, nil, nil)

	err := engine.RegisterValidator([]byte{0x02}, []byte("pk2"), 200)
	if err != nil {
		t.Fatalf("RegisterValidator failed: %v", err)
	}

	record, exists := sm.GetValidator([]byte{0x02})
	if !exists {
		t.Fatal("Validator not found after registration")
	}

	if record.Stake != 200 {
		t.Errorf("Expected stake 200, got %d", record.Stake)
	}
}

func TestExportState(t *testing.T) {
	validators := []*Validator{
		{Address: []byte{0x01}, PublicKey: []byte("pk1"), Stake: 100, IsActive: true},
	}

	vs := NewValidatorSet(validators, 1)
	bp := newMockBlockProducer()
	sm := NewStakingModule(24*time.Hour, 0.01)

	config := DefaultConsensusConfig()
	config.BlockTime = 10 * time.Millisecond
	config.ViewTimeout = 20 * time.Millisecond

	engine := NewHotStuffEngine(config, vs, bp, sm, nil, nil)
	engine.Start(1)

	data, err := engine.ExportState()
	if err != nil {
		t.Fatalf("ExportState failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty state export")
	}

	engine.Stop()
}
