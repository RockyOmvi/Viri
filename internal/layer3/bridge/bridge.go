package bridge

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"
	"sync"
	"time"
)

type ChainID uint64

const (
	Ethereum    ChainID = 1
	BinanceChain ChainID = 56
	Polygon     ChainID = 137
	Avalanche   ChainID = 43114
	Viri        ChainID = 7777777
)

type BridgeMessage struct {
	ID           []byte
	SourceChain  ChainID
	DestChain    ChainID
	Sender       []byte
	Receiver     []byte
	Token        []byte
	Amount       *big.Int
	Nonce        uint64
	Data         []byte
	Timestamp    uint64
	Signatures   [][]byte
	ValidatorIdx []int
	Status       MessageStatus
}

type MessageStatus int

const (
	Pending   MessageStatus = iota
	Confirmed
	Executed
	Failed
)

type ValidatorSet struct {
	Validators []*BridgeValidator
	Threshold  int
}

type BridgeValidator struct {
	Address    []byte
	PublicKey  []byte
	Stake      uint64
	IsActive   bool
	LastSeen   time.Time
}

type LockEvent struct {
	TxHash    []byte
	Chain     ChainID
	Sender    []byte
	Token     []byte
	Amount    *big.Int
	Nonce     uint64
	BlockNum  uint64
}

type UnlockEvent struct {
	TxHash    []byte
	Chain     ChainID
	Receiver  []byte
	Token     []byte
	Amount    *big.Int
	Nonce     uint64
	BlockNum  uint64
}

type BridgeState struct {
	mu               sync.RWMutex
	pendingMessages  map[string]*BridgeMessage
	executedMessages map[string]bool
	lockEvents       []*LockEvent
	unlockEvents     []*UnlockEvent
	validatorSets    map[ChainID]*ValidatorSet
	sequenceNumbers  map[ChainID]uint64
	totalLocked      map[string]*big.Int
	totalUnlocked    map[string]*big.Int
}

func NewBridgeState() *BridgeState {
	return &BridgeState{
		pendingMessages:  make(map[string]*BridgeMessage),
		executedMessages: make(map[string]bool),
		lockEvents:       make([]*LockEvent, 0),
		unlockEvents:     make([]*UnlockEvent, 0),
		validatorSets:    make(map[ChainID]*ValidatorSet),
		sequenceNumbers:  make(map[ChainID]uint64),
		totalLocked:      make(map[string]*big.Int),
		totalUnlocked:    make(map[string]*big.Int),
	}
}

func (bs *BridgeState) RegisterValidatorSet(chain ChainID, vs *ValidatorSet) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.validatorSets[chain] = vs
}

func (bs *BridgeState) SubmitLockEvent(event *LockEvent) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	bs.lockEvents = append(bs.lockEvents, event)

	tokenKey := fmt.Sprintf("%d:%x", event.Chain, event.Token)
	if _, ok := bs.totalLocked[tokenKey]; !ok {
		bs.totalLocked[tokenKey] = new(big.Int)
	}
	bs.totalLocked[tokenKey].Add(bs.totalLocked[tokenKey], event.Amount)

	return nil
}

func (bs *BridgeState) CreateBridgeMessage(source ChainID, dest ChainID, sender []byte, receiver []byte, token []byte, amount *big.Int, data []byte) (*BridgeMessage, error) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	bs.sequenceNumbers[dest]++
	nonce := bs.sequenceNumbers[dest]

	id := computeMessageID(source, dest, sender, receiver, token, amount, nonce)

	msg := &BridgeMessage{
		ID:          id,
		SourceChain: source,
		DestChain:   dest,
		Sender:      sender,
		Receiver:    receiver,
		Token:       token,
		Amount:      amount,
		Nonce:       nonce,
		Data:        data,
		Timestamp:   uint64(time.Now().Unix()),
		Status:      Pending,
		Signatures:  make([][]byte, 0),
		ValidatorIdx: make([]int, 0),
	}

	bs.pendingMessages[string(id)] = msg
	return msg, nil
}

func (bs *BridgeState) AddSignature(msgID []byte, validatorIdx int, signature []byte) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	msg, ok := bs.pendingMessages[string(msgID)]
	if !ok {
		return fmt.Errorf("message not found")
	}

	vs, ok := bs.validatorSets[msg.SourceChain]
	if !ok {
		return fmt.Errorf("validator set not found for source chain")
	}

	for _, idx := range msg.ValidatorIdx {
		if idx == validatorIdx {
			return fmt.Errorf("validator already signed")
		}
	}

	msg.Signatures = append(msg.Signatures, signature)
	msg.ValidatorIdx = append(msg.ValidatorIdx, validatorIdx)

	totalStake := uint64(0)
	for _, idx := range msg.ValidatorIdx {
		if idx < len(vs.Validators) {
			totalStake += vs.Validators[idx].Stake
		}
	}

	requiredStake := uint64(0)
	for _, v := range vs.Validators {
		if v.IsActive {
			requiredStake += v.Stake
		}
	}
	requiredStake = requiredStake * 2 / 3

	if totalStake >= requiredStake {
		msg.Status = Confirmed
	}

	return nil
}

