package recovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewRecoveryState(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")
	if rs == nil {
		t.Fatal("expected non-nil RecoveryState")
	}
	if rs.chainDataPath != "/tmp/chain" {
		t.Fatalf("chain path mismatch")
	}
	if rs.backupPath != "/tmp/backup" {
		t.Fatalf("backup path mismatch")
	}
}

func TestCreateCheckpoint(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")
	err := rs.CreateCheckpoint(100, "hash100", "root100", []byte("data100"))
	if err != nil {
		t.Fatalf("CreateCheckpoint failed: %v", err)
	}

	checkpoints := rs.GetCheckpoints()
	if len(checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(checkpoints))
	}
	if checkpoints[0].BlockNumber != 100 {
		t.Fatalf("expected block 100, got %d", checkpoints[0].BlockNumber)
	}
	if checkpoints[0].BlockHash != "hash100" {
		t.Fatalf("expected hash hash100, got %s", checkpoints[0].BlockHash)
	}
}

func TestCreateMultipleCheckpoints(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")

	for i := uint64(1); i <= 5; i++ {
		err := rs.CreateCheckpoint(i*10, "hash", "root", []byte("data"))
		if err != nil {
			t.Fatalf("CreateCheckpoint %d failed: %v", i, err)
		}
	}
	if len(rs.GetCheckpoints()) != 5 {
		t.Fatalf("expected 5 checkpoints, got %d", len(rs.GetCheckpoints()))
	}
}

func TestCheckpointLimit(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")

	for i := uint64(1); i <= 15; i++ {
		err := rs.CreateCheckpoint(i*10, "hash", "root", []byte("data"))
		if err != nil {
			t.Fatalf("CreateCheckpoint %d failed: %v", i, err)
		}
	}

	checkpoints := rs.GetCheckpoints()
	if len(checkpoints) > 10 {
		t.Fatalf("expected at most 10 checkpoints, got %d", len(checkpoints))
	}
}

func TestFindCheckpointBefore(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")

	rs.CreateCheckpoint(100, "h100", "r100", []byte("d100"))
	rs.CreateCheckpoint(200, "h200", "r200", []byte("d200"))
	rs.CreateCheckpoint(300, "h300", "r300", []byte("d300"))

	tests := []struct {
		block    uint64
		expected uint64
	}{
		{50, 0},
		{100, 100},
		{150, 100},
		{200, 200},
		{250, 200},
		{300, 300},
		{999, 300},
	}

	for _, tc := range tests {
		cp := rs.findCheckpointBefore(tc.block)
		if tc.expected == 0 {
			if cp != nil {
				t.Fatalf("block %d: expected nil, got %d", tc.block, cp.BlockNumber)
			}
		} else {
			if cp == nil {
				t.Fatalf("block %d: expected checkpoint %d, got nil", tc.block, tc.expected)
			} else if cp.BlockNumber != tc.expected {
				t.Fatalf("block %d: expected %d, got %d", tc.block, tc.expected, cp.BlockNumber)
			}
		}
	}
}

func TestRollbackToBlock(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	rs := NewRecoveryState(dir, backupDir)

	rs.CreateCheckpoint(100, "h100", "r100", []byte("checkpoint data 100"))
	err := rs.RollbackToBlock(150)
	if err != nil {
		t.Fatalf("RollbackToBlock failed: %v", err)
	}

	files, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("failed to read backup dir: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected backup files after rollback")
	}
}

func TestRollbackToBlockNoCheckpoint(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")
	err := rs.RollbackToBlock(50)
	if err == nil {
		t.Fatal("expected error when no checkpoint found")
	}
}

func TestRollbackToBlockEmptyData(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")
	rs.CreateCheckpoint(100, "h100", "r100", nil)
	err := rs.RollbackToBlock(150)
	if err == nil {
		t.Fatal("expected error for empty checkpoint data")
	}
}

