package sync

import (
	"context"
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

func (m *mockFetcher) GetRemoteHeight(ctx context.Context) (uint64, error) {
	return m.remoteHeight, m.getRemoteError
}

func (m *mockFetcher) GetHeaders(ctx context.Context, from, to uint64) ([]*ledger.Header, error) {
	return m.headers, m.getHeadersErr
}

func (m *mockFetcher) GetBlocks(ctx context.Context, from, to uint64) ([]*ledger.Block, error) {
	return m.blocks, m.getBlocksErr
}

func (m *mockFetcher) ApplyBlock(ctx context.Context, block *ledger.Block) error {
	return m.applyBlockErr
}

func (m *mockFetcher) ApplyHeader(ctx context.Context, header *ledger.Header) error {
	return m.applyHeaderErr
}

func (m *mockFetcher) GetStateSnapshot(ctx context.Context, height uint64) (*ledger.StateSnapshot, error) {
	if len(m.snapshots) > 0 {
		return m.snapshots[0], m.getSnapErr
	}
	return nil, m.getSnapErr
}

func (m *mockFetcher) GetStateSnapshots(ctx context.Context, from, to uint64) ([]*ledger.StateSnapshot, error) {
	return m.snapshots, m.getSnapErr
}

func (m *mockFetcher) ApplyStateSnapshot(ctx context.Context, snapshot *ledger.StateSnapshot) error {
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

func TestSyncConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *SyncConfig
		wantErr bool
	}{
		{"valid default", DefaultSyncConfig(), false},
		{"zero pivot interval", &SyncConfig{PivotInterval: 0, BatchSize: 128, Timeout: time.Second, MaxRetries: 3}, true},
		{"zero batch size", &SyncConfig{PivotInterval: 64, BatchSize: 0, Timeout: time.Second, MaxRetries: 3}, true},
		{"zero timeout", &SyncConfig{PivotInterval: 64, BatchSize: 128, Timeout: 0, MaxRetries: 3}, true},
		{"negative max retries", &SyncConfig{PivotInterval: 64, BatchSize: 128, Timeout: time.Second, MaxRetries: -1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestStartWithNilFetcher(t *testing.T) {
	logger := logging.NewLogger("test", logging.INFO, "text")
	syncer := NewSyncer(nil, logger)
	err := syncer.Start(0, nil)
	if err == nil {
		t.Error("expected error when starting with nil fetcher")
	}
}
