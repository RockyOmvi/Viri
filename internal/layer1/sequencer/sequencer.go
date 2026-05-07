package sequencer

import (
	"fmt"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer1/ledger"
)

type SequencerConfig struct {
	BatchSize       int
	BatchTimeout    time.Duration
	MaxBlockSize    uint64
	MaxGasPerBlock  uint64
}

type Sequencer struct {
	mu          sync.Mutex
	config      SequencerConfig
	pending     []*ledger.Transaction
	proposers   [][]byte
	currentProposer int
	blockchain  *ledger.PersistentBlockchain
	lastBatch   time.Time
	running     bool
	done        chan struct{}
}

func DefaultSequencerConfig() SequencerConfig {
	return SequencerConfig{
		BatchSize:      100,
		BatchTimeout:   2 * time.Second,
		MaxBlockSize:   10 * 1024 * 1024,
		MaxGasPerBlock: 30_000_000,
	}
}

func NewSequencer(config SequencerConfig, bc *ledger.PersistentBlockchain) *Sequencer {
	return &Sequencer{
		config:     config,
		pending:    make([]*ledger.Transaction, 0),
		blockchain: bc,
		done:       make(chan struct{}),
	}
}

func (s *Sequencer) AddTransaction(tx *ledger.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("sequencer not running")
	}

	s.pending = append(s.pending, tx)
	return nil
}

func (s *Sequencer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("sequencer already running")
	}

	s.running = true
	s.lastBatch = time.Now()

	go s.batchLoop()

	return nil
}

func (s *Sequencer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.done)
}

func (s *Sequencer) batchLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			if len(s.pending) >= s.config.BatchSize || time.Since(s.lastBatch) > s.config.BatchTimeout {
				if len(s.pending) > 0 {
					s.createBlock()
				}
			}
			s.mu.Unlock()
		case <-s.done:
			return
		}
	}
}

func (s *Sequencer) createBlock() {
	batch := s.pending
	if len(batch) > s.config.BatchSize {
		batch = batch[:s.config.BatchSize]
	}

	totalGas := uint64(0)
	totalSize := uint64(0)

	for _, tx := range batch {
		totalGas += tx.GasLimit
		totalSize += uint64(len(tx.Data)) + 256
	}

	if totalGas > s.config.MaxGasPerBlock || totalSize > s.config.MaxBlockSize {
		batch = s.filterBatch(batch)
	}

	if len(batch) == 0 {
		return
	}

	s.pending = s.pending[len(batch):]
	s.lastBatch = time.Now()
}

func (s *Sequencer) filterBatch(batch []*ledger.Transaction) []*ledger.Transaction {
	var filtered []*ledger.Transaction
	totalGas := uint64(0)
	totalSize := uint64(0)

	for _, tx := range batch {
		if totalGas+tx.GasLimit <= s.config.MaxGasPerBlock && totalSize+uint64(len(tx.Data))+256 <= s.config.MaxBlockSize {
			filtered = append(filtered, tx)
			totalGas += tx.GasLimit
			totalSize += uint64(len(tx.Data)) + 256
		}
	}

	return filtered
}

func (s *Sequencer) PendingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

func (s *Sequencer) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