func TestDetectFork(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")

	head1 := &ForkHead{Hash: "hashA", Height: 100, Weight: 10, Timestamp: 1000}
	head2 := &ForkHead{Hash: "hashB", Height: 100, Weight: 20, Timestamp: 1001}

	_, err := rs.DetectFork(head1, head2)
	if err != nil {
		t.Fatalf("DetectFork failed: %v", err)
	}

	winner := rs.GetForkResolution()
	if winner == nil {
		t.Fatal("expected fork resolution winner")
	}
	if winner.Hash != "hashB" {
		t.Fatalf("expected hashB (higher weight) to win, got %s", winner.Hash)
	}
}

func TestDetectForkDeduplicate(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")

	head1 := &ForkHead{Hash: "sameHash", Height: 100, Weight: 10}
	head2 := &ForkHead{Hash: "sameHash", Height: 100, Weight: 10}

	_, err := rs.DetectFork(head1, head2)
	if err == nil {
		t.Fatal("expected error for duplicate fork heads")
	}
}

func TestRecordValidatorVote(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")

	rs.RecordValidatorVote("val1", "hashA")
	rs.RecordValidatorVote("val1", "hashA")
	rs.RecordValidatorVote("val2", "hashA")

	if rs.forkResolver.validatorVotes["val1-hashA"] != 2 {
		t.Fatalf("expected 2 votes from val1 for hashA")
	}
	if rs.forkResolver.validatorVotes["val2-hashA"] != 1 {
		t.Fatalf("expected 1 vote from val2 for hashA")
	}
}

func TestCreateSnapshot(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")
	snapshot, err := rs.CreateSnapshot(500, "root500")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.BlockNumber != 500 {
		t.Fatalf("expected block 500, got %d", snapshot.BlockNumber)
	}
	if snapshot.StateRoot != "root500" {
		t.Fatalf("expected root500, got %s", snapshot.StateRoot)
	}
}

func TestExportImportSnapshot(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")
	snapshot, _ := rs.CreateSnapshot(100, "root100")
	snapshot.Accounts["0xabc"] = &AccountState{Balance: "1000", Nonce: 1}

	path := filepath.Join(t.TempDir(), "snapshot.json")
	err := rs.ExportSnapshot(snapshot, path)
	if err != nil {
		t.Fatalf("ExportSnapshot failed: %v", err)
	}

	imported, err := rs.ImportSnapshot(path)
	if err != nil {
		t.Fatalf("ImportSnapshot failed: %v", err)
	}
	if imported.BlockNumber != 100 {
		t.Fatalf("expected block 100, got %d", imported.BlockNumber)
	}
	if imported.Accounts["0xabc"].Balance != "1000" {
		t.Fatalf("expected balance 1000, got %s", imported.Accounts["0xabc"].Balance)
	}
}

