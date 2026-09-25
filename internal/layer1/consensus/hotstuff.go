package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
	"github.com/viri-chain/viri/internal/pkg/audit"
	"github.com/viri-chain/viri/internal/pkg/metrics"
)

type BlockProducer interface {
	CreateBlock(proposer []byte, height uint64) ([]byte, []byte, error)
	ValidateBlock(blockData []byte, blockHash []byte, height uint64) error
	CommitBlock(blockHash []byte, height uint64) error
	GetBlockHash(height uint64) ([]byte, error)
	GetBlockData(height uint64) ([]byte, error)
	GetChainHeight() uint64
	RotateKey() error
	Sign(data []byte) (*crypto.Signature, error)
	VerifySign(pubKey []byte, data []byte, sig *crypto.Signature) bool
	GetValidatorAddress() []byte
	GetValidatorPublicKey() []byte
}

type BroadcastFunc func(msg *ConsensusMessage)

func (hs *HotStuffEngine) logInfo(msg string) {
	if hs.logger != nil {
		hs.logger.Info(msg)
	}
}

func (hs *HotStuffEngine) logDebug(msg string) {
	if hs.logger != nil {
		hs.logger.Debug(msg)
	}
}

func (hs *HotStuffEngine) logWarn(msg string) {
	if hs.logger != nil {
		hs.logger.Warn(msg)
	}
}

func (hs *HotStuffEngine) logError(msg string) {
	if hs.logger != nil {
		hs.logger.Error(msg)
	}
}

type HotStuffEngine struct {
	mu            sync.Mutex
	config        *ConsensusConfig
	chainID       uint64
	state         *ConsensusState
	validatorSet  *ValidatorSet
	blockProducer BlockProducer
	staking       *StakingModule
	logger        *logging.Logger
	broadcastFn   BroadcastFunc

	doubleSign    *DoubleSignDetector
	finality      *FinalityGadget
	stateStore    *ConsensusStateStore
	liveness      *LivenessTracker
	stateSyncer   *StateSyncer

	viewVotes     map[string]*ViewVoteRegistry
	timeoutTimer  *time.Timer
	timeoutView   uint64
	viewTimeout   time.Duration
	timeouts      map[uint64]map[string]bool
	timeoutSignatures map[uint64]map[string]*crypto.Signature
	timeoutHighQCs    map[uint64]map[string]*QC

	messageCh     chan *ConsensusMessage
	proposeCh     chan struct{}
	futureMsgCh   chan *ConsensusMessage
	syncMsgCh     chan *ConsensusMessage
	applyCh       chan *blockApplyRequest
	done          chan struct{}
	running       atomic.Bool
	curHeight     atomic.Uint64
	curView       atomic.Uint64

	pendingBlocks map[uint64]*pendingBlock

	votesOrder    []string
	timeoutsOrder []uint64

	epochStartHeight uint64
	rewardPool       *big.Int // C-03: Use *big.Int to prevent overflow

	metrics   *metrics.MetricsCollector
	auditLog  audit.AuditLoggerInterface
	wg        sync.WaitGroup

	rateLimiter *messageRateLimiter

	BlockRewardFn func(height uint64, proposer []byte, amount *big.Int)
}

type blockApplyRequest struct {
	height uint64
	hash   []byte
	prevHash []byte
	proposer []byte
	resultCh chan error
}

type messageRateLimiter struct {
	mu         sync.Mutex
	counters   map[string]*rateCounter
	limit      int
	window     time.Duration
	lastReset  time.Time
}

type rateCounter struct {
	count    int
	lastSeen time.Time
}

func newMessageRateLimiter(limit int, window time.Duration) *messageRateLimiter {
	return &messageRateLimiter{
		counters:  make(map[string]*rateCounter),
		limit:     limit,
		window:    window,
		lastReset: time.Now(),
	}
}

func (rl *messageRateLimiter) Allow(addr string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.lastReset) > rl.window {
		rl.counters = make(map[string]*rateCounter)
		rl.lastReset = now
	}

	counter, exists := rl.counters[addr]
	if !exists {
		rl.counters[addr] = &rateCounter{count: 1, lastSeen: now}
		return true
	}

	counter.lastSeen = now
	counter.count++
	return counter.count <= rl.limit
}

type pendingBlock struct {
	hash     []byte
	proposer []byte
	payload  []byte
}

type ViewVoteRegistry struct {
	Votes              map[Phase]map[string]map[string]bool
	VoteCache          map[Phase]map[string][]*Vote
	VoteSignatureCache map[Phase]map[string]map[string]*crypto.Signature
	MyVotes            map[Phase][]byte
}

func newViewVoteRegistry() *ViewVoteRegistry {
	return &ViewVoteRegistry{
		Votes:              make(map[Phase]map[string]map[string]bool),
		VoteCache:          make(map[Phase]map[string][]*Vote),
		VoteSignatureCache: make(map[Phase]map[string]map[string]*crypto.Signature),
		MyVotes:            make(map[Phase][]byte),
	}
}

func (hs *HotStuffEngine) SetMetrics(mc *metrics.MetricsCollector) {
	hs.metrics = mc
}

func NewHotStuffEngine(config *ConsensusConfig, vs *ValidatorSet, bp BlockProducer, staking *StakingModule, log *logging.Logger, auditLog audit.AuditLoggerInterface) *HotStuffEngine {
	return newHotStuffEngineWithChainID(config, vs, bp, staking, log, auditLog, 0)
}

func newHotStuffEngineWithChainID(config *ConsensusConfig, vs *ValidatorSet, bp BlockProducer, staking *StakingModule, log *logging.Logger, auditLog audit.AuditLoggerInterface, chainID uint64) *HotStuffEngine {
	if config == nil {
		config = DefaultConsensusConfig()
	}

	hs := &HotStuffEngine{
		config:       config,
		chainID:      chainID,
		validatorSet: vs,
		blockProducer: bp,
		staking:      staking,
		logger:       log,
		auditLog:     auditLog,
		state: &ConsensusState{
			Phase: PhaseIdle,
		},
		viewVotes:     make(map[string]*ViewVoteRegistry),
		timeouts:      make(map[uint64]map[string]bool),
		timeoutSignatures: make(map[uint64]map[string]*crypto.Signature),
		timeoutHighQCs:    make(map[uint64]map[string]*QC),
		viewTimeout:   config.ViewTimeout,
		messageCh:     make(chan *ConsensusMessage, 1000),
		proposeCh:     make(chan struct{}, 1),
		futureMsgCh:   make(chan *ConsensusMessage, 500),
		syncMsgCh:     make(chan *ConsensusMessage, 2000),
		applyCh:       make(chan *blockApplyRequest, 500),
		done:          make(chan struct{}),
		doubleSign:    NewDoubleSignDetector(),
		finality:      NewFinalityGadget(config.EpochLength, 0.67),
		stateStore:    NewConsensusStateStore(),
		liveness:      NewLivenessTracker(config.DowntimeThreshold),
		epochStartHeight: 1,
		pendingBlocks:    make(map[uint64]*pendingBlock),
		votesOrder:       make([]string, 0, 1024),
		timeoutsOrder:    make([]uint64, 0, 1024),
		rateLimiter:      newMessageRateLimiter(config.MessageRateLimit, config.MessageRateWindow),
	}

	hs.stateSyncer = NewStateSyncer(
		hs.defaultBlockRequester,
		hs.defaultBlockApplier,
		hs.Height,
		log,
	)

	return hs
}

func (hs *HotStuffEngine) getOrCreateVoteRegistry(height uint64, view uint64) *ViewVoteRegistry {
	key := fmt.Sprintf("%d-%d", height, view)
	reg, exists := hs.viewVotes[key]
	if !exists {
		reg = newViewVoteRegistry()
		hs.viewVotes[key] = reg
		hs.votesOrder = append(hs.votesOrder, key)
	}
	return reg
}

func (hs *HotStuffEngine) Start(height uint64) error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if hs.running.Load() {
		return fmt.Errorf("consensus already running")
	}

	saved, err := hs.stateStore.Load()
	if err == nil && saved.Height > 0 {
		hs.restoreState(saved)
	} else {
		hs.state.Height = height
		hs.state.View = 0
		hs.state.Phase = PhasePrepare
		hs.state.StartTime = time.Now()
		hs.state.ProtocolVersion = hs.config.ProtocolVersion
	}

	hs.curHeight.Store(hs.state.Height)
	hs.curView.Store(hs.state.View)

	if hs.state.ProtocolVersion > 0 && hs.config.ProtocolVersion != 0 && hs.state.ProtocolVersion != hs.config.ProtocolVersion {
		return fmt.Errorf("protocol version mismatch: state=%d config=%d", hs.state.ProtocolVersion, hs.config.ProtocolVersion)
	}

	if hs.config.ProtocolVersion != 0 && hs.state.ProtocolVersion == 0 {
		hs.state.ProtocolVersion = hs.config.ProtocolVersion
	}

	proposer, err := hs.validatorSet.GetProposer(hs.state.Height)
	leaderStr := "unknown"
	if err == nil && proposer != nil {
		leaderStr = fmt.Sprintf("%x", proposer.Address)
	}
	hs.logInfo(fmt.Sprintf("Consensus started height=%d validators=%d leader=%s me=%x",
		hs.state.Height, hs.validatorSet.Size(),
		leaderStr, hs.blockProducer.GetValidatorAddress()))

	if hs.metrics != nil {
		hs.metrics.SetConsensusHeight(hs.state.Height)
		hs.metrics.SetConsensusView(hs.state.View)
		hs.metrics.SetConsensusPhase(float64(hs.state.Phase))
		hs.metrics.SetConsensusValidators(hs.validatorSet.Size())
	}

	hs.running.Store(true)
	hs.wg.Add(5)
	go hs.loop()
	go hs.futureLoop()
	go hs.syncLoop()
	go hs.livenessLoop()
	go hs.applyLoop()

	if proposer != nil && bytes.Equal(hs.blockProducer.GetValidatorAddress(), proposer.Address) {
		hs.triggerPropose()
	}

	hs.startTimeout()

	return nil
}

