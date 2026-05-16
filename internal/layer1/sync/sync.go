package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
)

type SyncMode uint8

const (
	FullSync SyncMode = iota
	FastSync
	SnapSync
)

type SyncConfig struct {
	Mode          SyncMode
	PivotInterval uint64
	BatchSize     int
	Timeout       time.Duration
	MaxRetries    int
}

func (c *SyncConfig) Validate() error {
	if c.PivotInterval == 0 {
		return fmt.Errorf("PivotInterval must be > 0")
	}
	if c.BatchSize < 1 {
		return fmt.Errorf("BatchSize must be >= 1")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("Timeout must be positive")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("MaxRetries must be >= 0")
	}
	return nil
}

func DefaultSyncConfig() *SyncConfig {
	return &SyncConfig{
		Mode:          FastSync,
		PivotInterval: 64,
		BatchSize:     128,
		Timeout:       10 * time.Second,
		MaxRetries:    3,
	}
}

type SyncState uint8

const (
	SyncIdle SyncState = iota
	SyncingHeaders
	SyncingState
	SyncingBlocks
	SyncComplete
	SyncFailed
)

type SyncProgress struct {
	State          SyncState
	StartingHeight uint64
	CurrentHeight  uint64
	HighestHeight  uint64
	PivotBlock     uint64
	Phase          string
	ETA            time.Duration
}

type Syncer struct {
	mu       sync.Mutex
	config   *SyncConfig
	state    SyncState
	progress SyncProgress
	logger   *logging.Logger
	done     chan struct{}
}

func NewSyncer(config *SyncConfig, log *logging.Logger) *Syncer {
	if config == nil {
		config = DefaultSyncConfig()
	}
	return &Syncer{
		config: config,
		logger: log,
		done:   make(chan struct{}),
	}
}

func (s *Syncer) Start(initialHeight uint64, fetcher SyncFetcher) error {
	s.mu.Lock()
	if s.state != SyncIdle {
		s.mu.Unlock()
		return fmt.Errorf("sync already in progress")
	}

	if fetcher == nil {
		s.mu.Unlock()
		return fmt.Errorf("fetcher must not be nil")
	}

	if err := s.config.Validate(); err != nil {
		s.mu.Unlock()
		return fmt.Errorf("invalid config: %w", err)
	}

	s.state = SyncingHeaders
	s.progress.StartingHeight = initialHeight
	s.mu.Unlock()

	go s.run(initialHeight, fetcher)
	return nil
}

func (s *Syncer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

func (s *Syncer) Progress() SyncProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

func (s *Syncer) IsSyncing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state != SyncIdle && s.state != SyncComplete && s.state != SyncFailed
}

func (s *Syncer) IsComplete() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == SyncComplete
}

func (s *Syncer) IsFailed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == SyncFailed
}

func (s *Syncer) run(initialHeight uint64, fetcher SyncFetcher) {
	syncOK := false
	defer func() {
		s.mu.Lock()
		if syncOK {
			s.state = SyncComplete
		} else if s.state != SyncIdle {
			s.state = SyncFailed
		}
		s.mu.Unlock()
		if syncOK {
			s.logger.Info("Sync completed")
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), s.config.Timeout)
	defer cancel()

	remoteHeight, err := fetcher.GetRemoteHeight(ctx)
	if err != nil {
		s.logger.WithField("error", err.Error()).Error("Failed to get remote height")
		return
	}

	s.mu.Lock()
	s.progress.HighestHeight = remoteHeight
	s.mu.Unlock()

	if remoteHeight <= initialHeight {
		s.logger.Info("Already in sync")
		syncOK = true
		return
	}

	s.logger.WithField("local", initialHeight).
		WithField("remote", remoteHeight).
		WithField("blocks_behind", remoteHeight-initialHeight).
		Info("Starting sync")

	switch s.config.Mode {
	case FullSync:
		syncOK = s.syncFull(ctx, initialHeight, remoteHeight, fetcher)
	case FastSync:
		syncOK = s.syncFast(ctx, initialHeight, remoteHeight, fetcher)
	case SnapSync:
		syncOK = s.syncSnap(ctx, initialHeight, remoteHeight, fetcher)
	}
}

