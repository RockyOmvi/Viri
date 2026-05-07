package mev

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"
)

type MempoolMode string

const (
	StandardMode   MempoolMode = "standard"
	EncryptedMode  MempoolMode = "encrypted"
	CommitReveal   MempoolMode = "commit_reveal"
)

type EncryptedTx struct {
	ID          []byte
	Commitment  []byte
	Encrypted   []byte
	Sender      []byte
	Nonce       uint64
	GasTipCap   *big.Int
	GasFeeCap   *big.Int
	Timestamp   uint64
	Status      TxStatus
	Decrypted   []byte
}

type Commitment struct {
	TxID       []byte
	Commitment []byte
	Sender     []byte
	RevealData []byte
	Nonce      uint64
	Submitted  uint64
	Revealed   bool
}

type PBSBid struct {
	BlockBuilder   []byte
	BidAmount      *big.Int
	BlockHash      []byte
	Transactions   [][]byte
	StateRoot      []byte
	Signature      []byte
	Timestamp      uint64
}

type PBSState struct {
	mu         sync.RWMutex
	bids       []*PBSBid
	winningBid *PBSBid
	round      uint64
	proposer   []byte
}

type MEVState struct {
	mu             sync.RWMutex
	mode           MempoolMode
	encryptedTxs   map[string]*EncryptedTx
	commitments    map[string]*Commitment
	pbsState       *PBSState
	commitWindow   uint64
	revealWindow   uint64
	flushQueue     []*EncryptedTx
}

type TxStatus int

const (
	TxSubmitted TxStatus = iota
	TxRevealed
	TxDecrypted
	TxIncluded
	TxExpired
)

func NewMEVState(mode MempoolMode) *MEVState {
	return &MEVState{
		mode:         mode,
		encryptedTxs: make(map[string]*EncryptedTx),
		commitments:  make(map[string]*Commitment),
		pbsState: &PBSState{
			bids: make([]*PBSBid, 0),
		},
		commitWindow: 60,
		revealWindow: 60,
	}
}

func (ms *MEVState) SubmitEncryptedTx(tx *EncryptedTx) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.mode != EncryptedMode && ms.mode != CommitReveal {
		return fmt.Errorf("encrypted mode not active")
	}

	ms.encryptedTxs[string(tx.ID)] = tx
	return nil
}

func (ms *MEVState) SubmitCommitment(sender []byte, commitment []byte, nonce uint64) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.mode != CommitReveal {
		return fmt.Errorf("commit-reveal mode not active")
	}

	key := fmt.Sprintf("%x:%d", sender, nonce)
	ms.commitments[key] = &Commitment{
		TxID:       sha256.New().Sum([]byte(key)),
		Commitment: commitment,
		Sender:     sender,
		Nonce:      nonce,
		Submitted:  uint64(time.Now().Unix()),
	}

	return nil
}

func (ms *MEVState) RevealTransaction(sender []byte, nonce uint64, txData []byte) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.mode != CommitReveal {
		return fmt.Errorf("commit-reveal mode not active")
	}

	key := fmt.Sprintf("%x:%d", sender, nonce)
	commit, ok := ms.commitments[key]
	if !ok {
		return fmt.Errorf("no commitment found")
	}

	if commit.Revealed {
		return fmt.Errorf("already revealed")
	}

	if uint64(time.Now().Unix()) > commit.Submitted + ms.revealWindow {
		return fmt.Errorf("reveal window expired")
	}

	expectedCommit := sha256.Sum256(txData)
	if !bytes.Equal(commit.Commitment, expectedCommit[:]) {
		return fmt.Errorf("commitment mismatch")
	}

	commit.RevealData = txData
	commit.Revealed = true

	ms.flushQueue = append(ms.flushQueue, &EncryptedTx{
		ID:        commit.TxID,
		Sender:    sender,
		Nonce:     nonce,
		Decrypted: txData,
		Status:    TxRevealed,
	})

	return nil
}

func (ms *MEVState) SubmitPBSBid(bid *PBSBid) error {
	ms.pbsState.mu.Lock()
	defer ms.pbsState.mu.Unlock()

	if bid.BidAmount.Sign() <= 0 {
		return fmt.Errorf("bid amount must be positive")
	}

	ms.pbsState.bids = append(ms.pbsState.bids, bid)

	sort.Slice(ms.pbsState.bids, func(i, j int) bool {
		return ms.pbsState.bids[i].BidAmount.Cmp(ms.pbsState.bids[j].BidAmount) > 0
	})

	if len(ms.pbsState.bids) > 0 {
		ms.pbsState.winningBid = ms.pbsState.bids[0]
	}

	return nil
}

func (ms *MEVState) GetWinningBid() *PBSBid {
	ms.pbsState.mu.RLock()
	defer ms.pbsState.mu.RUnlock()
	return ms.pbsState.winningBid
}