func (bs *BridgeState) ExecuteMessage(msgID []byte) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	msg, ok := bs.pendingMessages[string(msgID)]
	if !ok {
		return fmt.Errorf("message not found")
	}

	if msg.Status != Confirmed {
		return fmt.Errorf("message not confirmed")
	}

	if bs.executedMessages[string(msgID)] {
		return fmt.Errorf("message already executed")
	}

	bs.executedMessages[string(msgID)] = true
	msg.Status = Executed

	tokenKey := fmt.Sprintf("%d:%x", msg.DestChain, msg.Token)
	if _, ok := bs.totalUnlocked[tokenKey]; !ok {
		bs.totalUnlocked[tokenKey] = new(big.Int)
	}
	bs.totalUnlocked[tokenKey].Add(bs.totalUnlocked[tokenKey], msg.Amount)

	return nil
}

func (bs *BridgeState) VerifyMessage(msg *BridgeMessage) error {
	vs, ok := bs.validatorSets[msg.SourceChain]
	if !ok {
		return fmt.Errorf("validator set not found")
	}

	if len(msg.Signatures) < vs.Threshold {
		return fmt.Errorf("insufficient signatures: %d < %d", len(msg.Signatures), vs.Threshold)
	}

	validStake := uint64(0)
	totalStake := uint64(0)
	for i, v := range vs.Validators {
		if !v.IsActive {
			continue
		}
		totalStake += v.Stake

		for _, idx := range msg.ValidatorIdx {
			if idx == i {
				validStake += v.Stake
				break
			}
		}
	}

	if validStake < totalStake*2/3 {
		return fmt.Errorf("insufficient validator stake: %d < %d", validStake, totalStake*2/3)
	}

	return nil
}

func (bs *BridgeState) GetPendingMessages(chain ChainID) []*BridgeMessage {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	result := make([]*BridgeMessage, 0)
	for _, msg := range bs.pendingMessages {
		if msg.DestChain == chain && msg.Status == Pending {
			result = append(result, msg)
		}
	}
	return result
}

func (bs *BridgeState) GetMessage(msgID []byte) *BridgeMessage {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.pendingMessages[string(msgID)]
}

func (bs *BridgeState) GetTotalLocked(chain ChainID, token []byte) *big.Int {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	key := fmt.Sprintf("%d:%x", chain, token)
	if val, ok := bs.totalLocked[key]; ok {
		return val
	}
	return new(big.Int)
}

func (bs *BridgeState) GetTotalUnlocked(chain ChainID, token []byte) *big.Int {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	key := fmt.Sprintf("%d:%x", chain, token)
	if val, ok := bs.totalUnlocked[key]; ok {
		return val
	}
	return new(big.Int)
}

func computeMessageID(source, dest ChainID, sender, receiver, token []byte, amount *big.Int, nonce uint64) []byte {
	h := sha256.New()
	binary.Write(h, binary.BigEndian, source)
	binary.Write(h, binary.BigEndian, dest)
	h.Write(sender)
	h.Write(receiver)
	h.Write(token)
	h.Write(amount.Bytes())
	binary.Write(h, binary.BigEndian, nonce)
	return h.Sum(nil)
}

type CrossChainCall struct {
	SourceChain   ChainID
	DestChain     ChainID
	ContractAddr  []byte
	Method        string
	Args          [][]byte
	GasLimit      uint64
	MaxFee        *big.Int
	CallbackAddr  []byte
}

type OracleProof struct {
	BlockHash    []byte
	TxIndex      uint32
	MerkleProof  [][]byte
	StateRoot    []byte
	ReceiptRoot  []byte
	LogsBloom    []byte
}

type BridgeContract struct {
	Address      []byte
	SourceChain  ChainID
	TargetChain  ChainID
	TargetAddr   []byte
	IsPaused     bool
	DailyLimit   *big.Int
	CurrentDayUsed *big.Int
	LastDayReset uint64
}

func (bc *BridgeContract) ProcessOutbound(msg *BridgeMessage) error {
	if bc.IsPaused {
		return fmt.Errorf("bridge is paused")
	}

	now := uint64(time.Now().Unix())
	day := now / 86400
	if day > bc.LastDayReset {
		bc.CurrentDayUsed = new(big.Int)
		bc.LastDayReset = day
	}

	if bc.CurrentDayUsed.Add(bc.CurrentDayUsed, msg.Amount).Cmp(bc.DailyLimit) > 0 {
		return fmt.Errorf("daily limit exceeded")
	}

	bc.CurrentDayUsed.Add(bc.CurrentDayUsed, msg.Amount)
	return nil
}