func (s *Syncer) syncFull(ctx context.Context, from, to uint64, fetcher SyncFetcher) bool {
	s.mu.Lock()
	s.state = SyncingBlocks
	s.progress.Phase = "downloading_blocks"
	s.mu.Unlock()

	start := time.Now()
	batchSize := uint64(s.config.BatchSize)

	for height := from + 1; height <= to; height += batchSize {
		select {
		case <-s.done:
			s.logger.Info("Sync cancelled")
			return false
		default:
		}

		end := height + batchSize - 1
		if end > to {
			end = to
		}

		var blocks []*ledger.Block
		var err error
		for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
			blocks, err = fetcher.GetBlocks(ctx, height, end)
			if err == nil {
				break
			}
			s.logger.WithField("error", err.Error()).
				WithField("from", height).
				WithField("to", end).
				WithField("attempt", attempt+1).
				Warn("Failed to fetch block batch, retrying...")
			height -= batchSize
		}
		if err != nil {
			s.logger.WithField("error", err.Error()).
				WithField("from", height).
				WithField("to", end).
				Error("Failed to fetch block batch after retries")
			return false
		}

		for _, block := range blocks {
			if err := fetcher.ApplyBlock(ctx, block); err != nil {
				s.logger.WithField("error", err.Error()).
					WithField("height", block.Header.Height).
					Error("Failed to apply block")
				return false
			}

			s.mu.Lock()
			s.progress.CurrentHeight = block.Header.Height
			elapsed := time.Since(start)
			remaining := to - block.Header.Height
			speed := float64(block.Header.Height-from) / elapsed.Seconds()
			if speed > 0 {
				s.progress.ETA = time.Duration(float64(remaining)/speed) * time.Second
			}
			eta := s.progress.ETA
			s.mu.Unlock()

			s.logger.WithField("height", end).
				WithField("progress", fmt.Sprintf("%.1f%%", float64(end-from)/float64(to-from)*100)).
				WithField("eta", eta).
				Debug("Block sync progress")
		}
	}
	return true
}

func (s *Syncer) syncFast(ctx context.Context, from, to uint64, fetcher SyncFetcher) bool {
	s.mu.Lock()
	s.state = SyncingHeaders
	s.progress.Phase = "downloading_headers"
	s.mu.Unlock()

	pivotBlock := to - (to % s.config.PivotInterval)
	if pivotBlock < from {
		pivotBlock = to
	}

	s.logger.WithField("pivot", pivotBlock).Info("Downloading headers to pivot block")

	headers, err := fetcher.GetHeaders(ctx, from+1, pivotBlock)
	if err != nil {
		s.logger.WithField("error", err.Error()).Error("Failed to fetch headers")
		return false
	}

	for _, header := range headers {
		select {
		case <-s.done:
			s.logger.Info("Sync cancelled")
			return false
		default:
		}

		if err := fetcher.ApplyHeader(ctx, header); err != nil {
			s.logger.WithField("error", err.Error()).
				WithField("height", header.Height).
				Warn("Failed to apply header")
			continue
		}

		s.mu.Lock()
		s.progress.CurrentHeight = header.Height
		s.progress.PivotBlock = pivotBlock
		s.mu.Unlock()
	}

	s.mu.Lock()
	s.state = SyncingState
	s.progress.Phase = "downloading_state"
	s.mu.Unlock()

	s.logger.WithField("pivot", pivotBlock).Info("Downloading state snapshot")

	snapshot, err := fetcher.GetStateSnapshot(ctx, pivotBlock)
	if err != nil {
		s.logger.WithField("error", err.Error()).Error("Failed to fetch state snapshot")
		return false
	}

	if err := fetcher.ApplyStateSnapshot(ctx, snapshot); err != nil {
		s.logger.WithField("error", err.Error()).Error("Failed to apply state snapshot")
		return false
	}

	s.mu.Lock()
	s.state = SyncingBlocks
	s.progress.Phase = "downloading_recent_blocks"
	s.mu.Unlock()

	s.logger.Info("Downloading recent blocks after pivot")

	return s.syncFastRecentBlocks(ctx, pivotBlock, to, fetcher)
}