func (hs *HotStuffEngine) Stop() {
	hs.mu.Lock()
	if !hs.running.Load() {
		hs.mu.Unlock()
		return
	}

	hs.running.Store(false)
	if hs.timeoutTimer != nil {
		hs.timeoutTimer.Stop()
	}
	select {
	case <-hs.done:
	default:
		close(hs.done)
	}
	hs.mu.Unlock()

	hs.wg.Wait()
}

// IsRunning returns whether the consensus engine is active.
func (hs *HotStuffEngine) IsRunning() bool {
	return hs.running.Load()
}

func (hs *HotStuffEngine) loop() {
	defer hs.wg.Done()
	defer hs.recoverPanic("loop")
	for {
		select {
		case msg := <-hs.messageCh:
			hs.handleMessage(msg)
		case <-hs.proposeCh:
			hs.doPropose()
		case <-hs.done:
			return
		}
	}
}

func (hs *HotStuffEngine) triggerPropose() {
	select {
	case hs.proposeCh <- struct{}{}:
	default:
	}
}

func (hs *HotStuffEngine) applyLoop() {
	defer hs.wg.Done()
	defer hs.recoverPanic("applyLoop")
	for {
		select {
		case req := <-hs.applyCh:
			commitErr := hs.blockProducer.CommitBlock(req.hash, req.height)
			if commitErr == nil {
				hs.mu.Lock()
				if req.height >= hs.state.Height {
					hs.state.Height = req.height + 1
					hs.state.View = 0
					hs.state.Phase = PhasePrepare
					hs.curHeight.Store(hs.state.Height)
					hs.curView.Store(0)
				}
				hs.mu.Unlock()
			}
			if req.resultCh != nil {
				req.resultCh <- commitErr
			}
		case <-hs.done:
			return
		}
	}
}

func (hs *HotStuffEngine) futureLoop() {
	defer hs.wg.Done()
	defer hs.recoverPanic("futureLoop")
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case msg := <-hs.futureMsgCh:
			hs.mu.Lock()
			if msg.Height == hs.state.Height {
				hs.handleMessageLocked(msg)
			} else {
				select {
				case hs.futureMsgCh <- msg:
				default:
				}
			}
			hs.mu.Unlock()
		case <-ticker.C:
			continue
		case <-hs.done:
			return
		}
	}
}

func (hs *HotStuffEngine) livenessLoop() {
	defer hs.wg.Done()
	defer hs.recoverPanic("livenessLoop")
	ticker := time.NewTicker(hs.config.BlockTime)
	defer ticker.Stop()

	unjailTicker := time.NewTicker(30 * time.Second)
	defer unjailTicker.Stop()

	for {
		select {
		case <-ticker.C:
			currentHeight := hs.curHeight.Load()

			hs.liveness.UpdateHeight(currentHeight)
			offline := hs.liveness.GetOfflineValidators()

			if len(offline) > 0 {
				hs.logInfo(fmt.Sprintf("Offline validators detected: count=%d", len(offline)))
				for _, addr := range offline {
					missed := hs.liveness.GetMissedString(addr)
					if missed > hs.config.DowntimeThreshold*2 {
						hs.handleDowntimeSlash([]byte(addr), missed)
					}
				}
			}

		case <-unjailTicker.C:
			hs.processAutoUnjail()

		case <-hs.done:
			return
		}
	}
}

func (hs *HotStuffEngine) recoverPanic(goroutine string) {
	if r := recover(); r != nil {
		hs.logInfo(fmt.Sprintf("CRITICAL: consensus %s panicked: %v", goroutine, r))
		hs.logInfo("Consensus goroutine recovered — node may be in inconsistent state, restart recommended")
		// C-02: Mark engine as stopped and close done channel so other goroutines exit.
		hs.running.Store(false)
		select {
		case <-hs.done:
		default:
			close(hs.done)
		}
	}
}

func (hs *HotStuffEngine) Propose() error {
	return hs.doPropose()
}

func (hs *HotStuffEngine) doPropose() error {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return hs.proposeLocked()
}

func (hs *HotStuffEngine) proposeLocked() error {

	if hs.state.Phase != PhasePrepare {
		return nil
	}

	if _, exists := hs.pendingBlocks[hs.state.Height]; exists {
		hs.logDebug(fmt.Sprintf("Already has pending block for height=%d, skipping proposal", hs.state.Height))
		return nil
	}

	activeValidators := hs.staking.GetActiveValidators()
	if len(activeValidators) < hs.config.MinValidators {
		hs.logWarn(fmt.Sprintf("Skipping proposal: insufficient active validators (%d < %d)", len(activeValidators), hs.config.MinValidators))
		return nil
	}

	proposer, err := hs.validatorSet.GetProposerForView(hs.state.Height, hs.state.View)
	if err != nil {
		hs.logWarn(fmt.Sprintf("Failed to get proposer height=%d error=%v", hs.state.Height, err))
		return err
	}

	if !bytes.Equal(hs.blockProducer.GetValidatorAddress(), proposer.Address) {
		hs.logDebug(fmt.Sprintf("Not proposer height=%d me=%x proposer=%x", hs.state.Height, hs.blockProducer.GetValidatorAddress(), proposer.Address))
		return nil
	}

	hs.logInfo(fmt.Sprintf("Proposing block height=%d view=%d", hs.state.Height, hs.state.View))

	if hs.auditLog != nil {
		hs.auditLog.LogProposal(hs.state.Height, hs.state.View, fmt.Sprintf("%x", hs.blockProducer.GetValidatorAddress()), "")
	}

	if hs.metrics != nil {
		hs.metrics.IncConsensusProposals()
	}

	blockData, blockHash, err := hs.blockProducer.CreateBlock(proposer.Address, hs.state.Height)
	hs.logInfo(fmt.Sprintf("CreateBlock returned: height=%d err=%v", hs.state.Height, err))
	if err != nil {
		return err
	}

	hs.pendingBlocks[hs.state.Height] = &pendingBlock{
		hash:     blockHash,
		proposer: proposer.Address,
		payload:  blockData,
	}

	sig, err := hs.blockProducer.Sign(hs.createProposalData(blockHash))
	if err != nil {
		return err
	}

	doubleSign := hs.doubleSign.CheckProposal(proposer.Address, hs.state.Height, hs.state.View, blockHash)
	if doubleSign != nil {
		hs.handleDoubleSign(doubleSign)
		return fmt.Errorf("double sign detected")
	}

	justifyQC := hs.state.LockedQC
	if justifyQC != nil && len(justifyQC.Signatures) == 0 {
		justifyQC = nil
	}

	proposal := &Proposal{
		Height:    hs.state.Height,
		View:      hs.state.View,
		BlockHash: blockHash,
		Proposer:  proposer.Address,
		JustifyQC: justifyQC,
		Payload:   blockData,
	}

	msg := &ConsensusMessage{
		Type:      MsgProposal,
		Height:    hs.state.Height,
		View:      hs.state.View,
		BlockHash: blockHash,
		Validator: proposer.Address,
		Signature: sig,
		JustifyQC: justifyQC,
		Payload:   proposal.Payload,
		Timestamp: time.Now(),
	}

	hs.doBroadcast(msg)

	hs.selfVote(blockHash, hs.state.Height, hs.state.View)

	if hs.state.Phase != PhaseDecide {
		hs.state.Phase = PhasePrepare
	}

	if hs.validatorSet.Size() > 1 {
		hs.startTimeout()
	}

	return nil
}

