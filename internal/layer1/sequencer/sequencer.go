package sequencer

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

type SequencerConfig struct {
	BatchSize       int
	BatchTimeout    time.Duration
	MaxBlockSize    uint64
	MaxGasPerBlock  uint64
	ProposerKey     *crypto.PrivateKey
}

func (c SequencerConfig) Validate() error {
	if c.BatchSize <= 0 {
		return fmt.Errorf("BatchSize must be positive")
	}
	if c.BatchTimeout <= 0 {
		return fmt.Errorf("BatchTimeout must be positive")
	}
	if c.MaxBlockSize == 0 {
		return fmt.Errorf("MaxBlockSize must be positive")
	}
	if c.MaxGasPerBlock == 0 {
		return fmt.Errorf("MaxGasPerBlock must be positive")
	}
	return nil
}

type Sequencer struct {
	mu          sync.Mutex
	wg          sync.WaitGroup
	config      SequencerConfig
	pending     []*ledger.Transaction
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

	if tx == nil {
		return fmt.Errorf("transaction must not be nil")
	}
	if tx.GasLimit == 0 {
		return fmt.Errorf("transaction gas limit must be positive")
	}
	if tx.Signature == nil {
		return fmt.Errorf("transaction must be signed")
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

	if err := s.config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	s.running = true
	s.lastBatch = time.Now()
	s.done = make(chan struct{})

	s.wg.Add(1)
	go s.batchLoop()

	return nil
}

func (s *Sequencer) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.done)
	s.mu.Unlock()

	s.wg.Wait()
}

func (s *Sequencer) batchLoop() {
	defer s.wg.Done()
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
	overflow := false

	for _, tx := range batch {
		if totalGas > math.MaxUint64-tx.GasLimit {
			overflow = true
			break
		}
		totalGas += tx.GasLimit
		if totalSize > math.MaxUint64-uint64(len(tx.Data))+256 {
			overflow = true
			break
		}
		totalSize += uint64(len(tx.Data)) + 256
	}

	if overflow || totalGas > s.config.MaxGasPerBlock || totalSize > s.config.MaxBlockSize {
		batch = s.filterBatch(batch)
	}

	if len(batch) == 0 {
		return
	}

	if s.config.ProposerKey == nil {
		return
	}

	height := s.blockchain.Height() + 1
	prevHash := s.blockchain.TipHash()

	block, err := ledger.NewBlock(height, prevHash, batch, s.config.ProposerKey.PubKey().Address(), s.config.ProposerKey)
	if err != nil {
		return
	}

	if err := s.blockchain.AddBlock(block); err != nil {
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
		nextGas := totalGas + tx.GasLimit
		nextSize := totalSize + uint64(len(tx.Data)) + 256

		if nextGas >= totalGas && nextGas <= s.config.MaxGasPerBlock &&
			nextSize >= totalSize && nextSize <= s.config.MaxBlockSize {
			filtered = append(filtered, tx)
			totalGas = nextGas
			totalSize = nextSize
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