func (s *Syncer) syncFastRecentBlocks(ctx context.Context, from, to uint64, fetcher SyncFetcher) bool {
	start := time.Now()
	batchSize := uint64(s.config.BatchSize)

	for height := from + 1; height <= to; height += batchSize {
		select {
		case <-s.done:
			return false
		default:
		}

		end := height + batchSize - 1
		if end > to {
			end = to
		}

		var blocks []*ledger.Block
		var err error
		for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
			blocks, err = fetcher.GetBlocks(ctx, height, end)
			if err == nil {
				break
			}
			s.logger.WithField("error", err.Error()).Warn("Failed to fetch blocks, retrying")
			height -= batchSize
		}
		if err != nil {
			s.logger.WithField("error", err.Error()).
				WithField("from", height).
				WithField("to", end).
				Error("Failed to fetch blocks after retries")
			return false
		}

		for _, block := range blocks {
			if err := fetcher.ApplyBlock(ctx, block); err != nil {
				s.logger.WithField("error", err.Error()).Error("Failed to apply block")
				return false
			}

			s.mu.Lock()
			s.progress.CurrentHeight = block.Header.Height
			elapsed := time.Since(start)
			remaining := to - block.Header.Height
			speed := float64(block.Header.Height-from) / elapsed.Seconds()
			if speed > 0 {
				s.progress.ETA = time.Duration(float64(remaining)/speed) * time.Second
			}
			s.mu.Unlock()
		}
	}
	return true
}

func (s *Syncer) syncSnap(ctx context.Context, from, to uint64, fetcher SyncFetcher) bool {
	s.mu.Lock()
	s.state = SyncingState
	s.progress.Phase = "downloading_snapshots"
	s.mu.Unlock()

	s.logger.Info("Starting snap sync")

	snapshots, err := fetcher.GetStateSnapshots(ctx, from, to)
	if err != nil {
		s.logger.WithField("error", err.Error()).Error("Failed to fetch snapshots")
		return false
	}

	for _, snapshot := range snapshots {
		select {
		case <-s.done:
			return false
		default:
		}

		if err := fetcher.ApplyStateSnapshot(ctx, snapshot); err != nil {
			s.logger.WithField("error", err.Error()).Error("Failed to apply snapshot")
			return false
		}

		s.mu.Lock()
		s.progress.CurrentHeight = snapshot.BlockHeight
		s.mu.Unlock()
	}

	s.mu.Lock()
	s.state = SyncingBlocks
	s.progress.Phase = "syncing_recent"
	s.mu.Unlock()

	return s.syncFastRecentBlocks(ctx, from, to, fetcher)
}

type SyncFetcher interface {
	GetRemoteHeight(ctx context.Context) (uint64, error)
	GetHeaders(ctx context.Context, from, to uint64) ([]*ledger.Header, error)
	GetBlocks(ctx context.Context, from, to uint64) ([]*ledger.Block, error)
	ApplyBlock(ctx context.Context, block *ledger.Block) error
	ApplyHeader(ctx context.Context, header *ledger.Header) error
	GetStateSnapshot(ctx context.Context, height uint64) (*ledger.StateSnapshot, error)
	GetStateSnapshots(ctx context.Context, from, to uint64) ([]*ledger.StateSnapshot, error)
	ApplyStateSnapshot(ctx context.Context, snapshot *ledger.StateSnapshot) error
}