func (hs *HotStuffEngine) selfVote(blockHash []byte, height uint64, view uint64) {
	if hs.metrics != nil {
		hs.metrics.IncConsensusVotes()
	}

	blockHashStr := hex.EncodeToString(blockHash)
	reg := hs.getOrCreateVoteRegistry(height, view)

	if reg.Votes[PhasePrepare] == nil {
		reg.Votes[PhasePrepare] = make(map[string]map[string]bool)
	}
	if reg.Votes[PhasePrepare][blockHashStr] == nil {
		reg.Votes[PhasePrepare][blockHashStr] = make(map[string]bool)
	}

	validatorAddr := hex.EncodeToString(hs.blockProducer.GetValidatorAddress())
	reg.Votes[PhasePrepare][blockHashStr][validatorAddr] = true
	reg.MyVotes[PhasePrepare] = blockHash
	hs.liveness.RecordActivity(hs.blockProducer.GetValidatorAddress(), height)

	sigData := hs.createVoteData(height, view, PhasePrepare, blockHash)
	selfSig, err := hs.blockProducer.Sign(sigData)
	if err == nil {
		hs.storeVoteSignature(height, view, PhasePrepare, blockHashStr, validatorAddr, selfSig)
		// Broadcast the proposer's prepare vote so other validators can reach quorum
		vote := &ConsensusMessage{
			Type:      MsgVotePrepare,
			Height:    height,
			View:      view,
			BlockHash: blockHash,
			Validator: hs.blockProducer.GetValidatorAddress(),
			Signature: selfSig,
			Timestamp: time.Now(),
		}
		hs.doBroadcast(vote)
	}

	if hs.validatorSet.HasSuperMajority(reg.Votes[PhasePrepare][blockHashStr]) {
		qc := hs.createQC(height, view, PhasePrepare, blockHash)

		hs.state.PreparedQC = qc
		if hs.state.LockedQC == nil || qc.View >= hs.state.LockedQC.View {
			hs.state.LockedQC = qc
		}
		hs.state.Phase = PhasePreCommit

		if reg.Votes[PhasePreCommit] == nil {
			reg.Votes[PhasePreCommit] = make(map[string]map[string]bool)
		}
		if reg.Votes[PhasePreCommit][blockHashStr] == nil {
			reg.Votes[PhasePreCommit][blockHashStr] = make(map[string]bool)
		}
		reg.Votes[PhasePreCommit][blockHashStr][validatorAddr] = true
		reg.MyVotes[PhasePreCommit] = blockHash

		pcSigData := hs.createVoteData(height, view, PhasePreCommit, blockHash)
		pcSig, err := hs.blockProducer.Sign(pcSigData)
		if err == nil {
			hs.storeVoteSignature(height, view, PhasePreCommit, blockHashStr, validatorAddr, pcSig)
		}

		if hs.validatorSet.HasSuperMajority(reg.Votes[PhasePreCommit][blockHashStr]) {
			hs.state.Phase = PhaseCommit

			if reg.Votes[PhaseCommit] == nil {
				reg.Votes[PhaseCommit] = make(map[string]map[string]bool)
			}
			if reg.Votes[PhaseCommit][blockHashStr] == nil {
				reg.Votes[PhaseCommit][blockHashStr] = make(map[string]bool)
			}
			reg.Votes[PhaseCommit][blockHashStr][validatorAddr] = true
			reg.MyVotes[PhaseCommit] = blockHash

			cSigData := hs.createVoteData(height, view, PhaseCommit, blockHash)
			cSig, err := hs.blockProducer.Sign(cSigData)
			if err == nil {
				hs.storeVoteSignature(height, view, PhaseCommit, blockHashStr, validatorAddr, cSig)
			}

			if hs.validatorSet.HasSuperMajority(reg.Votes[PhaseCommit][blockHashStr]) {
				hs.decide(blockHash, height)
				return
			}
		}
	}
}

func (hs *HotStuffEngine) HandleMessage(msg *ConsensusMessage) {
	if msg != nil {
		hs.logDebug(fmt.Sprintf("HandleMessage type=%v height=%d view=%d validator=%s", msg.Type, msg.Height, msg.View, safeValStr(msg.Validator)))
	}
	if !hs.running.Load() {
		select {
		case hs.messageCh <- msg:
		default:
		}
		return
	}

	if msg.Validator != nil {
		addr := hex.EncodeToString(msg.Validator)
		if !hs.rateLimiter.Allow(addr) {
			if hs.metrics != nil {
				hs.metrics.IncRateLimitedMessages()
			}
			return
		}
	}

	if msg.Type == MsgBlockResponse {
		hs.mu.Lock()
		stateSyncer := hs.stateSyncer
		hs.mu.Unlock()
		isSyncing := stateSyncer != nil && stateSyncer.IsSyncing()
		if isSyncing && len(msg.Payload) > 0 {
			blockData := prependHeightToPayload(msg.Payload, msg.Height)
			if err := stateSyncer.ReceiveBlock(blockData); err != nil {
				hs.logWarn(fmt.Sprintf("Sync block apply failed height=%d error=%v", msg.Height, err))
			}
		}
		return
	}

	if msg.Type == MsgBlockRequest {
		hs.mu.Lock()
		payload := make([]byte, len(msg.Payload))
		copy(payload, msg.Payload)
		hs.mu.Unlock()
		hs.handleBlockRequestData(payload)
		return
	}

	height := hs.curHeight.Load()

	if msg.Height > height+1 {
		// stateSyncer is set-once in constructor, safe to read without lock
		if syncer := hs.stateSyncer; syncer != nil && !syncer.IsSyncing() {
			syncer.StartSync(msg.Height)
		}
		select {
		case hs.syncMsgCh <- msg:
		default:
		}
		select {
		case hs.futureMsgCh <- msg:
		default:
		}
		return
	}

	select {
	case hs.messageCh <- msg:
	default:
	}
}

func (hs *HotStuffEngine) handleMessage(msg *ConsensusMessage) {
	if msg != nil {
		hs.logDebug(fmt.Sprintf("handleMessage type=%v height=%d view=%d validator=%s", msg.Type, msg.Height, msg.View, safeValStr(msg.Validator)))
	}
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.handleMessageLocked(msg)
}

func (hs *HotStuffEngine) handleMessageLocked(msg *ConsensusMessage) {
	if msg != nil {
		hs.logDebug(fmt.Sprintf("handleMessageLocked type=%v height=%d view=%d validator=%s (state: height=%d view=%d phase=%v)", msg.Type, msg.Height, msg.View, safeValStr(msg.Validator), hs.state.Height, hs.state.View, hs.state.Phase))
	}
	switch msg.Type {
	case MsgBlockRequest:
		// MsgBlockRequest is handled in HandleMessage without the mutex
		// to avoid deadlock with broadcastFn. This path should not be reached.
		return
	case MsgBlockResponse:
		// MsgBlockResponse is handled directly in HandleMessage (without the mutex)
		// to avoid deadlock with applyLoop. This path should not be reached.
		return
	}

	if msg.Height < hs.state.Height {
		return
	}

	if msg.Height > hs.state.Height {
		if hs.stateSyncer != nil && !hs.stateSyncer.IsSyncing() {
			hs.stateSyncer.StartSync(msg.Height)
		}
		select {
		case hs.syncMsgCh <- msg:
		default:
		}
		select {
		case hs.futureMsgCh <- msg:
		default:
		}
		return
	}

	if msg.View < hs.state.View {
		return
	}

	switch msg.Type {
	case MsgTimeout:
		hs.handleTimeout(msg)
	case MsgProposal, MsgVotePrepare, MsgVotePreCommit, MsgVoteCommit:
		if msg.View > hs.state.View {
			hs.updateView(msg.View)
		}
		switch msg.Type {
		case MsgProposal:
			hs.handleProposal(msg)
		case MsgVotePrepare:
			hs.handleVote(msg, PhasePrepare)
		case MsgVotePreCommit:
			hs.handleVote(msg, PhasePreCommit)
		case MsgVoteCommit:
			hs.handleVote(msg, PhaseCommit)
		}
	case MsgNewView:
		if msg.View > hs.state.View {
			hs.updateView(msg.View)
		}
		hs.handleNewView(msg)
	}
}