func (bc *BridgeContract) VerifyOracleProof(proof *OracleProof, msg *BridgeMessage) bool {
	if len(proof.BlockHash) != 32 {
		return false
	}
	if len(proof.MerkleProof) == 0 {
		return false
	}
	if len(proof.StateRoot) != 32 {
		return false
	}
	return true
}

type TransferStatus int

const (
	TransferStatusPending TransferStatus = iota
	TransferStatusLocked
	TransferStatusBurned
	TransferStatusMinted
	TransferStatusCompleted
	TransferStatusFailed
)

type ChainInfo struct {
	ID         string
	Name       string
	Endpoint   string
	IsActive   bool
	ContractAddr []byte
	BlockHeight uint64
}

type BridgeTransfer struct {
	ID           string
	SourceChain  string
	DestChain    string
	Sender       []byte
	Receiver     []byte
	Amount       uint64
	Token        []byte
	Status       TransferStatus
	LockTxHash   []byte
	MintTxHash   []byte
	Sigs         map[string]bool
}

type ChainBridge struct {
	mu           sync.RWMutex
	chains       map[string]*ChainInfo
	transfers    map[string]*BridgeTransfer
	validators   map[string]bool
	threshold    int
	state        *BridgeState
	nextID       int
}

func NewChainBridge(threshold int) *ChainBridge {
	return &ChainBridge{
		chains:     make(map[string]*ChainInfo),
		transfers:  make(map[string]*BridgeTransfer),
		validators: make(map[string]bool),
		state:      NewBridgeState(),
		threshold:  threshold,
	}
}

func (cb *ChainBridge) AddChain(info *ChainInfo) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.chains[info.ID] = info
}

func (cb *ChainBridge) GetTransfers() []*BridgeTransfer {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	result := make([]*BridgeTransfer, 0, len(cb.transfers))
	for _, t := range cb.transfers {
		result = append(result, t)
	}
	return result
}

func (cb *ChainBridge) GetPendingTransfers() []*BridgeTransfer {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	result := make([]*BridgeTransfer, 0)
	for _, t := range cb.transfers {
		if t.Status == TransferStatusPending {
			result = append(result, t)
		}
	}
	return result
}

func (cb *ChainBridge) InitiateTransfer(sourceChain, destChain string, sender, receiver []byte, amount uint64, token []byte) (*BridgeTransfer, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if _, ok := cb.chains[sourceChain]; !ok {
		return nil, fmt.Errorf("source chain not found")
	}
	if _, ok := cb.chains[destChain]; !ok {
		return nil, fmt.Errorf("dest chain not found")
	}

	cb.nextID++
	id := fmt.Sprintf("transfer-%d", cb.nextID)
	t := &BridgeTransfer{
		ID:          id,
		SourceChain: sourceChain,
		DestChain:   destChain,
		Sender:      sender,
		Receiver:    receiver,
		Amount:      amount,
		Token:       token,
		Status:      TransferStatusPending,
		Sigs:        make(map[string]bool),
	}
	cb.transfers[id] = t
	return t, nil
}

func (cb *ChainBridge) LockTokens(transferID string, txHash []byte) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	t, ok := cb.transfers[transferID]
	if !ok {
		return fmt.Errorf("transfer not found")
	}
	t.LockTxHash = txHash
	t.Status = TransferStatusLocked
	return nil
}

func (cb *ChainBridge) AddValidatorSignature(transferID, validator string) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	t, ok := cb.transfers[transferID]
	if !ok {
		return fmt.Errorf("transfer not found")
	}
	if !cb.validators[validator] {
		return fmt.Errorf("unknown validator: %s", validator)
	}
	t.Sigs[validator] = true
	if len(t.Sigs) >= cb.threshold {
		t.Status = TransferStatusCompleted
	}
	return nil
}

func (cb *ChainBridge) CompleteTransfer(transferID string, txHash []byte) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	t, ok := cb.transfers[transferID]
	if !ok {
		return fmt.Errorf("transfer not found")
	}
	if len(t.Sigs) < cb.threshold {
		return fmt.Errorf("insufficient signatures")
	}
	t.MintTxHash = txHash
	t.Status = TransferStatusCompleted
	return nil
}

func (cb *ChainBridge) RegisterChain(id, name, endpoint string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.chains[id] = &ChainInfo{
		ID:       id,
		Name:     name,
		Endpoint: endpoint,
		IsActive: true,
	}
}

func (cb *ChainBridge) RegisterValidator(validator string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.validators[validator] = true
}

func (cb *ChainBridge) GetTransfer(transferID string) (*BridgeTransfer, bool) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	t, ok := cb.transfers[transferID]
	return t, ok
}
