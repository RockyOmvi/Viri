package recovery

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

type RecoveryState struct {
	mu               sync.RWMutex
	chainDataPath    string
	backupPath       string
	checkpoints      []*Checkpoint
	forkResolver     *ForkResolver
	rollbackTarget   uint64
	recoveryLog      []*RecoveryEntry
}

type Checkpoint struct {
	BlockNumber uint64   `json:"block_number"`
	BlockHash   string   `json:"block_hash"`
	StateRoot   string   `json:"state_root"`
	Timestamp   uint64   `json:"timestamp"`
	Data        []byte   `json:"data"`
	Validators  [][]byte `json:"validators"`
	Signature   []byte   `json:"signature"`
}

type RecoveryEntry struct {
	Timestamp   uint64  `json:"timestamp"`
	Action      string  `json:"action"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
}

type ForkResolver struct {
	mu          sync.Mutex
	forkHeads   []*ForkHead
	threshold   int
	validatorVotes map[string]int
}

type ForkHead struct {
	Hash      string
	Height    uint64
	Parent    string
	Weight    uint64
	Timestamp uint64
	Supported int
}

type Snapshot struct {
	BlockNumber   uint64            `json:"block_number"`
	StateRoot     string            `json:"state_root"`
	Accounts      map[string]*AccountState `json:"accounts"`
	Validators    []*ValidatorState `json:"validators"`
	Parameters    map[string]interface{} `json:"parameters"`
	Timestamp     uint64            `json:"timestamp"`
}

type AccountState struct {
	Balance string `json:"balance"`
	Nonce   uint64 `json:"nonce"`
	Code    string `json:"code"`
	Storage map[string]string `json:"storage"`
}

type ValidatorState struct {
	Address string `json:"address"`
	Stake   uint64 `json:"stake"`
	Active  bool   `json:"active"`
}

func NewRecoveryState(chainPath, backupPath string) *RecoveryState {
	return &RecoveryState{
		chainDataPath: chainPath,
		backupPath:    backupPath,
		checkpoints:   make([]*Checkpoint, 0),
		forkResolver: &ForkResolver{
			forkHeads:      make([]*ForkHead, 0),
			threshold:      2,
			validatorVotes: make(map[string]int),
		},
		recoveryLog: make([]*RecoveryEntry, 0),
	}
}

func (rs *RecoveryState) CreateCheckpoint(blockNum uint64, blockHash, stateRoot string, data []byte) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	checkpoint := &Checkpoint{
		BlockNumber: blockNum,
		BlockHash:   blockHash,
		StateRoot:   stateRoot,
		Timestamp:   uint64(time.Now().Unix()),
		Data:        data,
	}

	rs.checkpoints = append(rs.checkpoints, checkpoint)

	sort.Slice(rs.checkpoints, func(i, j int) bool {
		return rs.checkpoints[i].BlockNumber > rs.checkpoints[j].BlockNumber
	})

	if len(rs.checkpoints) > 10 {
		rs.checkpoints = rs.checkpoints[:10]
	}

	rs.logRecovery("checkpoint", fmt.Sprintf("Created checkpoint at block %d", blockNum), "success")
	return nil
}

func (rs *RecoveryState) RollbackToBlock(targetBlock uint64) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.rollbackTarget = targetBlock

	rs.logRecovery("rollback", fmt.Sprintf("Initiating rollback to block %d", targetBlock), "started")

	checkpoint := rs.findCheckpointBefore(targetBlock)
	if checkpoint == nil {
		return fmt.Errorf("no checkpoint found before block %d", targetBlock)
	}

	if err := rs.restoreFromCheckpoint(checkpoint); err != nil {
		rs.logRecovery("rollback", fmt.Sprintf("Rollback failed: %v", err), "failed")
		return err
	}

	rs.logRecovery("rollback", fmt.Sprintf("Rollback to block %d completed", targetBlock), "success")
	return nil
}

func (rs *RecoveryState) findCheckpointBefore(block uint64) *Checkpoint {
	for _, cp := range rs.checkpoints {
		if cp.BlockNumber <= block {
			return cp
		}
	}
	return nil
}

func (rs *RecoveryState) restoreFromCheckpoint(cp *Checkpoint) error {
	if cp.Data == nil || len(cp.Data) == 0 {
		return fmt.Errorf("checkpoint data is empty")
	}

	if err := os.MkdirAll(rs.backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	backupFile := fmt.Sprintf("%s/checkpoint_%d.json", rs.backupPath, cp.BlockNumber)
	if err := os.WriteFile(backupFile, cp.Data, 0644); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	return nil
}

func (rs *RecoveryState) DetectFork(head1, head2 *ForkHead) (*ForkHead, error) {
	rs.forkResolver.mu.Lock()
	defer rs.forkResolver.mu.Unlock()

	rs.forkResolver.forkHeads = append(rs.forkResolver.forkHeads, head1, head2)

	sort.Slice(rs.forkResolver.forkHeads, func(i, j int) bool {
		return rs.forkResolver.forkHeads[i].Weight > rs.forkResolver.forkHeads[j].Weight
	})

	if len(rs.forkResolver.forkHeads) >= 2 {
		winner := rs.forkResolver.forkHeads[0]
		rs.logRecovery("fork_resolution", fmt.Sprintf("Selected fork: %s (weight: %d)", winner.Hash, winner.Weight), "success")
		return winner, nil
	}

	return nil, fmt.Errorf("insufficient fork heads")
}

func (rs *RecoveryState) RecordValidatorVote(validator, forkHash string) {
	rs.forkResolver.mu.Lock()
	defer rs.forkResolver.mu.Unlock()

	rs.forkResolver.validatorVotes[validator+"-"+forkHash]++
}

func (rs *RecoveryState) GetForkResolution() *ForkHead {
	rs.forkResolver.mu.Lock()
	defer rs.forkResolver.mu.Unlock()

	if len(rs.forkResolver.forkHeads) == 0 {
		return nil
	}

	return rs.forkResolver.forkHeads[0]
}

func (rs *RecoveryState) CreateSnapshot(blockNum uint64, stateRoot string) (*Snapshot, error) {
	return &Snapshot{
		BlockNumber: blockNum,
		StateRoot:   stateRoot,
		Accounts:    make(map[string]*AccountState),
		Validators:  make([]*ValidatorState, 0),
		Parameters:  make(map[string]interface{}),
		Timestamp:   uint64(time.Now().Unix()),
	}, nil
}

func (rs *RecoveryState) ExportSnapshot(snapshot *Snapshot, path string) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func (rs *RecoveryState) ImportSnapshot(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot: %w", err)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot: %w", err)
	}

	return &snapshot, nil
}

func (rs *RecoveryState) VerifyChainIntegrity(checkpoints []*Checkpoint) error {
	for i := 1; i < len(checkpoints); i++ {
		if checkpoints[i].BlockNumber >= checkpoints[i-1].BlockNumber {
			return fmt.Errorf("checkpoints not in order")
		}
	}

	return nil
}

func (rs *RecoveryState) EmergencyPause(reason string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.logRecovery("emergency_pause", reason, "activated")
	return nil
}

func (rs *RecoveryState) logRecovery(action, description, status string) {
	rs.recoveryLog = append(rs.recoveryLog, &RecoveryEntry{
		Timestamp:   uint64(time.Now().Unix()),
		Action:      action,
		Description: description,
		Status:      status,
	})
}

func (rs *RecoveryState) GetRecoveryLog() []*RecoveryEntry {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	log := make([]*RecoveryEntry, len(rs.recoveryLog))
	copy(log, rs.recoveryLog)
	return log
}

func (rs *RecoveryState) GetCheckpoints() []*Checkpoint {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	checkpoints := make([]*Checkpoint, len(rs.checkpoints))
	copy(checkpoints, rs.checkpoints)
	return checkpoints
}

type ChainSplitDetector struct {
	mu           sync.Mutex
	validatorHeads map[string]string
	splitThreshold int
}

func NewChainSplitDetector(threshold int) *ChainSplitDetector {
	return &ChainSplitDetector{
		validatorHeads: make(map[string]string),
		splitThreshold: threshold,
	}
}

func (csd *ChainSplitDetector) ReportHead(validator, headHash string) {
	csd.mu.Lock()
	defer csd.mu.Unlock()
	csd.validatorHeads[validator] = headHash
}

func (csd *ChainSplitDetector) DetectSplit() ([]string, bool) {
	csd.mu.Lock()
	defer csd.mu.Unlock()

	heads := make(map[string]int)
	for _, head := range csd.validatorHeads {
		heads[head]++
	}

	if len(heads) <= 1 {
		return nil, false
	}

	differentHeads := make([]string, 0)
	for head, count := range heads {
		if count >= csd.splitThreshold {
			differentHeads = append(differentHeads, head)
		}
	}

	return differentHeads, len(differentHeads) > 1
}

func ComputeStateHash(accounts map[string]*AccountState) string {
	h := sha256.New()

	keys := make([]string, 0, len(accounts))
	for k := range accounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, addr := range keys {
		acc := accounts[addr]
		h.Write([]byte(addr))
		h.Write([]byte(acc.Balance))
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}