func TestImportSnapshotInvalidFile(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	if _, err := rs.ImportSnapshot(path); err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestVerifyChainIntegrity(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")

	checkpoints := []*Checkpoint{
		{BlockNumber: 100, BlockHash: "h100"},
		{BlockNumber: 200, BlockHash: "h200"},
		{BlockNumber: 300, BlockHash: "h300"},
	}

	if err := rs.VerifyChainIntegrity(checkpoints); err != nil {
		t.Fatalf("VerifyChainIntegrity failed: %v", err)
	}
}

func TestVerifyChainIntegrityOutOfOrder(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")

	checkpoints := []*Checkpoint{
		{BlockNumber: 200, BlockHash: "h200"},
		{BlockNumber: 100, BlockHash: "h100"},
	}

	if err := rs.VerifyChainIntegrity(checkpoints); err == nil {
		t.Fatal("expected error for out-of-order checkpoints")
	}
}

func TestVerifyChainIntegritySingleCheckpoint(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")
	checkpoints := []*Checkpoint{
		{BlockNumber: 100, BlockHash: "h100"},
	}
	if err := rs.VerifyChainIntegrity(checkpoints); err != nil {
		t.Fatalf("single checkpoint should pass: %v", err)
	}
}

func TestEmergencyPause(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")
	err := rs.EmergencyPause("test reason")
	if err != nil {
		t.Fatalf("EmergencyPause failed: %v", err)
	}

	log := rs.GetRecoveryLog()
	if len(log) == 0 {
		t.Fatal("expected recovery log entries")
	}

	found := false
	for _, entry := range log {
		if entry.Action == "emergency_pause" && entry.Description == "test reason" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected emergency_pause entry in recovery log")
	}
}

func TestGetRecoveryLog(t *testing.T) {
	rs := NewRecoveryState("/tmp/chain", "/tmp/backup")
	rs.CreateCheckpoint(100, "h", "r", []byte("d"))
	rs.RollbackToBlock(150)

	log := rs.GetRecoveryLog()
	if len(log) < 2 {
		t.Fatalf("expected at least 2 log entries, got %d", len(log))
	}
}

func TestNewChainSplitDetector(t *testing.T) {
	csd := NewChainSplitDetector(2)
	if csd == nil {
		t.Fatal("expected non-nil ChainSplitDetector")
	}
}

func TestChainSplitDetectorNoSplit(t *testing.T) {
	csd := NewChainSplitDetector(2)
	csd.ReportHead("val1", "hashA")
	csd.ReportHead("val2", "hashA")
	csd.ReportHead("val3", "hashA")

	heads, split := csd.DetectSplit()
	if split {
		t.Fatalf("expected no split, got heads: %v", heads)
	}
}

func TestChainSplitDetectorWithSplit(t *testing.T) {
	csd := NewChainSplitDetector(1)
	csd.ReportHead("val1", "hashA")
	csd.ReportHead("val2", "hashB")

	heads, split := csd.DetectSplit()
	if !split {
		t.Fatal("expected split detection")
	}
	if len(heads) != 2 {
		t.Fatalf("expected 2 different heads, got %d: %v", len(heads), heads)
	}
}

func TestChainSplitDetectorBelowThreshold(t *testing.T) {
	csd := NewChainSplitDetector(3)
	csd.ReportHead("val1", "hashA")
	csd.ReportHead("val2", "hashA")
	csd.ReportHead("val3", "hashB")
	csd.ReportHead("val4", "hashB")

	heads, split := csd.DetectSplit()
	if split {
		t.Fatalf("expected no split below threshold, got heads: %v", heads)
	}
}

func TestChainSplitDetectorEmpty(t *testing.T) {
	csd := NewChainSplitDetector(2)
	heads, split := csd.DetectSplit()
	if split {
		t.Fatal("expected no split when empty")
	}
	if heads != nil {
		t.Fatalf("expected nil heads, got %v", heads)
	}
}

func TestComputeStateHash(t *testing.T) {
	accounts := map[string]*AccountState{
		"0xabc": {Balance: "1000", Nonce: 1},
		"0xdef": {Balance: "2000", Nonce: 2},
	}

	hash1 := ComputeStateHash(accounts)
	if hash1 == "" {
		t.Fatal("expected non-empty hash")
	}

	hash2 := ComputeStateHash(accounts)
	if hash1 != hash2 {
		t.Fatal("hash should be deterministic")
	}
}

func TestComputeStateHashDifferent(t *testing.T) {
	accounts1 := map[string]*AccountState{
		"0xabc": {Balance: "1000"},
	}
	accounts2 := map[string]*AccountState{
		"0xabc": {Balance: "2000"},
	}

	h1 := ComputeStateHash(accounts1)
	h2 := ComputeStateHash(accounts2)
	if h1 == h2 {
		t.Fatal("different state should produce different hashes")
	}
}

func TestComputeStateHashEmpty(t *testing.T) {
	hash := ComputeStateHash(map[string]*AccountState{})
	if hash == "" {
		t.Fatal("expected non-empty hash even for empty state")
	}
}

func TestRestoreFromCheckpointCreatesDir(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backup")
	rs := NewRecoveryState(dir, backupDir)

	cp := &Checkpoint{
		BlockNumber: 100,
		BlockHash:   "h100",
		Data:        []byte(`{"test": true}`),
	}

	err := rs.restoreFromCheckpoint(cp)
	if err != nil {
		t.Fatalf("restoreFromCheckpoint failed: %v", err)
	}

	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		t.Fatal("backup directory was not created")
	}

	backupFile := filepath.Join(backupDir, "checkpoint_100.json")
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Fatal("backup file was not created")
	}
}