func (hs *HotStuffEngine) handleProposal(msg *ConsensusMessage) {
	if hs.state.Phase != PhasePrepare {
		hs.logDebug(fmt.Sprintf("handleProposal rejected: wrong phase %v", hs.state.Phase))
		return
	}

	if hs.validatorSet == nil {
		return
	}

	proposer, err := hs.validatorSet.GetProposerForView(hs.state.Height, hs.state.View)
	if err != nil {
		hs.logWarn(fmt.Sprintf("handleProposal: no proposer height=%d", hs.state.Height))
		return
	}

	if !bytes.Equal(msg.Validator, proposer.Address) {
		hs.logDebug(fmt.Sprintf("handleProposal: wrong proposer msg=%x expected=%x", msg.Validator[:8], proposer.Address[:8]))
		return
	}

	if msg.Signature != nil && hs.blockProducer != nil {
		if !hs.blockProducer.VerifySign(proposer.PublicKey, hs.createProposalData(msg.BlockHash), msg.Signature) {
			hs.logWarn(fmt.Sprintf("Invalid proposal signature height=%d view=%d", msg.Height, msg.View))
			return
		}
	}

	doubleSign := hs.doubleSign.CheckProposal(msg.Validator, msg.Height, msg.View, msg.BlockHash)
	if doubleSign != nil {
		hs.handleDoubleSign(doubleSign)
		return
	}

	if msg.JustifyQC != nil && !hs.isValidQC(msg.JustifyQC) {
		hs.logDebug("handleProposal: invalid JustifyQC")
		return
	}

	if msg.JustifyQC != nil && (hs.state.LockedQC == nil || msg.JustifyQC.View > hs.state.LockedQC.View) {
		hs.state.LockedQC = msg.JustifyQC
	}

	validationErr := hs.blockProducer.ValidateBlock(msg.Payload, msg.BlockHash, msg.Height)
	if validationErr != nil {
		hs.logInfo(fmt.Sprintf("handleProposal: block validation failed: %v", validationErr))
		return
	}

	if _, exists := hs.pendingBlocks[msg.Height]; !exists {
		hs.pendingBlocks[msg.Height] = &pendingBlock{
			hash:     msg.BlockHash,
			proposer: msg.Validator,
			payload:  msg.Payload,
		}
	}

	if hs.auditLog != nil {
		hs.auditLog.LogProposal(msg.Height, msg.View, fmt.Sprintf("%x", msg.Validator), fmt.Sprintf("%x", msg.BlockHash))
	}

	reg := hs.getOrCreateVoteRegistry(msg.Height, msg.View)
	if existing, voted := reg.MyVotes[PhasePrepare]; voted {
		if !bytes.Equal(existing, msg.BlockHash) {
			hs.logDebug(fmt.Sprintf("handleProposal: already voted for different block at height=%d view=%d, rejecting", msg.Height, msg.View))
			return
		}
		return
	}

	sigData := hs.createVoteData(msg.Height, msg.View, PhasePrepare, msg.BlockHash)
	sig, err := hs.blockProducer.Sign(sigData)
	if err != nil {
		return
	}

	vote := &ConsensusMessage{
		Type:      MsgVotePrepare,
		Height:    msg.Height,
		View:      msg.View,
		BlockHash: msg.BlockHash,
		Validator: hs.blockProducer.GetValidatorAddress(),
		Signature: sig,
		Timestamp: time.Now(),
	}

	hs.doBroadcast(vote)

	blockHashStr := hex.EncodeToString(msg.BlockHash)
	if reg.Votes[PhasePrepare] == nil {
		reg.Votes[PhasePrepare] = make(map[string]map[string]bool)
	}
	if reg.Votes[PhasePrepare][blockHashStr] == nil {
		reg.Votes[PhasePrepare][blockHashStr] = make(map[string]bool)
	}
	validatorAddr := hex.EncodeToString(hs.blockProducer.GetValidatorAddress())
	reg.Votes[PhasePrepare][blockHashStr][validatorAddr] = true
	reg.MyVotes[PhasePrepare] = msg.BlockHash

	if sig != nil {
		hs.storeVoteSignature(msg.Height, msg.View, PhasePrepare, blockHashStr, validatorAddr, sig)
	}

	hs.liveness.RecordActivity(hs.blockProducer.GetValidatorAddress(), msg.Height)

	if hs.validatorSet.HasSuperMajority(reg.Votes[PhasePrepare][blockHashStr]) {
		qc := hs.createQC(msg.Height, msg.View, PhasePrepare, msg.BlockHash)
		hs.state.PreparedQC = qc
		if hs.state.LockedQC == nil || qc.View >= hs.state.LockedQC.View {
			hs.state.LockedQC = qc
		}
	}

	hs.startTimeout()

	hs.state.Phase = PhasePreCommit
}

func (hs *HotStuffEngine) handleVote(msg *ConsensusMessage, phase Phase) {
	if hs.metrics != nil {
		hs.metrics.IncConsensusVotes()
	}

	if msg.Signature != nil {
		v, exists := hs.validatorSet.GetValidator(msg.Validator)
		if !exists {
			return
		}

		voteData := hs.createVoteData(msg.Height, msg.View, phase, msg.BlockHash)
		if !hs.blockProducer.VerifySign(v.PublicKey, voteData, msg.Signature) {
			hs.logWarn(fmt.Sprintf("Invalid vote signature validator=%x height=%d phase=%s", msg.Validator, msg.Height, phase))
			return
		}
	}

	doubleSign := hs.doubleSign.CheckVote(msg.Validator, msg.Height, msg.View, phase, msg.BlockHash)
	if doubleSign != nil {
		hs.handleDoubleSign(doubleSign)
		return
	}

	blockHashStr := hex.EncodeToString(msg.BlockHash)
	reg := hs.getOrCreateVoteRegistry(msg.Height, msg.View)

	if reg.Votes[phase] == nil {
		reg.Votes[phase] = make(map[string]map[string]bool)
	}
	if reg.Votes[phase][blockHashStr] == nil {
		reg.Votes[phase][blockHashStr] = make(map[string]bool)
	}

	validatorAddr := hex.EncodeToString(msg.Validator)
	reg.Votes[phase][blockHashStr][validatorAddr] = true

	if msg.Signature != nil {
		hs.storeVoteSignature(msg.Height, msg.View, phase, blockHashStr, validatorAddr, msg.Signature)
	}

	hs.liveness.RecordActivity(msg.Validator, msg.Height)

	if hs.auditLog != nil {
		hs.auditLog.LogVote(msg.Height, msg.View, phase.String(), fmt.Sprintf("%x", msg.Validator), fmt.Sprintf("%x", msg.BlockHash))
	}

	if hs.validatorSet.HasSuperMajority(reg.Votes[phase][blockHashStr]) {
		qc := hs.createQC(msg.Height, msg.View, phase, msg.BlockHash)

		switch phase {
		case PhasePrepare:
			hs.state.PreparedQC = qc
			if hs.state.LockedQC == nil || hs.state.PreparedQC.View >= hs.state.LockedQC.View {
				hs.state.LockedQC = qc
			}
			hs.advancePhase(PhasePreCommit)

		case PhasePreCommit:
			hs.advancePhase(PhaseCommit)

		case PhaseCommit:
			hs.decide(msg.BlockHash, msg.Height)
		}
	} else if phase == PhasePrepare && msg.View == hs.state.View {
		hs.startTimeout()
	}

	hs.trimVotes()
}

func (hs *HotStuffEngine) trimVotes() {
	// C-05: Prune vote registries for heights < current - 10.
	const keepHeights = 10
	currentHeight := hs.state.Height

	const maxVoteEntries = 2048
	for len(hs.votesOrder) > maxVoteEntries {
		oldest := hs.votesOrder[0]
		hs.votesOrder = hs.votesOrder[1:]
		delete(hs.viewVotes, oldest)
	}

	// Additionally prune by height
	if currentHeight > keepHeights {
		pruneBelow := currentHeight - keepHeights
		newOrder := make([]string, 0, len(hs.votesOrder))
		for _, key := range hs.votesOrder {
			var h, v uint64
			if _, err := fmt.Sscanf(key, "%d-%d", &h, &v); err == nil && h < pruneBelow {
				delete(hs.viewVotes, key)
				continue
			}
			newOrder = append(newOrder, key)
		}
		hs.votesOrder = newOrder
	}
}

func (hs *HotStuffEngine) handleTimeout(msg *ConsensusMessage) {
	if msg.View < hs.state.View {
		return
	}

	if msg.Signature == nil {
		return
	}

	v, exists := hs.validatorSet.GetValidator(msg.Validator)
	if !exists {
		return
	}

	timeoutData := hs.createTimeoutData(msg.Height, msg.View)
	if !hs.blockProducer.VerifySign(v.PublicKey, timeoutData, msg.Signature) {
		hs.logWarn(fmt.Sprintf("Invalid timeout signature validator=%x view=%d", msg.Validator, msg.View))
		return
	}

	if msg.JustifyQC != nil && !hs.isValidQC(msg.JustifyQC) {
		hs.logWarn(fmt.Sprintf("Invalid highQC in timeout from validator=%x view=%d", msg.Validator, msg.View))
		return
	}

	if hs.auditLog != nil {
		hs.auditLog.LogTimeout(msg.Height, msg.View, 1)
	}

	if hs.timeouts[msg.View] == nil {
		hs.timeouts[msg.View] = make(map[string]bool)
		hs.timeoutsOrder = append(hs.timeoutsOrder, msg.View)
	}
	if hs.timeoutSignatures[msg.View] == nil {
		hs.timeoutSignatures[msg.View] = make(map[string]*crypto.Signature)
	}
	if hs.timeoutHighQCs[msg.View] == nil {
		hs.timeoutHighQCs[msg.View] = make(map[string]*QC)
	}

	validatorAddr := hex.EncodeToString(msg.Validator)
	if hs.timeouts[msg.View][validatorAddr] {
		return
	}

	hs.timeouts[msg.View][validatorAddr] = true
	hs.timeoutSignatures[msg.View][validatorAddr] = msg.Signature
	hs.timeoutHighQCs[msg.View][validatorAddr] = msg.JustifyQC
	hs.liveness.RecordActivity(msg.Validator, msg.Height)

	if hs.validatorSet.HasSuperMajority(hs.timeouts[msg.View]) {
		hs.createNewViewAndBroadcast(msg.View)
	}

	hs.trimTimeouts()

	hs.timeoutView = msg.View
	if hs.timeoutView > hs.state.View {
		hs.updateView(hs.timeoutView)

		proposer, err := hs.validatorSet.GetProposerForView(hs.state.Height, hs.state.View)
		if err == nil && bytes.Equal(hs.blockProducer.GetValidatorAddress(), proposer.Address) {
			if hs.state.Phase == PhasePrepare {
				hs.triggerPropose()
			}
		}
	}
}

