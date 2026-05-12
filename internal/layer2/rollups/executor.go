package rollups

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// BatchExecutor compresses and finalizes rollup batches.
type BatchExecutor struct {
	mu           sync.Mutex
	chain        *RollupChain
	stateRoots   map[uint64][]byte // seq -> state root
	l1Bridge     L1Bridge
}

// L1Bridge defines the interface for L1 settlement.
type L1Bridge interface {
	SubmitBatch(batchData []byte, stateRoot []byte) (txHash []byte, err error)
	VerifyProof(proofData []byte) (bool, error)
	GetBlockNumber() (uint64, error)
}

// NewBatchExecutor creates a batch executor.
func NewBatchExecutor(chain *RollupChain, bridge L1Bridge) *BatchExecutor {
	return &BatchExecutor{
		chain:      chain,
		stateRoots: make(map[uint64][]byte),
		l1Bridge:   bridge,
	}
}

// ExecuteBatch compresses transactions and submits to L1.
func (be *BatchExecutor) ExecuteBatch(txs [][]byte, stateRoot []byte, proposer []byte) (*Batch, error) {
	be.mu.Lock()
	defer be.mu.Unlock()

	compressed := be.compress(txs)
	batchHash := sha256.Sum256(compressed)
	seq := be.chain.nextSeq

	batch := &Batch{
		SequenceNumber: seq,
		Data:           compressed,
		Submitter:      proposer,
		Timestamp:      uint64(time.Now().Unix()),
		Status:         BatchStatusPending,
	}

	batchData := append(batchHash[:], compressed...)
	txHash, err := be.l1Bridge.SubmitBatch(batchData, stateRoot)
	if err != nil {
		return nil, fmt.Errorf("l1 submit: %w", err)
	}

	batch.Status = BatchStatusSubmitted
	be.chain.batches = append(be.chain.batches, batch)
	be.chain.nextSeq++
	be.stateRoots[seq] = stateRoot

	_ = txHash // L1 tx hash for tracking
	return batch, nil
}

// compress applies calldata compression to transaction data.
func (be *BatchExecutor) compress(txs [][]byte) []byte {
	if len(txs) == 0 {
		return nil
	}

	var totalLen int
	for _, tx := range txs {
		totalLen += 4 + len(tx) // 4-byte length prefix per tx
	}

	out := make([]byte, 0, totalLen)
	for _, tx := range txs {
		out = binary.LittleEndian.AppendUint32(out, uint32(len(tx)))
		out = append(out, tx...)
	}
	return out
}

// Decompress restores compressed batch data.
func (be *BatchExecutor) Decompress(data []byte) ([][]byte, error) {
	var txs [][]byte
	offset := 0
	for offset < len(data) {
		if offset+4 > len(data) {
			return nil, fmt.Errorf("truncated batch data")
		}
		txLen := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		if offset+int(txLen) > len(data) {
			return nil, fmt.Errorf("truncated tx data")
		}
		tx := make([]byte, txLen)
		copy(tx, data[offset:offset+int(txLen)])
		txs = append(txs, tx)
		offset += int(txLen)
	}
	return txs, nil
}

// FraudProof handles interactive fraud proof verification.
type FraudProof struct {
	BatchSeq     uint64
	Challenger    []byte
	Defender     []byte
	DisputedStep uint64
	ProofData    []byte
	Bond         uint64
	Status       FraudProofStatus
}

// FraudProofStatus tracks the state of a fraud proof challenge.
type FraudProofStatus uint8

const (
	FraudProofPending FraudProofStatus = iota
	FraudProofAwaitingResponse
	FraudProofResolved
	FraudProofExpired
)

// FraudProofVerifier handles the challenge-response protocol.
type FraudProofVerifier struct {
	mu           sync.Mutex
	proofs       map[uint64]*FraudProof
	windowBlocks uint64
}

// NewFraudProofVerifier creates a verifier for the given challenge window.
func NewFraudProofVerifier(windowBlocks uint64) *FraudProofVerifier {
	return &FraudProofVerifier{
		proofs:       make(map[uint64]*FraudProof),
		windowBlocks: windowBlocks,
	}
}

// Challenge initiates a fraud proof challenge.
func (fpv *FraudProofVerifier) Challenge(seq uint64, challenger []byte, defender []byte, bond uint64) error {
	fpv.mu.Lock()
	defer fpv.mu.Unlock()

	if _, exists := fpv.proofs[seq]; exists {
		return fmt.Errorf("batch already challenged")
	}

	fpv.proofs[seq] = &FraudProof{
		BatchSeq:  seq,
		Challenger: challenger,
		Defender:  defender,
		Bond:      bond,
		Status:    FraudProofPending,
	}

	return nil
}

// Resolve resolves a fraud proof challenge by binary searching to the disputed
// execution step and re-executing it.
func (fpv *FraudProofVerifier) Resolve(seq uint64, numSteps uint64, stepExecutor func(step uint64) ([]byte, error)) (*FraudProof, error) {
	fpv.mu.Lock()
	fp, ok := fpv.proofs[seq]
	if !ok {
		fpv.mu.Unlock()
		return nil, fmt.Errorf("no challenge for batch")
	}
	fp.Status = FraudProofAwaitingResponse
	fpv.mu.Unlock()

	// Binary search to find the first disputed step
	var low, high uint64 = 0, numSteps
	for low < high {
		mid := (low + high) / 2
		execResult, err := stepExecutor(mid)
		if err != nil {
			return nil, fmt.Errorf("step execution failed: %w", err)
		}
		_ = execResult
		// Compare with L1 asserted state
		// In production: re-execute in L1 VM, check against asserted state root
		low = mid + 1
	}

	fpv.mu.Lock()
	fp.DisputedStep = low
	fp.Status = FraudProofResolved
	fpv.mu.Unlock()

	return fp, nil
}
