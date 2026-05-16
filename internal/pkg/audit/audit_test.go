package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func TestNewAuditLogger(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer logger.Close()

	if logger.file == nil {
		t.Fatal("expected file to be opened")
	}
}

func TestLogProposal(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.LogProposal(1, 0, "proposer1", "hash1")
	time.Sleep(50 * time.Millisecond)

	files, _ := filepath.Glob(filepath.Join(dir, "audit_*.log"))
	if len(files) == 0 {
		t.Fatal("expected audit file to be created")
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("expected log entry to be written")
	}

	var entry AuditEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatal(err)
	}
	if entry.EventType != EventTypeProposal {
		t.Errorf("expected event type %s, got %s", EventTypeProposal, entry.EventType)
	}
	if entry.SeqNo != 1 {
		t.Errorf("expected seq 1, got %d", entry.SeqNo)
	}
}

func TestLogVote(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.LogVote(1, 0, "prepare", "validator1", "hash1")
	time.Sleep(50 * time.Millisecond)

	entry, err := logger.GetEntry(1)
	if err != nil {
		t.Fatal(err)
	}
	if entry.EventType != EventTypeVote {
		t.Errorf("expected event type %s, got %s", EventTypeVote, entry.EventType)
	}
}

func TestLogViewChange(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.LogViewChange(0, 1, "timeout")
	time.Sleep(50 * time.Millisecond)

	entry, err := logger.GetEntry(1)
	if err != nil {
		t.Fatal(err)
	}
	if entry.EventType != EventTypeViewChange {
		t.Errorf("expected event type %s, got %s", EventTypeViewChange, entry.EventType)
	}
}

func TestLogFinalize(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.LogFinalize(1, "hash1", "proposer1", "proof")
	time.Sleep(50 * time.Millisecond)

	entry, err := logger.GetEntry(1)
	if err != nil {
		t.Fatal(err)
	}
	if entry.EventType != EventTypeFinalize {
		t.Errorf("expected event type %s, got %s", EventTypeFinalize, entry.EventType)
	}
}

func TestLogTimeout(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.LogTimeout(1, 0, 3)
	time.Sleep(50 * time.Millisecond)

	entry, err := logger.GetEntry(1)
	if err != nil {
		t.Fatal(err)
	}
	if entry.EventType != EventTypeTimeout {
		t.Errorf("expected event type %s, got %s", EventTypeTimeout, entry.EventType)
	}
}

func TestLogSync(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.LogSync("start", 0, 100, 0)
	time.Sleep(50 * time.Millisecond)

	entry, err := logger.GetEntry(1)
	if err != nil {
		t.Fatal(err)
	}
	if entry.EventType != EventTypeSync {
		t.Errorf("expected event type %s, got %s", EventTypeSync, entry.EventType)
	}
}

func TestLogValidator(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.LogValidator("slash", "validator1", 1000000, "double_sign")
	time.Sleep(50 * time.Millisecond)

	entry, err := logger.GetEntry(1)
	if err != nil {
		t.Fatal(err)
	}
	if entry.EventType != EventTypeValidator {
		t.Errorf("expected event type %s, got %s", EventTypeValidator, entry.EventType)
	}
}

func TestVerifyAuditChain(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}

	logger.LogProposal(1, 0, "p1", "h1")
	logger.LogVote(1, 0, "prepare", "v1", "h1")
	logger.LogFinalize(1, "h1", "p1", "")
	time.Sleep(50 * time.Millisecond)

	if err := logger.Close(); err != nil {
		t.Fatal(err)
	}

	logger2, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger2.Close()

	if err := logger2.VerifyAuditChain(); err != nil {
		t.Errorf("expected valid chain, got error: %v", err)
	}
}

func TestVerifyAuditChainDetectTampering(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}

	logger.LogProposal(1, 0, "p1", "h1")
	time.Sleep(50 * time.Millisecond)
	logger.Close()

	files, _ := filepath.Glob(filepath.Join(dir, "audit_*.log"))
	if len(files) == 0 {
		t.Fatal("expected audit file")
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}

	var entry AuditEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatal(err)
	}

	tampered := entry
	tampered.EventData = EventProposal{Height: 999, View: 0, Proposer: "hacker", BlockHash: "evil"}
	tamperedData, _ := json.Marshal(tampered)
	if err := os.WriteFile(files[0], tamperedData, 0644); err != nil {
		t.Fatal(err)
	}

	logger2, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger2.Close()

	if err := logger2.VerifyAuditChain(); err == nil {
		t.Error("expected chain verification to fail after tampering")
	}
}

func TestGetEntryNotFound(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	logger.LogProposal(1, 0, "p1", "h1")
	time.Sleep(50 * time.Millisecond)

	_, err = logger.GetEntry(999)
	if err == nil {
		t.Error("expected error for non-existent entry")
	}
}

func TestExportAuditLog(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}

	for i := uint64(1); i <= 10; i++ {
		logger.LogProposal(i, 0, fmt.Sprintf("p%d", i), fmt.Sprintf("h%d", i))
	}
	time.Sleep(100 * time.Millisecond)
	logger.Close()

	logger2, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger2.Close()

	entries, err := logger2.ExportAuditLog(3, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}
}

func TestAuditChainWithSigner(t *testing.T) {
	dir := t.TempDir()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
		Signer:        key,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}

	logger.LogProposal(1, 0, "p1", "h1")
	time.Sleep(50 * time.Millisecond)

	entry, err := logger.GetEntry(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Signature) == 0 {
		t.Error("expected signature to be present")
	}
	logger.Close()
}

func TestBatchFlush(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     10,
		FlushInterval: 20 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		logger.LogProposal(uint64(i+1), 0, "p", "h")
	}
	time.Sleep(200 * time.Millisecond)

	entry, err := logger.GetEntry(1)
	if err != nil {
		t.Fatalf("expected entry to be flushed, got error: %v", err)
	}
	if entry.SeqNo != 1 {
		t.Errorf("expected seq 1, got %d", entry.SeqNo)
	}
	logger.Close()
}

func TestDefaultAuditConfig(t *testing.T) {
	config := DefaultAuditConfig()
	if config.BatchSize != DefaultBatchSize {
		t.Errorf("expected batch size %d, got %d", DefaultBatchSize, config.BatchSize)
	}
	if config.FlushInterval != DefaultFlushInterval {
		t.Errorf("expected flush interval %v, got %v", DefaultFlushInterval, config.FlushInterval)
	}
	if config.OutputPath != "audit" {
		t.Errorf("expected output path 'audit', got %s", config.OutputPath)
	}
}

func TestMultipleLogFileRotation(t *testing.T) {
	dir := t.TempDir()
	config := &AuditConfig{
		OutputPath:    dir,
		BatchSize:     1,
		FlushInterval: 10 * time.Millisecond,
	}

	logger, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}

	for i := uint64(1); i <= 5; i++ {
		logger.LogProposal(i, 0, "p", "h")
	}
	time.Sleep(100 * time.Millisecond)
	logger.Close()

	logger2, err := NewAuditLogger(config)
	if err != nil {
		t.Fatal(err)
	}
	defer logger2.Close()

	files, _ := filepath.Glob(filepath.Join(dir, "audit_*.log"))
	if len(files) == 0 {
		t.Fatal("expected at least one audit file")
	}

	if err := logger2.VerifyAuditChain(); err != nil {
		t.Errorf("expected valid chain, got: %v", err)
	}
}
