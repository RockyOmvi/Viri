package consensus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type DoubleSignType uint8

const (
	DoubleSignProposal DoubleSignType = iota
	DoubleSignVote
	DoubleSignFork
)

type DoubleSignRecord struct {
	Validator     []byte
	Type          DoubleSignType
	Height        uint64
	View          uint64
	Evidence1     []byte
	Evidence2     []byte
	Timestamp     time.Time
	IsSlashed     bool
	SlashAmount   uint64
}

type DoubleSignDetector struct {
	mu            sync.RWMutex
	history       map[heightViewKey]*signRecord
	evidence      []*DoubleSignRecord
	maxHistoryLen int
}

type heightViewKey struct {
	height uint64
	view   uint64
}

type signRecord struct {
	proposer    []byte
	blockHash   []byte
	votes       map[Phase]map[string]*voteInfo
}

type voteInfo struct {
	blockHash []byte
	signature []byte
}

func NewDoubleSignDetector() *DoubleSignDetector {
	return &DoubleSignDetector{
		history:       make(map[heightViewKey]*signRecord),
		evidence:      make([]*DoubleSignRecord, 0),
		maxHistoryLen: 10000,
	}
}

func (d *DoubleSignDetector) CheckProposal(proposer []byte, height, view uint64, blockHash []byte) *DoubleSignRecord {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := heightViewKey{height, view}
	record, exists := d.history[key]
	if !exists {
		record = &signRecord{
			proposer:  proposer,
			blockHash: blockHash,
			votes:     make(map[Phase]map[string]*voteInfo),
		}
		d.history[key] = record
		d.trimHistory()
		return nil
	}

	if !bytes.Equal(record.proposer, proposer) {
		return nil
	}

	if !bytes.Equal(record.blockHash, blockHash) {
		rec := &DoubleSignRecord{
			Validator: proposer,
			Type:      DoubleSignProposal,
			Height:    height,
			View:      view,
			Evidence1: record.blockHash,
			Evidence2: blockHash,
			Timestamp: time.Now(),
		}
		d.evidence = append(d.evidence, rec)
		return rec
	}

	return nil
}

func (d *DoubleSignDetector) CheckVote(validator []byte, height, view uint64, phase Phase, blockHash []byte) *DoubleSignRecord {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := heightViewKey{height, view}
	record, exists := d.history[key]
	if !exists {
		record = &signRecord{
			votes: make(map[Phase]map[string]*voteInfo),
		}
		d.history[key] = record
		d.trimHistory()
	}

	if record.votes[phase] == nil {
		record.votes[phase] = make(map[string]*voteInfo)
	}

	valKey := string(validator)
	existing, exists := record.votes[phase][valKey]
	if !exists {
		record.votes[phase][valKey] = &voteInfo{
			blockHash: append([]byte(nil), blockHash...),
		}
		return nil
	}

	if !bytes.Equal(existing.blockHash, blockHash) {
		rec := &DoubleSignRecord{
			Validator: validator,
			Type:      DoubleSignVote,
			Height:    height,
			View:      view,
			Evidence1: existing.blockHash,
			Evidence2: blockHash,
			Timestamp: time.Now(),
		}
		d.evidence = append(d.evidence, rec)
		return rec
	}

	return nil
}

func (d *DoubleSignDetector) GetEvidence() []*DoubleSignRecord {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*DoubleSignRecord, len(d.evidence))
	copy(result, d.evidence)
	return result
}

func (d *DoubleSignDetector) MarkSlashed(validator []byte, height uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, e := range d.evidence {
		if bytes.Equal(e.Validator, validator) && e.Height == height {
			e.IsSlashed = true
		}
	}
}

func (d *DoubleSignDetector) trimHistory() {
	if len(d.history) <= d.maxHistoryLen {
		return
	}

	var keys []heightViewKey
	for k := range d.history {
		keys = append(keys, k)
	}

	for i := 0; i < len(keys)/2; i++ {
		delete(d.history, keys[i])
	}
}

type FinalityGadget struct {
	mu              sync.RWMutex
	checkpoints     []*Checkpoint
	lastFinalized   uint64
	finalityWindow  uint64
	requiredVotes   float64
}

type Checkpoint struct {
	Height     uint64
	BlockHash  []byte
	Validators []string
	Signatures map[string]bool
	IsFinal    bool
	CreatedAt  time.Time
}

func NewFinalityGadget(finalityWindow uint64, requiredVotes float64) *FinalityGadget {
	if requiredVotes == 0 {
		requiredVotes = 0.67
	}
	return &FinalityGadget{
		checkpoints:    make([]*Checkpoint, 0),
		finalityWindow: finalityWindow,
		requiredVotes:  requiredVotes,
	}
}

func (fg *FinalityGadget) CreateCheckpoint(height uint64, blockHash []byte, validators []string) *Checkpoint {
	fg.mu.Lock()
	defer fg.mu.Unlock()

	cp := &Checkpoint{
		Height:     height,
		BlockHash:  append([]byte(nil), blockHash...),
		Validators: validators,
		Signatures: make(map[string]bool),
		CreatedAt:  time.Now(),
	}

	fg.checkpoints = append(fg.checkpoints, cp)
	return cp
}