func (hs *HotStuffEngine) trimTimeouts() {
	const maxTimeoutEntries = 2048
	for len(hs.timeoutsOrder) > maxTimeoutEntries {
		oldest := hs.timeoutsOrder[0]
		hs.timeoutsOrder = hs.timeoutsOrder[1:]
		delete(hs.timeouts, oldest)
		delete(hs.timeoutSignatures, oldest)
		delete(hs.timeoutHighQCs, oldest)
	}
}

func (hs *HotStuffEngine) createNewViewAndBroadcast(newView uint64) {
	if newView <= hs.state.View {
		return
	}

	signatures := make(map[string]crypto.Signature)
	var bestHighQC *QC

	for addr, sig := range hs.timeoutSignatures[newView] {
		signatures[addr] = *sig
		if qc := hs.timeoutHighQCs[newView][addr]; qc != nil {
			if bestHighQC == nil || qc.View > bestHighQC.View {
				bestHighQC = qc
			}
		}
	}

	tc := &TimeoutCert{
		Height:     hs.state.Height,
		View:       newView,
		Timeouts:   hs.timeouts[newView],
		Signatures: signatures,
		TotalStake: hs.validatorSet.TotalStake(),
		HighQC:     bestHighQC,
	}

	tcPayload := hs.encodeTimeoutCert(tc)

	newViewMsg := &ConsensusMessage{
		Type:      MsgNewView,
		Height:    hs.state.Height,
		View:      newView,
		Validator: hs.blockProducer.GetValidatorAddress(),
		JustifyQC: bestHighQC,
		Payload:   tcPayload,
		Timestamp: time.Now(),
	}

	sig, err := hs.blockProducer.Sign(tcPayload)
	if err == nil {
		newViewMsg.Signature = sig
	}

	hs.doBroadcast(newViewMsg)

	delete(hs.timeouts, newView)
	delete(hs.timeoutSignatures, newView)
	delete(hs.timeoutHighQCs, newView)
}

func (hs *HotStuffEngine) encodeTimeoutPayload(view uint64, sig *crypto.Signature, highQC *QC) []byte {
	if sig == nil {
		return nil
	}
	data := make([]byte, 16)
	binary.BigEndian.PutUint64(data[0:8], hs.state.Height)
	binary.BigEndian.PutUint64(data[8:16], view)
	sigBytes := sig.Bytes()
	data = append(data, sigBytes...)
	if highQC != nil {
		qcBytes, _ := highQC.Encode()
		data = append(data, qcBytes...)
	}
	return data
}

func (hs *HotStuffEngine) validateTimeoutCert(tc *TimeoutCert) bool {
	if tc == nil {
		return false
	}

	if tc.Height != hs.state.Height {
		return false
	}

	if len(tc.Signatures) == 0 {
		return false
	}

	timeoutData := hs.createTimeoutData(tc.Height, tc.View)

	var signedStake uint64
	for addrStr, sig := range tc.Signatures {
		addrBytes, err := hexDecode(addrStr)
		if err != nil {
			return false
		}

		v, exists := hs.validatorSet.GetValidator(addrBytes)
		if !exists {
			return false
		}

		if !hs.blockProducer.VerifySign(v.PublicKey, timeoutData, &sig) {
			return false
		}

		signedStake += v.Stake
	}

	return signedStake*3 > hs.validatorSet.TotalStake()*2
}

func (hs *HotStuffEngine) encodeTimeoutCert(tc *TimeoutCert) []byte {
	data, _ := json.Marshal(tc)
	return data
}

func (hs *HotStuffEngine) handleNewView(msg *ConsensusMessage) {
	if msg.JustifyQC == nil && msg.Payload == nil {
		return
	}

	if msg.Signature == nil {
		return
	}

	var tc TimeoutCert
	if len(msg.Payload) > 0 {
		if err := json.Unmarshal(msg.Payload, &tc); err != nil {
			hs.logWarn(fmt.Sprintf("Failed to unmarshal TimeoutCert: %v", err))
			return
		}

		if tc.View != msg.View {
			hs.logWarn(fmt.Sprintf("TimeoutCert view mismatch: tc=%d msg=%d", tc.View, msg.View))
			return
		}

		if tc.Height != msg.Height {
			hs.logWarn(fmt.Sprintf("TimeoutCert height mismatch: tc=%d msg=%d", tc.Height, msg.Height))
			return
		}

		if !hs.validateTimeoutCert(&tc) {
			hs.logWarn(fmt.Sprintf("Invalid TimeoutCert for view=%d", msg.View))
			return
		}

		if tc.HighQC != nil && !hs.isValidQC(tc.HighQC) {
			hs.logWarn("Invalid HighQC in TimeoutCert")
			return
		}

		if tc.HighQC != nil && (hs.state.LockedQC == nil || tc.HighQC.View > hs.state.LockedQC.View) {
			hs.state.LockedQC = tc.HighQC
		}
	} else {
		if !hs.isValidQC(msg.JustifyQC) {
			return
		}
	}

	if msg.JustifyQC != nil && (hs.state.LockedQC == nil || msg.JustifyQC.View > hs.state.LockedQC.View) {
		hs.state.LockedQC = msg.JustifyQC
	}

	hs.updateView(msg.View)

	for view := range hs.timeouts {
		if view < msg.View {
			delete(hs.timeouts, view)
			delete(hs.timeoutSignatures, view)
			delete(hs.timeoutHighQCs, view)
		}
	}

	proposer, err := hs.validatorSet.GetProposerForView(hs.state.Height, hs.state.View)
	if err == nil && bytes.Equal(hs.blockProducer.GetValidatorAddress(), proposer.Address) {
		if hs.state.Phase == PhasePrepare {
			hs.triggerPropose()
		}
	}
}

func (hs *HotStuffEngine) advancePhase(phase Phase) {
	if hs.state.PreparedQC == nil {
		hs.logWarn(fmt.Sprintf("advancePhase skipped: PreparedQC is nil phase=%s height=%d view=%d", phase, hs.state.Height, hs.state.View))
		return
	}

	hs.state.Phase = phase
	hs.startTimeout()

	voteMsg := &ConsensusMessage{
		Height:    hs.state.Height,
		View:      hs.state.View,
		BlockHash: hs.state.PreparedQC.BlockHash,
		Validator: hs.blockProducer.GetValidatorAddress(),
		Timestamp: time.Now(),
	}

	switch phase {
	case PhasePreCommit:
		voteMsg.Type = MsgVotePreCommit
	case PhaseCommit:
		voteMsg.Type = MsgVoteCommit
	default:
		return
	}

	voteData := hs.createVoteData(hs.state.Height, hs.state.View, phase, hs.state.PreparedQC.BlockHash)
	sig, err := hs.blockProducer.Sign(voteData)
	if err == nil {
		voteMsg.Signature = sig
	}

	hs.doBroadcast(voteMsg)

	reg := hs.getOrCreateVoteRegistry(hs.state.Height, hs.state.View)
	blockHashStr := hex.EncodeToString(hs.state.PreparedQC.BlockHash)
	if reg.Votes[phase] == nil {
		reg.Votes[phase] = make(map[string]map[string]bool)
	}
	if reg.Votes[phase][blockHashStr] == nil {
		reg.Votes[phase][blockHashStr] = make(map[string]bool)
	}
	validatorAddr := hex.EncodeToString(hs.blockProducer.GetValidatorAddress())
	reg.Votes[phase][blockHashStr][validatorAddr] = true
	reg.MyVotes[phase] = hs.state.PreparedQC.BlockHash

	if sig != nil {
		hs.storeVoteSignature(hs.state.Height, hs.state.View, phase, blockHashStr, validatorAddr, sig)
	}
}

