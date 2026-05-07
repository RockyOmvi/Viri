package zk

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

type ShieldedTxType uint8

const (
	ShieldedTxTypeDeposit ShieldedTxType = iota
	ShieldedTxTypeWithdraw
	ShieldedTxTypeTransfer
)

type ShieldedTransaction struct {
	Type        ShieldedTxType
	Commitment  []byte
	Nullifier   []byte
	Proof       *Proof
	PublicData  []byte
	Signature   []byte
	Sender      []byte
	Receiver    []byte
	Amount      uint64
	Timestamp   time.Time
	Nonce       uint64
}

type ShieldedPool struct {
	commitments map[string]bool
	nullifiers  map[string]bool
	proofs      []*Proof
	transactions []*ShieldedTransaction
	vk          *VerifyingKey
	circuit     *Circuit
	nonce       uint64
}

func NewShieldedPool(circuit *Circuit, vk *VerifyingKey) *ShieldedPool {
	return &ShieldedPool{
		commitments: make(map[string]bool),
		nullifiers:  make(map[string]bool),
		vk:          vk,
		circuit:     circuit,
	}
}

func (p *ShieldedTransaction) Serialize() ([]byte, error) {
	return json.Marshal(p)
}

func (p *ShieldedTransaction) Deserialize(data []byte) error {
	return json.Unmarshal(data, p)
}

func (p *ShieldedTransaction) ComputeHash() []byte {
	h := sha256.New()

	h.Write([]byte{byte(p.Type)})
	h.Write(p.Commitment)
	h.Write(p.Nullifier)
	h.Write(p.PublicData)
	h.Write([]byte(fmt.Sprintf("%d", p.Amount)))
	h.Write([]byte(fmt.Sprintf("%d", p.Timestamp.UnixNano())))
	h.Write([]byte(fmt.Sprintf("%d", p.Nonce)))

	return h.Sum(nil)
}

func (p *ShieldedTransaction) Validate() error {
	if len(p.Commitment) == 0 {
		return fmt.Errorf("missing commitment")
	}

	if len(p.Nullifier) == 0 {
		return fmt.Errorf("missing nullifier")
	}

	if p.Proof == nil {
		return fmt.Errorf("missing proof")
	}

	if p.Amount == 0 && p.Type != ShieldedTxTypeTransfer {
		return fmt.Errorf("amount required for deposit/withdraw")
	}

	return nil
}

func (p *ShieldedPool) ProcessDeposit(amount uint64, sender []byte, proof *Proof) (*ShieldedTransaction, error) {
	verifier := NewVerifier(p.vk, p.circuit)
	if err := verifier.Verify(proof); err != nil {
		return nil, fmt.Errorf("invalid proof: %w", err)
	}

	commitment := proof.ComputeCommitment()
	commitmentKey := string(commitment)

	if p.commitments[commitmentKey] {
		return nil, fmt.Errorf("duplicate commitment")
	}

	tx := &ShieldedTransaction{
		Type:        ShieldedTxTypeDeposit,
		Commitment:  commitment,
		Nullifier:   generateNullifier(sender, p.nonce),
		Proof:       proof,
		Sender:      sender,
		Amount:      amount,
		Timestamp:   time.Now(),
		Nonce:       p.nonce,
	}

	txHash := tx.ComputeHash()
	tx.PublicData = txHash

	p.commitments[commitmentKey] = true
	p.nullifiers[string(tx.Nullifier)] = true
	p.proofs = append(p.proofs, proof)
	p.transactions = append(p.transactions, tx)
	p.nonce++

	return tx, nil
}

func (p *ShieldedPool) ProcessWithdraw(amount uint64, receiver []byte, nullifier []byte, proof *Proof) (*ShieldedTransaction, error) {
	nullifierKey := string(nullifier)
	if !p.nullifiers[nullifierKey] {
		return nil, fmt.Errorf("nullifier not found")
	}

	verifier := NewVerifier(p.vk, p.circuit)
	if err := verifier.Verify(proof); err != nil {
		return nil, fmt.Errorf("invalid proof: %w", err)
	}

	tx := &ShieldedTransaction{
		Type:        ShieldedTxTypeWithdraw,
		Commitment:  proof.ComputeCommitment(),
		Nullifier:   nullifier,
		Proof:       proof,
		Receiver:    receiver,
		Amount:      amount,
		Timestamp:   time.Now(),
		Nonce:       p.nonce,
	}

	txHash := tx.ComputeHash()
	tx.PublicData = txHash

	p.proofs = append(p.proofs, proof)
	p.transactions = append(p.transactions, tx)
	p.nonce++

	return tx, nil
}

func (p *ShieldedPool) ProcessTransfer(amount uint64, sender, receiver []byte, proof *Proof) (*ShieldedTransaction, error) {
	verifier := NewVerifier(p.vk, p.circuit)
	if err := verifier.Verify(proof); err != nil {
		return nil, fmt.Errorf("invalid proof: %w", err)
	}

	commitment := proof.ComputeCommitment()
	commitmentKey := string(commitment)

	if p.commitments[commitmentKey] {
		return nil, fmt.Errorf("duplicate commitment")
	}

	nullifier := generateNullifier(sender, p.nonce)
	nullifierKey := string(nullifier)

	if p.nullifiers[nullifierKey] {
		return nil, fmt.Errorf("nullifier already used")
	}

	tx := &ShieldedTransaction{
		Type:        ShieldedTxTypeTransfer,
		Commitment:  commitment,
		Nullifier:   nullifier,
		Proof:       proof,
		Sender:      sender,
		Receiver:    receiver,
		Amount:      amount,
		Timestamp:   time.Now(),
		Nonce:       p.nonce,
	}

	txHash := tx.ComputeHash()
	tx.PublicData = txHash

	p.commitments[commitmentKey] = true
	p.nullifiers[nullifierKey] = true
	p.proofs = append(p.proofs, proof)
	p.transactions = append(p.transactions, tx)
	p.nonce++

	return tx, nil
}

func (p *ShieldedPool) GetCommitmentCount() int {
	return len(p.commitments)
}

func (p *ShieldedPool) GetNullifierCount() int {
	return len(p.nullifiers)
}

func (p *ShieldedPool) HasCommitment(commitment []byte) bool {
	return p.commitments[string(commitment)]
}

func (p *ShieldedPool) HasNullifier(nullifier []byte) bool {
	return p.nullifiers[string(nullifier)]
}

func (p *ShieldedPool) GetProofCount() int {
	return len(p.proofs)
}

func (p *ShieldedPool) GetTransactions() []*ShieldedTransaction {
	return p.transactions
}

func generateNullifier(sender []byte, nonce uint64) []byte {
	h := sha256.New()
	h.Write(sender)
	h.Write([]byte(fmt.Sprintf("%d", nonce)))
	return h.Sum(nil)
}

func NewShieldedProof(assignment *Assignment, pk *ProvingKey) (*Proof, error) {
	prover := NewProver(pk, NewShieldedTransferCircuit())
	return prover.Prove(assignment)
}
