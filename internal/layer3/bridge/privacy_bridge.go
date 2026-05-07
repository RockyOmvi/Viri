package bridge

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer2/zk"
)

// PrivacyBridgeTransfer represents a transfer with ZK proof validation
type PrivacyBridgeTransfer struct {
	ID                  string
	SourceChain         string
	DestChain           string
	Amount              uint64
	Token               []byte
	DepositProof        *zk.Proof
	WithdrawalProof     *zk.Proof
	Commitment          []byte
	Nullifier           []byte
	Status              TransferStatus
	Timestamp           uint64
	DepositTxHash       []byte
	WithdrawalTxHash    []byte
	ValidatorSigs       map[string]bool
	RequiredSigs        int
	ProofVerificationID string
}

// PrivacyBridge extends ChainBridge with ZK proof validation for privacy-preserving transfers
type PrivacyBridge struct {
	mu              sync.RWMutex
	transfers       map[string]*PrivacyBridgeTransfer
	chains          map[string]*ChainInfo
	validators      map[string]bool
	threshold       int
	circuit         *zk.Circuit
	verifyingKey    *zk.VerifyingKey
	provingKey      *zk.ProvingKey
	commitments     map[string]bool // For replay protection
	nullifiers      map[string]bool  // For double-spend prevention
	proofCache      map[string]*zk.Proof
	verifiedProofs  map[string]uint64 // proofID -> timestamp
}

// NewPrivacyBridge creates a new privacy-preserving bridge with ZK proof validation
func NewPrivacyBridge(threshold int, circuit *zk.Circuit, vk *zk.VerifyingKey, pk *zk.ProvingKey) *PrivacyBridge {
	return &PrivacyBridge{
		transfers:      make(map[string]*PrivacyBridgeTransfer),
		chains:         make(map[string]*ChainInfo),
		validators:     make(map[string]bool),
		threshold:      threshold,
		circuit:        circuit,
		verifyingKey:   vk,
		provingKey:     pk,
		commitments:    make(map[string]bool),
		nullifiers:     make(map[string]bool),
		proofCache:     make(map[string]*zk.Proof),
		verifiedProofs: make(map[string]uint64),
	}
}

// RegisterChain registers a chain for cross-chain transfers
func (pb *PrivacyBridge) RegisterChain(id, name, endpoint string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.chains[id] = &ChainInfo{
		ID:       id,
		Name:     name,
		Endpoint: endpoint,
		IsActive: true,
	}
}

// RegisterValidator registers a validator for proof verification
func (pb *PrivacyBridge) RegisterValidator(validatorID string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.validators[validatorID] = true
}

// InitiatePrivacyTransfer initiates a privacy-preserving transfer with ZK proof
func (pb *PrivacyBridge) InitiatePrivacyTransfer(
	sourceChain, destChain string,
	amount uint64,
	token []byte,
	depositProof *zk.Proof,
) (*PrivacyBridgeTransfer, error) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	// Validate chains exist
	if _, exists := pb.chains[sourceChain]; !exists {
		return nil, fmt.Errorf("source chain not registered")
	}
	if _, exists := pb.chains[destChain]; !exists {
		return nil, fmt.Errorf("destination chain not registered")
	}

	// Verify deposit proof
	verifier := zk.NewVerifier(pb.verifyingKey, pb.circuit)
	if err := verifier.Verify(depositProof); err != nil {
		return nil, fmt.Errorf("invalid deposit proof: %w", err)
	}

	// Extract commitment from proof
	commitment := depositProof.ComputeCommitment()
	commitmentKey := string(commitment)

	// Check for replay attacks (duplicate commitment)
	if pb.commitments[commitmentKey] {
		return nil, fmt.Errorf("duplicate commitment detected - possible replay attack")
	}

	// Generate transfer ID
	id := pb.generatePrivacyTransferID(sourceChain, commitment, time.Now().Unix())

	// Create transfer with privacy proofs
	transfer := &PrivacyBridgeTransfer{
		ID:                  id,
		SourceChain:         sourceChain,
		DestChain:           destChain,
		Amount:              amount,
		Token:               token,
		DepositProof:        depositProof,
		Commitment:          commitment,
		Status:              TransferStatusPending,
		Timestamp:           uint64(time.Now().Unix()),
		ValidatorSigs:       make(map[string]bool),
		RequiredSigs:        pb.threshold,
		ProofVerificationID: pb.generateProofVerificationID(depositProof),
	}

	// Store commitment to prevent replay attacks
	pb.commitments[commitmentKey] = true
	pb.transfers[id] = transfer
	pb.proofCache[transfer.ProofVerificationID] = depositProof

	return transfer, nil
}