func (hs *HotStuffEngine) decide(blockHash []byte, height uint64) {
	if hs.state.LockedQC != nil && len(hs.state.LockedQC.BlockHash) > 0 && hs.state.LockedQC.Height == height {
		if !bytes.Equal(blockHash, hs.state.LockedQC.BlockHash) {
			hs.logError(fmt.Sprintf("Block hash mismatch: decided=%x locked=%x height=%d", blockHash, hs.state.LockedQC.BlockHash, height))
			return
		}
	}

	if pending := hs.pendingBlocks[height]; pending != nil {
		if !bytes.Equal(blockHash, pending.hash) {
			hs.logWarn(fmt.Sprintf("Block hash differs from pending block: decided=%x pending=%x height=%d", blockHash, pending.hash, height))
		}
	}

	hs.state.Phase = PhaseDecide
	hs.state.DecidedHash = blockHash

	delete(hs.pendingBlocks, height)

	proposer, _ := hs.validatorSet.GetProposer(height)
	proposerAddr := ""
	if proposer != nil {
		proposerAddr = fmt.Sprintf("%x", proposer.Address)
	}
	validatorAddrs := hs.getValidatorAddresses()
	isEpochRotation := height >= hs.epochStartHeight+hs.config.EpochLength

	commitErr := hs.blockProducer.CommitBlock(blockHash, height)
	if commitErr != nil {
		hs.logError(fmt.Sprintf("Failed to commit block height=%d error=%v", height, commitErr))
		return
	}

	if hs.auditLog != nil {
		hs.auditLog.LogFinalize(height, fmt.Sprintf("%x", blockHash), proposerAddr, "")
	}

	if hs.metrics != nil {
		hs.metrics.IncConsensusBlocksFinalized()
		hs.metrics.SetConsensusHeight(height)
	}

	hs.finality.CreateCheckpoint(height, blockHash, validatorAddrs)
	for _, addr := range validatorAddrs {
		hs.finality.AddVote(height, addr)
	}

	if isEpochRotation {
		hs.rotateEpoch(height)
	}

	hs.distributeRewards(height)

	if hs.BlockRewardFn != nil && proposer != nil {
		hs.BlockRewardFn(height, proposer.Address, nil)
	}

	hs.stateStore.Save(hs.state, validatorAddrs)

	if hs.state.Height != height {
		return
	}

	hs.state.Height++
	hs.state.View = 0
	hs.state.Phase = PhasePrepare
	hs.state.StartTime = time.Now()

	hs.curHeight.Store(hs.state.Height)
	hs.curView.Store(0)

	for addr := range hs.viewVotes {
		delete(hs.viewVotes, addr)
	}
	hs.votesOrder = hs.votesOrder[:0]

	for view := range hs.timeouts {
		delete(hs.timeouts, view)
		delete(hs.timeoutSignatures, view)
		delete(hs.timeoutHighQCs, view)
	}

	hs.viewTimeout = hs.config.ViewTimeout

	nextProposer, err := hs.validatorSet.GetProposer(hs.state.Height)
	if err == nil {
		hs.logInfo(fmt.Sprintf("Block finalized height=%d proposer=%x finality=%d",
			height, nextProposer.Address[:8], hs.finality.LastFinalized()))
	}

	if hs.validatorSet.Size() == 1 {
		if hs.timeoutTimer != nil {
			hs.timeoutTimer.Stop()
		}
		hs.timeoutTimer = time.AfterFunc(hs.config.ViewTimeout, func() {
			hs.mu.Lock()
			defer hs.mu.Unlock()
			if !hs.running.Load() {
				return
			}
			if hs.state.Phase == PhasePrepare {
				hs.proposeLocked()
			}
		})
	} else {
		nextProposer, err := hs.validatorSet.GetProposer(hs.state.Height)
		if err == nil && bytes.Equal(hs.blockProducer.GetValidatorAddress(), nextProposer.Address) {
			hs.triggerPropose()
		}
		hs.startTimeout()
	}

	if hs.config.ProtocolVersion > 0 {
		viols := hs.verifyInvariants()
		if len(viols) > 0 {
			if hs.metrics != nil {
				hs.metrics.IncConsensusInvariantViolations()
			}
		}
	}
}

func (hs *HotStuffEngine) updateView(newView uint64) {
	oldView := hs.state.View
	hs.state.View = newView
	hs.state.Phase = PhasePrepare
	hs.curView.Store(newView)

	if hs.metrics != nil {
		hs.metrics.SetConsensusView(newView)
		hs.metrics.SetConsensusPhase(float64(PhasePrepare))
		hs.metrics.IncConsensusViewChanges()
	}

	if hs.auditLog != nil {
		hs.auditLog.LogViewChange(oldView, newView, "timeout")
	}

	hs.viewTimeout = hs.config.ViewTimeout + (hs.config.TimeoutIncrease * time.Duration(newView))
	if hs.viewTimeout > hs.config.MaxViewTimeout {
		hs.viewTimeout = hs.config.MaxViewTimeout
	}

	for addr := range hs.viewVotes {
		delete(hs.viewVotes, addr)
	}

	delete(hs.pendingBlocks, hs.state.Height)

	if hs.validatorSet.Size() > 1 {
		hs.startTimeout()
	}

	hs.logInfo(fmt.Sprintf("View changed to %d at height %d", newView, hs.state.Height))

	if hs.config.ProtocolVersion > 0 {
		viols := hs.verifyInvariants()
		if len(viols) > 0 {
			if hs.metrics != nil {
				hs.metrics.IncConsensusInvariantViolations()
			}
		}
	}
}

func (hs *HotStuffEngine) startTimeout() {
	if hs.timeoutTimer != nil {
		hs.timeoutTimer.Stop()
	}

	hs.timeoutTimer = time.AfterFunc(hs.viewTimeout, func() {
		hs.mu.Lock()
		defer hs.mu.Unlock()

		if !hs.running.Load() {
			return
		}

		newView := hs.state.View + 1

		timeoutData := hs.createTimeoutData(hs.state.Height, newView)
		sig, err := hs.blockProducer.Sign(timeoutData)
		if err != nil {
			return
		}

		var highQC *QC
		if hs.state.PreparedQC != nil {
			highQC = hs.state.PreparedQC
		}

		timeoutPayload := hs.encodeTimeoutPayload(newView, sig, highQC)

		timeoutMsg := &ConsensusMessage{
			Type:      MsgTimeout,
			Height:    hs.state.Height,
			View:      newView,
			Validator: hs.blockProducer.GetValidatorAddress(),
			Signature: sig,
			Payload:   timeoutPayload,
			JustifyQC: highQC,
			Timestamp: time.Now(),
		}

		hs.doBroadcast(timeoutMsg)

		if hs.timeouts[newView] == nil {
			hs.timeouts[newView] = make(map[string]bool)
		}
		if hs.timeoutSignatures[newView] == nil {
			hs.timeoutSignatures[newView] = make(map[string]*crypto.Signature)
		}
		if hs.timeoutHighQCs[newView] == nil {
			hs.timeoutHighQCs[newView] = make(map[string]*QC)
		}

		validatorAddr := hex.EncodeToString(hs.blockProducer.GetValidatorAddress())
		hs.timeouts[newView][validatorAddr] = true
		hs.timeoutSignatures[newView][validatorAddr] = sig
		hs.timeoutHighQCs[newView][validatorAddr] = highQC

		if hs.validatorSet.HasSuperMajority(hs.timeouts[newView]) {
			hs.createNewViewAndBroadcast(newView)
			hs.updateView(newView)

			proposer, err := hs.validatorSet.GetProposerForView(hs.state.Height, hs.state.View)
			if err == nil && bytes.Equal(hs.blockProducer.GetValidatorAddress(), proposer.Address) {
				if hs.state.Phase == PhasePrepare {
					hs.triggerPropose()
				}
			}
		}
	})
}

func (hs *HotStuffEngine) isValidQC(qc *QC) bool {
	if qc == nil {
		return false
	}
	return qc.IsValid(hs.validatorSet)
}

func (hs *HotStuffEngine) doBroadcast(msg *ConsensusMessage) {
	if hs.validatorSet != nil && hs.validatorSet.Size() > 1 {
		if hs.broadcastFn != nil {
			go hs.broadcastFn(msg)
		}
		if hs.messageCh != nil {
			select {
			case hs.messageCh <- msg:
			default:
			}
		}
	}
}

func (hs *HotStuffEngine) SetBroadcast(fn BroadcastFunc) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.broadcastFn = fn
}

func (hs *HotStuffEngine) handleDoubleSign(evidence *DoubleSignRecord) {
	if evidence.IsSlashed {
		return
	}

	amount, err := hs.staking.Slash(evidence.Validator, hs.config.SlashingFraction)
	if err != nil {
		hs.logError(fmt.Sprintf("Failed to slash validator error=%v", err))
		return
	}

	hs.doubleSign.MarkSlashed(evidence.Validator, evidence.Height)
	evidence.SlashAmount = amount
	evidence.IsSlashed = true

	hs.logWarn(fmt.Sprintf("Double sign detected validator=%x type=%d slashed=%d",
		evidence.Validator, evidence.Type, amount))

	if hs.auditLog != nil {
		hs.auditLog.LogValidator("slashed", fmt.Sprintf("%x", evidence.Validator), amount, fmt.Sprintf("double_sign_type_%d", evidence.Type))
	}

	if err := hs.validatorSet.RemoveValidator(evidence.Validator); err != nil {
		hs.logWarn(fmt.Sprintf("RemoveValidator failed for slashed validator: %v", err))
	}
}

func (hs *HotStuffEngine) handleDowntimeSlash(validator []byte, missed uint64) {
	record, exists := hs.staking.GetValidator(validator)
	if !exists || !record.IsActive || record.Jailed {
		return
	}

	_, err := hs.staking.Slash(validator, hs.config.SlashingFraction/2)
	if err != nil {
		return
	}

	hs.logWarn(fmt.Sprintf("Downtime slash validator=%x missed=%d", validator, missed))
}