func (fg *FinalityGadget) AddVote(height uint64, validator string) bool {
	fg.mu.Lock()
	defer fg.mu.Unlock()

	for _, cp := range fg.checkpoints {
		if cp.Height == height && !cp.IsFinal {
			cp.Signatures[validator] = true
			if len(cp.Signatures) >= int(float64(len(cp.Validators))*fg.requiredVotes) {
				cp.IsFinal = true
				if height > fg.lastFinalized {
					fg.lastFinalized = height
				}
				return true
			}
		}
	}
	return false
}

func (fg *FinalityGadget) IsFinalized(height uint64) bool {
	fg.mu.RLock()
	defer fg.mu.RUnlock()
	return height <= fg.lastFinalized
}

func (fg *FinalityGadget) LastFinalized() uint64 {
	fg.mu.RLock()
	defer fg.mu.RUnlock()
	return fg.lastFinalized
}

func (fg *FinalityGadget) Cleanup(maxHeight uint64) {
	fg.mu.Lock()
	defer fg.mu.Unlock()

	var kept []*Checkpoint
	for _, cp := range fg.checkpoints {
		if cp.Height+fg.finalityWindow >= maxHeight {
			kept = append(kept, cp)
		}
	}
	fg.checkpoints = kept
}

func (fg *FinalityGadget) GetCheckpoints(from, to uint64) []*Checkpoint {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	var result []*Checkpoint
	for _, cp := range fg.checkpoints {
		if cp.Height >= from && cp.Height <= to {
			result = append(result, cp)
		}
	}
	return result
}

type ConsensusStateStore struct {
	mu   sync.RWMutex
	data *PersistedState
}

type PersistedState struct {
	Height         uint64
	View           uint64
	Phase          Phase
	LockedQC       *QC
	PreparedQC     *QC
	DecidedHash    []byte
	ValidatorAddrs []string
	LastSaved      time.Time
}

func NewConsensusStateStore() *ConsensusStateStore {
	return &ConsensusStateStore{
		data: &PersistedState{},
	}
}

func (s *ConsensusStateStore) Save(state *ConsensusState, validatorAddrs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Height = state.Height
	s.data.View = state.View
	s.data.Phase = state.Phase
	s.data.LockedQC = state.LockedQC
	s.data.PreparedQC = state.PreparedQC
	s.data.DecidedHash = append([]byte(nil), state.DecidedHash...)
	s.data.ValidatorAddrs = validatorAddrs
	s.data.LastSaved = time.Now()

	return nil
}

func (s *ConsensusStateStore) Load() (*PersistedState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.data.Height == 0 && s.data.Phase == PhaseIdle {
		return nil, fmt.Errorf("no saved state")
	}

	return s.data, nil
}

func (s *PersistedState) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

func (s *PersistedState) FromJSON(data []byte) error {
	return json.Unmarshal(data, s)
}

type LivenessTracker struct {
	mu              sync.RWMutex
	lastActivity    map[string]uint64
	currentHeight   uint64
	threshold       uint64
	offlineSince    map[string]uint64
}

func NewLivenessTracker(threshold uint64) *LivenessTracker {
	return &LivenessTracker{
		lastActivity: make(map[string]uint64),
		offlineSince: make(map[string]uint64),
		threshold:    threshold,
	}
}

func (lt *LivenessTracker) RecordActivity(validator []byte, height uint64) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	key := string(validator)
	lt.lastActivity[key] = height
	delete(lt.offlineSince, key)
}

func (lt *LivenessTracker) UpdateHeight(height uint64) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.currentHeight = height
}

func (lt *LivenessTracker) GetOfflineValidators() []string {
	lt.mu.RLock()
	defer lt.mu.RUnlock()

	var offline []string
	for key, lastHeight := range lt.lastActivity {
		if lt.currentHeight-lastHeight > lt.threshold {
			if _, tracked := lt.offlineSince[key]; !tracked {
				lt.offlineSince[key] = lt.currentHeight
			}
			offline = append(offline, key)
		}
	}
	return offline
}

func (lt *LivenessTracker) GetMissed(validator []byte) uint64 {
	lt.mu.RLock()
	defer lt.mu.RUnlock()

	key := string(validator)
	lastHeight, exists := lt.lastActivity[key]
	if !exists {
		return lt.currentHeight
	}
	return lt.currentHeight - lastHeight
}

func (lt *LivenessTracker) GetMissedString(validator string) uint64 {
	lt.mu.RLock()
	defer lt.mu.RUnlock()

	lastHeight, exists := lt.lastActivity[validator]
	if !exists {
		return lt.currentHeight
	}
	return lt.currentHeight - lastHeight
}

func (lt *LivenessTracker) Reset() {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	lt.lastActivity = make(map[string]uint64)
	lt.offlineSince = make(map[string]uint64)
}