// CompletePrivacyTransfer completes a privacy transfer with withdrawal proof validation
func (pb *PrivacyBridge) CompletePrivacyTransfer(
	transferID string,
	withdrawalProof *zk.Proof,
	nullifier []byte,
	withdrawalTxHash []byte,
) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	transfer, exists := pb.transfers[transferID]
	if !exists {
		return fmt.Errorf("transfer not found")
	}

	// Verify withdrawal proof
	verifier := zk.NewVerifier(pb.verifyingKey, pb.circuit)
	if err := verifier.Verify(withdrawalProof); err != nil {
		return fmt.Errorf("invalid withdrawal proof: %w", err)
	}

	// Check for double-spend (nullifier already used)
	nullifierKey := string(nullifier)
	if pb.nullifiers[nullifierKey] {
		return fmt.Errorf("nullifier already used - double-spend attempt detected")
	}

	// Verify withdrawal proof is for same commitment
	withdrawalCommitment := withdrawalProof.ComputeCommitment()
	if string(withdrawalCommitment) != string(transfer.Commitment) {
		return fmt.Errorf("withdrawal proof commitment mismatch")
	}

	transfer.WithdrawalProof = withdrawalProof
	transfer.Nullifier = nullifier
	transfer.WithdrawalTxHash = withdrawalTxHash
	transfer.Status = TransferStatusBurned

	// Mark nullifier as used to prevent double-spend
	pb.nullifiers[nullifierKey] = true
	pb.verifiedProofs[pb.generateProofVerificationID(withdrawalProof)] = uint64(time.Now().Unix())

	return nil
}

// AddValidatorSignature adds a validator signature to approve the transfer
func (pb *PrivacyBridge) AddValidatorSignature(transferID, validatorID string) error {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	transfer, exists := pb.transfers[transferID]
	if !exists {
		return fmt.Errorf("transfer not found")
	}

	if !pb.validators[validatorID] {
		return fmt.Errorf("unknown validator")
	}

	transfer.ValidatorSigs[validatorID] = true

	// Update status when threshold is reached
	if len(transfer.ValidatorSigs) >= transfer.RequiredSigs {
		transfer.Status = TransferStatusMinted
	}

	return nil
}

// GetPrivacyTransfer retrieves a privacy-preserving transfer
func (pb *PrivacyBridge) GetPrivacyTransfer(id string) (*PrivacyBridgeTransfer, bool) {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	t, exists := pb.transfers[id]
	if !exists {
		return nil, false
	}

	return t, true
}

// GetPendingPrivacyTransfers returns all pending privacy transfers
func (pb *PrivacyBridge) GetPendingPrivacyTransfers() []*PrivacyBridgeTransfer {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	var pending []*PrivacyBridgeTransfer
	for _, t := range pb.transfers {
		if t.Status == TransferStatusPending || t.Status == TransferStatusLocked {
			pending = append(pending, t)
		}
	}

	return pending
}

// VerifyProofByID verifies if a proof has been previously validated
func (pb *PrivacyBridge) VerifyProofByID(proofID string) (bool, uint64) {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	timestamp, exists := pb.verifiedProofs[proofID]
	return exists, timestamp
}

// HasCommitment checks if a commitment exists (replay protection)
func (pb *PrivacyBridge) HasCommitment(commitment []byte) bool {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	return pb.commitments[string(commitment)]
}

// HasNullifier checks if a nullifier has been used (double-spend prevention)
func (pb *PrivacyBridge) HasNullifier(nullifier []byte) bool {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	return pb.nullifiers[string(nullifier)]
}

// GetTransferCount returns total number of transfers
func (pb *PrivacyBridge) GetTransferCount() int {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	return len(pb.transfers)
}

// GetVerifiedProofCount returns number of verified proofs
func (pb *PrivacyBridge) GetVerifiedProofCount() int {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	return len(pb.verifiedProofs)
}

// PruneOldTransfers removes completed/failed transfers older than maxAge seconds
func (pb *PrivacyBridge) PruneOldTransfers(maxAge uint64) int {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	now := uint64(time.Now().Unix())
	pruned := 0

	for id, transfer := range pb.transfers {
		if (transfer.Status == TransferStatusCompleted || transfer.Status == TransferStatusFailed) &&
			(now-transfer.Timestamp) > maxAge {
			delete(pb.transfers, id)
			pruned++
		}
	}

	return pruned
}

// generatePrivacyTransferID generates a unique transfer ID using commitment
func (pb *PrivacyBridge) generatePrivacyTransferID(chain string, commitment []byte, timestamp int64) string {
	data := append([]byte(chain), commitment...)
	data = append(data, []byte(fmt.Sprintf("%d", timestamp))...)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:8])
}

// generateProofVerificationID generates a unique ID for proof verification caching
func (pb *PrivacyBridge) generateProofVerificationID(proof *zk.Proof) string {
	h := sha256.New()
	
	// Hash all proof elements
	for _, a := range proof.A {
		if a != nil {
			h.Write(a.Bytes())
		}
	}
	for _, b := range proof.B {
		if b != nil {
			h.Write(b.Bytes())
		}
	}
	for _, c := range proof.C {
		if c != nil {
			h.Write(c.Bytes())
		}
	}
	
	hash := h.Sum(nil)
	return fmt.Sprintf("%x", hash[:8])
}