func (hs *HotStuffEngine) processAutoUnjail() {
	validators := hs.staking.GetActiveValidators()
	for _, v := range validators {
		if !v.Jailed {
			continue
		}

		if time.Now().After(v.JailedUntil) {
			if err := hs.staking.Unjail(v.Address); err != nil {
				hs.logError(fmt.Sprintf("Failed to unjail validator %x: %v", v.Address, err))
				continue
			}
			hs.logInfo(fmt.Sprintf("Auto-unjailed validator %x", v.Address))
		}
	}

	inactiveValidators := hs.staking.GetInactiveValidators()
	for _, v := range inactiveValidators {
		if !v.Jailed {
			continue
		}

		if time.Now().After(v.JailedUntil) {
			if err := hs.staking.Unjail(v.Address); err != nil {
				hs.logError(fmt.Sprintf("Failed to unjail inactive validator %x: %v", v.Address, err))
			}
		}
	}
}

func (hs *HotStuffEngine) rotateEpoch(height uint64) {
	hs.epochStartHeight = height + 1

	activeValidators := hs.staking.GetActiveValidators()
	if len(activeValidators) < hs.config.MinValidators {
		hs.logWarn("Insufficient validators for epoch rotation")
		return
	}

	newValidators := make([]*Validator, 0, len(activeValidators))
	for _, sv := range activeValidators {
		newValidators = append(newValidators, &Validator{
			Address:  sv.Address,
			PublicKey: sv.PublicKey,
			Stake:    sv.Stake,
			IsActive: true,
		})

		if hs.auditLog != nil {
			hs.auditLog.LogValidator("epoch_add", fmt.Sprintf("%x", sv.Address), sv.Stake, "")
		}
	}

	hs.validatorSet = NewValidatorSet(newValidators, height/hs.config.EpochLength+1)

	hs.applyEpochSlashing()
	hs.liveness.Reset()

	hs.logInfo(fmt.Sprintf("Epoch rotated new_epoch=%d validators=%d",
		hs.validatorSet.Epoch(), hs.validatorSet.Size()))
}

func (hs *HotStuffEngine) applyEpochSlashing() {
	evidences := hs.doubleSign.GetEvidence()
	for _, evidence := range evidences {
		if evidence.IsSlashed {
			continue
		}
		amount, err := hs.staking.Slash(evidence.Validator, hs.config.SlashingFraction)
		if err != nil {
			continue
		}
		hs.doubleSign.MarkSlashed(evidence.Validator, evidence.Height)
		evidence.SlashAmount = amount
		evidence.IsSlashed = true
	if err := hs.validatorSet.RemoveValidator(evidence.Validator); err != nil {
		hs.logWarn(fmt.Sprintf("RemoveValidator failed for slashed validator: %v", err))
	}
	}
}

func (hs *HotStuffEngine) distributeRewards(height uint64) {
	if hs.rewardPool == nil || hs.rewardPool.Sign() == 0 {
		return
	}

	validators := hs.validatorSet.GetValidators()
	if len(validators) == 0 {
		return
	}

	rewardPerValidator := new(big.Int).Div(hs.rewardPool, big.NewInt(int64(len(validators))))
	for _, v := range validators {
		if rewardPerValidator.IsUint64() {
			hs.staking.Stake(v.Address, v.PublicKey, rewardPerValidator.Uint64())
		}
	}

	hs.rewardPool = new(big.Int)
}

func (hs *HotStuffEngine) AddReward(amount uint64) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if hs.rewardPool == nil {
		hs.rewardPool = new(big.Int)
	}
	hs.rewardPool.Add(hs.rewardPool, new(big.Int).SetUint64(amount))
}

func (hs *HotStuffEngine) RegisterValidator(address []byte, publicKey []byte, stake uint64) error {
	return hs.staking.Stake(address, publicKey, stake)
}

func (hs *HotStuffEngine) DeregisterValidator(address []byte) error {
	record, exists := hs.staking.GetValidator(address)
	if !exists {
		return fmt.Errorf("validator not found")
	}

	return hs.staking.Unstake(address, record.Stake)
}

func (hs *HotStuffEngine) GetState() *ConsensusState {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	s := *hs.state
	return &s
}

// StakingModule returns the engine's staking module for querying validator stake info.
func (hs *HotStuffEngine) StakingModule() *StakingModule {
	return hs.staking
}

func (hs *HotStuffEngine) IsLeader() bool {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	proposer, err := hs.validatorSet.GetProposerForView(hs.state.Height, hs.state.View)
	if err != nil {
		return false
	}

	return bytes.Equal(hs.blockProducer.GetValidatorAddress(), proposer.Address)
}

func (hs *HotStuffEngine) Height() uint64 {
	return hs.curHeight.Load()
}

func (hs *HotStuffEngine) View() uint64 {
	return hs.curView.Load()
}

func (hs *HotStuffEngine) Phase() Phase {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return hs.state.Phase
}

func (hs *HotStuffEngine) Finality() *FinalityGadget {
	return hs.finality
}

func (hs *HotStuffEngine) DoubleSign() *DoubleSignDetector {
	return hs.doubleSign
}

func (hs *HotStuffEngine) Liveness() *LivenessTracker {
	return hs.liveness
}

func (hs *HotStuffEngine) getValidatorAddresses() []string {
	validators := hs.validatorSet.GetValidators()
	addrs := make([]string, 0, len(validators))
	for _, v := range validators {
		addrs = append(addrs, string(v.Address))
	}
	return addrs
}

func (hs *HotStuffEngine) restoreState(saved *PersistedState) {
	hs.state.Height = saved.Height
	hs.state.View = saved.View
	hs.state.Phase = saved.Phase
	hs.state.LockedQC = saved.LockedQC
	hs.state.PreparedQC = saved.PreparedQC
	hs.state.DecidedHash = saved.DecidedHash
	hs.state.StartTime = time.Now()
}

func (hs *HotStuffEngine) createProposalData(blockHash []byte) []byte {
	data := make([]byte, 8+len(blockHash))
	binary.BigEndian.PutUint64(data[0:8], hs.chainID)
	copy(data[8:], blockHash)
	return data
}

func (hs *HotStuffEngine) createVoteData(height, view uint64, phase Phase, blockHash []byte) []byte {
	data := make([]byte, 25+len(blockHash))
	binary.BigEndian.PutUint64(data[0:8], hs.chainID)
	binary.BigEndian.PutUint64(data[8:16], height)
	binary.BigEndian.PutUint64(data[16:24], view)
	data[24] = byte(phase)
	copy(data[25:], blockHash)
	return data
}

func (hs *HotStuffEngine) createTimeoutData(height uint64, view uint64) []byte {
	data := make([]byte, 24)
	binary.BigEndian.PutUint64(data[0:8], hs.chainID)
	binary.BigEndian.PutUint64(data[8:16], height)
	binary.BigEndian.PutUint64(data[16:24], view)
	return data
}

func (hs *HotStuffEngine) storeVoteSignature(height, view uint64, phase Phase, blockHash string, validatorAddr string, sig *crypto.Signature) {
	if sig == nil {
		return
	}
	reg := hs.getOrCreateVoteRegistry(height, view)
	if reg.VoteSignatureCache[phase] == nil {
		reg.VoteSignatureCache[phase] = make(map[string]map[string]*crypto.Signature)
	}
	if reg.VoteSignatureCache[phase][blockHash] == nil {
		reg.VoteSignatureCache[phase][blockHash] = make(map[string]*crypto.Signature)
	}
	reg.VoteSignatureCache[phase][blockHash][validatorAddr] = sig
}

func (hs *HotStuffEngine) createQC(height, view uint64, phase Phase, blockHash []byte) *QC {
	blockHashStr := hex.EncodeToString(blockHash)
	reg := hs.getOrCreateVoteRegistry(height, view)
	sigCache := reg.VoteSignatureCache[phase]
	if sigCache == nil || sigCache[blockHashStr] == nil {
		return &QC{
			Height:    height,
			View:      view,
			Phase:     phase,
			BlockHash: blockHash,
			Signatures: make(map[string]crypto.Signature),
			ValidatorAddrs: []string{},
		}
	}

	signatures := make(map[string]crypto.Signature)
	validatorAddrs := make([]string, 0, len(sigCache[blockHashStr]))

	for addr, sig := range sigCache[blockHashStr] {
		signatures[addr] = *sig
		validatorAddrs = append(validatorAddrs, addr)
	}

	return &QC{
		Height:    height,
		View:      view,
		Phase:     phase,
		BlockHash: blockHash,
		Signatures: signatures,
		ValidatorAddrs: validatorAddrs,
	}
}

func (hs *HotStuffEngine) GetMissed(validator string) uint64 {
	return hs.liveness.GetMissedString(validator)
}

func (hs *HotStuffEngine) ExportState() ([]byte, error) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	state := map[string]interface{}{
		"height":          hs.state.Height,
		"view":            hs.state.View,
		"phase":           hs.state.Phase,
		"reward_pool":     hs.rewardPool,
		"epoch_start":     hs.epochStartHeight,
		"last_finalized":  hs.finality.LastFinalized(),
		"validator_count": hs.validatorSet.Size(),
		"total_stake":     hs.validatorSet.TotalStake(),
	}

	return json.Marshal(state)
}

