package sync

import (
	"errors"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
)

type mockFetcher struct {
	remoteHeight   uint64
	headers        []*ledger.Header
	blocks         []*ledger.Block
	snapshots      []*ledger.StateSnapshot
	getRemoteError error
	getHeadersErr  error
	getBlocksErr   error
	applyBlockErr  error
	applyHeaderErr error
	applySnapErr   error
	getSnapErr     error
}

func (m *mockFetcher) GetRemoteHeight() (uint64, error) {
	return m.remoteHeight, m.getRemoteError
}

func (m *mockFetcher) GetHeaders(from, to uint64) ([]*ledger.Header, error) {
	return m.headers, m.getHeadersErr
}

func (m *mockFetcher) GetBlocks(from, to uint64) ([]*ledger.Block, error) {
	return m.blocks, m.getBlocksErr
}

func (m *mockFetcher) ApplyBlock(block *ledger.Block) error {
	return m.applyBlockErr
}

func (m *mockFetcher) ApplyHeader(header *ledger.Header) error {
	return m.applyHeaderErr
}

func (m *mockFetcher) GetStateSnapshot(height uint64) (*ledger.StateSnapshot, error) {
	if len(m.snapshots) > 0 {
		return m.snapshots[0], m.getSnapErr
	}
	return nil, m.getSnapErr
}

func (m *mockFetcher) GetStateSnapshots(from, to uint64) ([]*ledger.StateSnapshot, error) {
	return m.snapshots, m.getSnapErr
}

func (m *mockFetcher) ApplyStateSnapshot(snapshot *ledger.StateSnapshot) error {
	return m.applySnapErr
}

func TestSyncInitialization(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "text")
	syncer := NewSyncer(nil, logger)

	if syncer == nil {
		t.Fatal("expected syncer to be created")
	}

	if syncer.IsSyncing() {
		t.Error("expected syncer not to be syncing initially")
	}

	if syncer.IsComplete() {
		t.Error("expected syncer not to be complete initially")
	}
}

func TestSyncInitialization_WithConfig(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "text")
	config := &SyncConfig{
		Mode:       FullSync,
		BatchSize:  64,
		Timeout:    5 * time.Second,
		MaxRetries: 5,
	}
	syncer := NewSyncer(config, logger)

	if syncer == nil {
		t.Fatal("expected syncer to be created")
	}

	progress := syncer.Progress()
	if progress.State != SyncIdle {
		t.Error("expected initial state to be SyncIdle")
	}
}

func TestPhaseTransitions_FastSync(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "text")
	config := DefaultSyncConfig()
	config.Mode = FastSync
	syncer := NewSyncer(config, logger)

	fetcher := &mockFetcher{
		remoteHeight: 10,
		headers: []*ledger.Header{
			{Height: 1},
			{Height: 2},
			{Height: 3},
		},
		blocks: []*ledger.Block{
			{Header: &ledger.Header{Height: 4}},
			{Header: &ledger.Header{Height: 5}},
		},
		snapshots: []*ledger.StateSnapshot{
			{BlockHeight: 3},
		},
	}

	err := syncer.Start(0, fetcher)
	if err != nil {
		t.Fatalf("failed to start sync: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if syncer.IsSyncing() {
		t.Error("expected syncer not to be syncing after completion")
	}

	if !syncer.IsComplete() {
		t.Error("expected sync to be complete")
	}
}

func TestProgressReporting(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "text")
	syncer := NewSyncer(nil, logger)

	fetcher := &mockFetcher{
		remoteHeight: 100,
		blocks:       make([]*ledger.Block, 0),
	}

	syncer.Start(0, fetcher)

	time.Sleep(100 * time.Millisecond)

	progress := syncer.Progress()
	if progress.StartingHeight != 0 {
		t.Errorf("expected starting height 0, got %d", progress.StartingHeight)
	}
}

func TestCompletionDetection(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "text")
	syncer := NewSyncer(nil, logger)

	fetcher := &mockFetcher{
		remoteHeight: 5,
		blocks: []*ledger.Block{
			{Header: &ledger.Header{Height: 1}},
			{Header: &ledger.Header{Height: 2}},
			{Header: &ledger.Header{Height: 3}},
			{Header: &ledger.Header{Height: 4}},
			{Header: &ledger.Header{Height: 5}},
		},
	}

	syncer.Start(0, fetcher)

	time.Sleep(300 * time.Millisecond)

	if !syncer.IsComplete() {
		t.Error("expected sync to be complete")
	}
}

func TestSyncStop(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "text")
	syncer := NewSyncer(nil, logger)

	fetcher := &mockFetcher{
		remoteHeight: 1000,
		blocks:      make([]*ledger.Block, 0),
	}

	syncer.Start(0, fetcher)

	time.Sleep(50 * time.Millisecond)

	syncer.Stop()

	time.Sleep(100 * time.Millisecond)

	if syncer.IsSyncing() {
		t.Error("expected syncer not to be syncing after stop")
	}
}

func TestErrorHandling_GetRemoteHeightFails(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "text")
	syncer := NewSyncer(nil, logger)

	fetcher := &mockFetcher{
		getRemoteError: errors.New("network error"),
	}

	err := syncer.Start(0, fetcher)
	if err != nil {
		t.Fatalf("Start should not return error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestErrorHandling_ApplyBlockFails(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "text")
	syncer := NewSyncer(nil, logger)

	fetcher := &mockFetcher{
		remoteHeight:  2,
		applyBlockErr: errors.New("apply error"),
		blocks: []*ledger.Block{
			{Header: &ledger.Header{Height: 1}},
			{Header: &ledger.Header{Height: 2}},
		},
	}

	syncer.Start(0, fetcher)

	time.Sleep(300 * time.Millisecond)
}

func TestSyncAlreadyInProgress(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "text")
	syncer := NewSyncer(nil, logger)

	fetcher := &mockFetcher{
		remoteHeight: 10,
		blocks:      make([]*ledger.Block, 0),
	}

	err := syncer.Start(0, fetcher)
	if err != nil {
		t.Fatalf("first start should succeed: %v", err)
	}

	err = syncer.Start(0, fetcher)
	if err == nil {
		t.Error("expected error when starting sync that is already in progress")
	}
}

func TestFullSync(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "text")
	config := DefaultSyncConfig()
	config.Mode = FullSync
	syncer := NewSyncer(config, logger)

	blocks := make([]*ledger.Block, 5)
	for i := range blocks {
		blocks[i] = &ledger.Block{Header: &ledger.Header{Height: uint64(i + 1)}}
	}

	fetcher := &mockFetcher{
		remoteHeight: 5,
		blocks:      blocks,
	}

	syncer.Start(0, fetcher)

	time.Sleep(300 * time.Millisecond)

	if !syncer.IsComplete() {
		t.Error("expected full sync to complete")
	}
}