func (ms *MEVState) SetProposer(proposer []byte) {
	ms.pbsState.mu.Lock()
	defer ms.pbsState.mu.Unlock()
	ms.pbsState.proposer = proposer
}

func (ms *MEVState) StartNewRound(round uint64, proposer []byte) {
	ms.pbsState.mu.Lock()
	defer ms.pbsState.mu.Unlock()

	ms.pbsState.round = round
	ms.pbsState.proposer = proposer
	ms.pbsState.bids = make([]*PBSBid, 0)
	ms.pbsState.winningBid = nil
}

func (ms *MEVState) FlushRevealedTxs() []*EncryptedTx {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	flushed := ms.flushQueue
	ms.flushQueue = make([]*EncryptedTx, 0)

	for _, tx := range flushed {
		tx.Status = TxIncluded
	}

	return flushed
}

func (ms *MEVState) OrderTransactions(txs []*StandardTx) []*StandardTx {
	sorted := make([]*StandardTx, len(txs))
	copy(sorted, txs)

	sort.Slice(sorted, func(i, j int) bool {
		tipI := sorted[i].GasTipCap
		tipJ := sorted[j].GasTipCap
		cmp := tipI.Cmp(tipJ)
		if cmp != 0 {
			return cmp > 0
		}
		return sorted[i].Timestamp < sorted[j].Timestamp
	})

	return sorted
}

func (ms *MEVState) SetMode(mode MempoolMode) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.mode = mode
}

func (ms *MEVState) GetMode() MempoolMode {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.mode
}

func (ms *MEVState) GetEncryptedTxCount() int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return len(ms.encryptedTxs)
}

func (ms *MEVState) GetCommitmentCount() int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return len(ms.commitments)
}

type StandardTx struct {
	ID        []byte
	Sender    []byte
	To        []byte
	Data      []byte
	Nonce     uint64
	GasTipCap *big.Int
	GasFeeCap *big.Int
	GasLimit  uint64
	Value     *big.Int
	Timestamp uint64
}

type OrderBundle struct {
	Transactions []*StandardTx
	ExpectedProfit *big.Int
	Sender       []byte
}

func (ms *MEVState) SubmitBundle(bundle *OrderBundle) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if len(bundle.Transactions) == 0 {
		return fmt.Errorf("empty bundle")
	}

	return nil
}

func ComputeCommitment(txData []byte) []byte {
	h := sha256.New()
	h.Write(txData)
	return h.Sum(nil)
}

func ValidatePBSBid(bid *PBSBid) error {
	if len(bid.BlockBuilder) != 20 {
		return fmt.Errorf("invalid builder address")
	}
	if bid.BidAmount.Sign() <= 0 {
		return fmt.Errorf("invalid bid amount")
	}
	if len(bid.BlockHash) != 32 {
		return fmt.Errorf("invalid block hash")
	}
	if len(bid.Signature) == 0 {
		return fmt.Errorf("missing signature")
	}
	return nil
}

type AuctionResult struct {
	Winner    []byte
	BidAmount *big.Int
	Transactions [][]byte
	BlockHash []byte
	Round     uint64
}

func (ms *MEVState) RunAuction() *AuctionResult {
	ms.pbsState.mu.Lock()
	defer ms.pbsState.mu.Unlock()

	if len(ms.pbsState.bids) == 0 {
		return nil
	}

	winner := ms.pbsState.bids[0]
	return &AuctionResult{
		Winner:       winner.BlockBuilder,
		BidAmount:    winner.BidAmount,
		Transactions: winner.Transactions,
		BlockHash:    winner.BlockHash,
		Round:        ms.pbsState.round,
	}
}

func (ms *MEVState) EncryptTransaction(txData []byte, key []byte) (*EncryptedTx, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes")
	}

	encrypted := make([]byte, len(txData))
	for i := range txData {
		encrypted[i] = txData[i] ^ key[i%32]
	}

	commitment := ComputeCommitment(txData)

	return &EncryptedTx{
		ID:         sha256.New().Sum(txData),
		Commitment: commitment,
		Encrypted:  encrypted,
		Timestamp:  uint64(time.Now().Unix()),
		Status:     TxSubmitted,
	}, nil
}

func (ms *MEVState) DecryptTransaction(tx *EncryptedTx, key []byte) ([]byte, error) {
	if tx.Status != TxSubmitted {
		return nil, fmt.Errorf("transaction not in submitted state")
	}

	decrypted := make([]byte, len(tx.Encrypted))
	for i := range tx.Encrypted {
		decrypted[i] = tx.Encrypted[i] ^ key[i%32]
	}

	expectedCommit := ComputeCommitment(decrypted)
	if !bytes.Equal(tx.Commitment, expectedCommit) {
		return nil, fmt.Errorf("commitment verification failed")
	}

	tx.Decrypted = decrypted
	tx.Status = TxDecrypted
	return decrypted, nil
}