func (hs *HotStuffEngine) syncLoop() {
	defer hs.wg.Done()
	defer hs.recoverPanic("syncLoop")
	if hs.auditLog != nil {
		hs.auditLog.LogSync("start", hs.curHeight.Load(), 0, 0)
	}

	for {
		select {
		case msg := <-hs.syncMsgCh:
			hs.processSyncMessage(msg)
		case <-hs.done:
			if hs.auditLog != nil {
				hs.auditLog.LogSync("complete", hs.curHeight.Load(), 0, 100)
			}
			return
		}
	}
}

func (hs *HotStuffEngine) processSyncMessage(msg *ConsensusMessage) {
	currentHeight := hs.curHeight.Load()

	if msg.Height <= currentHeight+1 {
		hs.HandleMessage(msg)
		return
	}

	if hs.stateSyncer == nil || !hs.stateSyncer.IsSyncing() {
		return
	}

	if msg.Type == MsgProposal && len(msg.Payload) > 0 {
		blockData := prependHeightToPayload(msg.Payload, msg.Height)
		if err := hs.stateSyncer.ReceiveBlock(blockData); err != nil {
			hs.logWarn(fmt.Sprintf("Sync block apply failed height=%d error=%v", msg.Height, err))
		}
	}
}

func (hs *HotStuffEngine) defaultBlockRequester(fromHeight, toHeight uint64) error {
	if hs.broadcastFn == nil {
		return fmt.Errorf("no broadcast function")
	}

	payload := SerializeBlockRequest(fromHeight, toHeight)
	reqMsg := &ConsensusMessage{
		Type:      MsgBlockRequest,
		Height:    fromHeight,
		View:      toHeight,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	hs.broadcastFn(reqMsg)
	hs.logInfo(fmt.Sprintf("Requested blocks from=%d to=%d", fromHeight, toHeight))
	return nil
}

func (hs *HotStuffEngine) handleBlockRequestData(payload []byte) {
	if len(payload) < 16 {
		return
	}

	fromHeight := binary.BigEndian.Uint64(payload[:8])
	toHeight := binary.BigEndian.Uint64(payload[8:16])

	for h := fromHeight; h <= toHeight; h++ {
		blockData, err := hs.blockProducer.GetBlockData(h)
		if err != nil {
			continue
		}

		respMsg := &ConsensusMessage{
			Type:      MsgBlockResponse,
			Height:    h,
			View:      0,
			Payload:   blockData,
			Timestamp: time.Now(),
		}

		if hs.broadcastFn != nil {
			go hs.broadcastFn(respMsg)
		}
	}
}

func (hs *HotStuffEngine) defaultBlockApplier(blockData []byte) error {
	if len(blockData) < 8 {
		return fmt.Errorf("invalid block data")
	}

	height := binary.BigEndian.Uint64(blockData[:8])
	payload := blockData[8:]

	// Skip blocks already committed by normal consensus
	chainHeight := hs.blockProducer.GetChainHeight()
	if height <= chainHeight {
		hs.logInfo(fmt.Sprintf("Skipping already-committed block height=%d chain_height=%d", height, chainHeight))
		return nil
	}

	// Validate the block and get the actual block hash (DoubleSHA256 of signing payload)
	var block ledger.Block
	if err := json.Unmarshal(payload, &block); err != nil {
		return fmt.Errorf("failed to unmarshal block: %w", err)
	}
	blockHash := block.Hash()

	if err := hs.blockProducer.ValidateBlock(payload, blockHash, height); err != nil {
		return fmt.Errorf("block validation failed: %w", err)
	}

	resultCh := make(chan error, 1)
	req := &blockApplyRequest{
		height:   height,
		hash:     blockHash,
		resultCh: resultCh,
	}
	select {
	case hs.applyCh <- req:
	case <-hs.done:
		return fmt.Errorf("consensus stopped")
	}

	select {
	case err := <-resultCh:
		if err != nil {
			return fmt.Errorf("block commit failed: %w", err)
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("block commit timeout")
	}

	return nil
}

func (hs *HotStuffEngine) GetStateSyncer() *StateSyncer {
	return hs.stateSyncer
}

func (hs *HotStuffEngine) OnSyncComplete(callback func()) {
	go func() {
		for {
			if hs.stateSyncer == nil {
				return
			}
			if !hs.stateSyncer.IsSyncing() {
				localHeight := hs.curHeight.Load()

				proposer, err := hs.validatorSet.GetProposer(localHeight)
				if err == nil && bytes.Equal(hs.blockProducer.GetValidatorAddress(), proposer.Address) {
					hs.triggerPropose()
				}

				hs.mu.Lock()
				if hs.state.Phase == PhaseIdle {
					hs.state.Phase = PhasePrepare
				}
				hs.mu.Unlock()

				if callback != nil {
					callback()
				}
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

func prependHeightToPayload(payload []byte, height uint64) []byte {
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, height)
	result := make([]byte, 8+len(payload))
	copy(result[:8], heightBytes)
	copy(result[8:], payload)
	return result
}

type invariantViolation struct {
	check   string
	detail  string
}

func (hs *HotStuffEngine) verifyInvariants() []invariantViolation {
	var violations []invariantViolation

	if hs.state.Height == 0 {
		violations = append(violations, invariantViolation{
			check:  "height_nonzero",
			detail: fmt.Sprintf("state.Height=%d must be >= 1", hs.state.Height),
		})
	}

	if hs.state.Phase > PhaseDecide {
		violations = append(violations, invariantViolation{
			check:  "phase_valid",
			detail: fmt.Sprintf("state.Phase=%d must be in range [0,4]", hs.state.Phase),
		})
	}

	atomicHeight := hs.curHeight.Load()
	if atomicHeight != hs.state.Height {
		violations = append(violations, invariantViolation{
			check:  "height_consistency",
			detail: fmt.Sprintf("curHeight=%d != state.Height=%d", atomicHeight, hs.state.Height),
		})
	}

	atomicView := hs.curView.Load()
	if atomicView != hs.state.View {
		violations = append(violations, invariantViolation{
			check:  "view_consistency",
			detail: fmt.Sprintf("curView=%d != state.View=%d", atomicView, hs.state.View),
		})
	}

	if hs.state.PreparedQC != nil && hs.state.PreparedQC.Height > hs.state.Height {
		violations = append(violations, invariantViolation{
			check:  "preparedqc_height",
			detail: fmt.Sprintf("PreparedQC.Height=%d > state.Height=%d", hs.state.PreparedQC.Height, hs.state.Height),
		})
	}

	if hs.state.LockedQC != nil && hs.state.LockedQC.Height > hs.state.Height {
		violations = append(violations, invariantViolation{
			check:  "lockedqc_height",
			detail: fmt.Sprintf("LockedQC.Height=%d > state.Height=%d", hs.state.LockedQC.Height, hs.state.Height),
		})
	}

	if hs.state.PreparedQC != nil && hs.state.LockedQC != nil {
		if hs.state.PreparedQC.View < hs.state.LockedQC.View && hs.state.PreparedQC.Height <= hs.state.LockedQC.Height {
			violations = append(violations, invariantViolation{
				check:  "lock_ordering",
				detail: fmt.Sprintf("PreparedQC(view=%d,height=%d) should not be behind LockedQC(view=%d,height=%d)",
					hs.state.PreparedQC.View, hs.state.PreparedQC.Height,
					hs.state.LockedQC.View, hs.state.LockedQC.Height),
			})
		}
	}

	if hs.timeoutView < hs.state.View {
		violations = append(violations, invariantViolation{
			check:  "timeout_view",
			detail: fmt.Sprintf("timeoutView=%d < state.View=%d", hs.timeoutView, hs.state.View),
		})
	}

	for heightKey := range hs.viewVotes {
		parts := splitHeightKey(heightKey)
		if len(parts) >= 1 {
			voteHeight := parseUint64(parts[0])
			if voteHeight > 0 && voteHeight < hs.state.Height {
				violations = append(violations, invariantViolation{
					check:  "stale_votes",
					detail: fmt.Sprintf("vote entry for height=%d < state.Height=%d", voteHeight, hs.state.Height),
				})
				break
			}
		}
	}

	for view := range hs.timeouts {
		if view < hs.state.View {
			violations = append(violations, invariantViolation{
				check:  "stale_timeouts",
				detail: fmt.Sprintf("timeout entry for view=%d < state.View=%d", view, hs.state.View),
			})
			break
		}
	}

	if violations != nil {
		hs.logWarn(fmt.Sprintf("Invariant violations detected: %d", len(violations)))
		for _, v := range violations {
			hs.logWarn(fmt.Sprintf("  [%s] %s", v.check, v.detail))
		}
	}

	return violations
}

func splitHeightKey(key string) []string {
	parts := []string{}
	start := 0
	for i := 0; i < len(key); i++ {
		if key[i] == '-' {
			parts = append(parts, key[start:i])
			start = i + 1
		}
	}
	parts = append(parts, key[start:])
	return parts
}

func parseUint64(s string) uint64 {
	var n uint64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + uint64(c-'0')
		}
	}
	return n
}

func safeValStr(val []byte) string {
	if len(val) == 0 {
		return ""
	}
	if len(val) >= 4 {
		return hex.EncodeToString(val[:4])
	}
	return hex.EncodeToString(val)
}
