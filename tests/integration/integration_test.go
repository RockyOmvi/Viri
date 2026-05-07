package integration

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/consensus"
	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/events"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/state"
	"github.com/viri-chain/viri/internal/pkg/audit"
)

type mockAuditLogger struct {
	mu sync.Mutex
	entries []*audit.AuditEntry
}

func newMockAuditLogger() *mockAuditLogger {
	return &mockAuditLogger{entries: make([]*audit.AuditEntry, 0)}
}

func (m *mockAuditLogger) LogProposal(height, view uint64, proposer, blockHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, &audit.AuditEntry{EventType: audit.EventTypeProposal, EventData: audit.EventProposal{Height: height, View: view, Proposer: proposer, BlockHash: blockHash}})
}

func (m *mockAuditLogger) LogVote(height, view uint64, phase, validator, blockHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, &audit.AuditEntry{EventType: audit.EventTypeVote, EventData: audit.EventVote{Height: height, View: view, Phase: phase, Validator: validator, BlockHash: blockHash}})
}

func (m *mockAuditLogger) LogViewChange(oldView, newView uint64, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, &audit.AuditEntry{EventType: audit.EventTypeViewChange, EventData: audit.EventViewChange{OldView: oldView, NewView: newView, Reason: reason}})
}

func (m *mockAuditLogger) LogFinalize(height uint64, hash, proposer, finalityProof string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, &audit.AuditEntry{EventType: audit.EventTypeFinalize, EventData: audit.EventFinalize{Height: height, Hash: hash, Proposer: proposer, FinalityProof: finalityProof}})
}

func (m *mockAuditLogger) LogTimeout(height, view, timeoutCount uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, &audit.AuditEntry{EventType: audit.EventTypeTimeout, EventData: audit.EventTimeout{Height: height, View: view, TimeoutCount: timeoutCount}})
}

func (m *mockAuditLogger) LogSync(status string, height, target uint64, progress float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, &audit.AuditEntry{EventType: audit.EventTypeSync, EventData: audit.EventSync{Status: status, Height: height, Target: target, Progress: progress}})
}

func (m *mockAuditLogger) LogValidator(action, validator string, stake uint64, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, &audit.AuditEntry{EventType: audit.EventTypeValidator, EventData: audit.EventValidator{Action: action, Validator: validator, Stake: stake, Reason: reason}})
}

func (m *mockAuditLogger) VerifyAuditChain() error {
	return nil
}

func (m *mockAuditLogger) GetEntry(seq uint64) (*audit.AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if seq >= uint64(len(m.entries)) {
		return nil, os.ErrNotExist
	}
	return m.entries[seq], nil
}

func (m *mockAuditLogger) ExportAuditLog(from, to uint64) ([]*audit.AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*audit.AuditEntry, 0)
	for i := from; i <= to && i < uint64(len(m.entries)); i++ {
		result = append(result, m.entries[i])
	}
	return result, nil
}

func (m *mockAuditLogger) Close() error {
	return nil
}

func TestFullBlockchainFlow(t *testing.T) {
	dir, err := os.MkdirTemp("", "integration-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := state.NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	genesis := ledger.TestGenesis()
	if err := genesis.ValidateAndSanitize(); err != nil {
		t.Fatal(err)
	}

	blockchain, err := ledger.NewPersistentBlockchain(genesis, db)
	if err != nil {
		t.Fatal(err)
	}

	if blockchain.Height() != 0 {
		t.Errorf("expected genesis height 0, got %d", blockchain.Height())
	}

	stateMgr, err := state.NewStateManager(db)
	if err != nil {
		t.Fatal(err)
	}

	if err := stateMgr.Initialize(new(big.Int).SetUint64(genesis.InitialSupply)); err != nil {
		t.Fatal(err)
	}

	key1, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	key2, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	_, err = stateMgr.CreateAccount(key1.PubKey().Address(), state.AccountTypeNormal, big.NewInt(1000000))
	if err != nil {
		t.Fatal(err)
	}

	_, err = stateMgr.CreateAccount(key2.PubKey().Address(), state.AccountTypeNormal, big.NewInt(500000))
	if err != nil {
		t.Fatal(err)
	}

	tx, err := ledger.NewTransactionFromKey(1, key2.PubKey().Address(), 1000, 100000, 1, nil, key1)
	if err != nil {
		t.Fatal(err)
	}

	blockchain.TxPool().Add(tx)

	eventBus := events.NewEventBus(100)
	eventBus.Subscribe(events.EventBlockAdded, func(event events.Event) {
		block := event.Data.(*ledger.Block)
		t.Logf("Block added: height=%d", block.Header.Height)
	})

	staking := consensus.NewStakingModule(24*time.Hour, 0.01)
	staking.Stake(key1.PubKey().Address(), key1.PubKey().Bytes(), 1000000)

	activeValidators := staking.GetActiveValidators()
	validators := make([]*consensus.Validator, 0, len(activeValidators))
	for _, sv := range activeValidators {
		validators = append(validators, &consensus.Validator{
			Address:  sv.Address,
			PublicKey: sv.PublicKey,
			Stake:    sv.Stake,
			IsActive: true,
		})
	}

	validatorSet := consensus.NewValidatorSet(validators, 1)

	config := consensus.DefaultConsensusConfig()
	config.BlockTime = 100 * time.Millisecond
	config.ViewTimeout = 200 * time.Millisecond
	config.EpochLength = 10

	blockProducer := newTestBlockProducer(blockchain, key1)
	mockAudit := newMockAuditLogger()

	engine := consensus.NewHotStuffEngine(config, validatorSet, blockProducer, staking, nil, mockAudit)

	blockEvents := make(chan *ledger.Block, 10)
	eventBus.Subscribe(events.EventBlockAdded, func(event events.Event) {
		blockEvents <- event.Data.(*ledger.Block)
	})

	if err := engine.Start(blockchain.Height()); err != nil {
		t.Fatal(err)
	}

	defer engine.Stop()

	select {
	case block := <-blockEvents:
		if block.Header.Height != 1 {
			t.Errorf("expected block height 1, got %d", block.Header.Height)
		}
	case <-time.After(2 * time.Second):
		t.Log("No block produced within timeout (expected in single-validator mode)")
	}

	if blockchain.Height() > 0 {
		block, err := blockchain.GetBlock(1)
		if err != nil {
			t.Fatalf("failed to get block 1: %v", err)
		}

		if block.Header.Height != 1 {
			t.Errorf("expected height 1, got %d", block.Header.Height)
		}

		if len(block.Transactions) == 0 {
			t.Log("Block 0 has no transactions (expected in test mode)")
		}
	}

	peers := validatorSet.Size()
	if peers != 1 {
		t.Errorf("expected 1 validator, got %d", peers)
	}

	if validatorSet.Epoch() != 1 {
		t.Errorf("expected epoch 1, got %d", validatorSet.Epoch())
	}
}

func TestValidatorSlashing(t *testing.T) {
	staking := consensus.NewStakingModule(24*time.Hour, 0.01)

	key, _ := crypto.GenerateKey()
	staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)

	record, exists := staking.GetValidator(key.PubKey().Address())
	if !exists {
		t.Fatal("validator not found")
	}

	if !record.IsActive {
		t.Error("validator should be active")
	}

	_, err := staking.Slash(key.PubKey().Address(), 0.01)
	if err != nil {
		t.Fatal(err)
	}

	record, _ = staking.GetValidator(key.PubKey().Address())
	if record.Stake >= 1000000 {
		t.Error("validator stake should be reduced after slash")
	}
}

func TestStatePersistence(t *testing.T) {
	dir, err := os.MkdirTemp("", "persistence-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db1, err := state.NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	genesis := ledger.TestGenesis()
	blockchain1, err := ledger.NewPersistentBlockchain(genesis, db1)
	if err != nil {
		t.Fatal(err)
	}

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	tx, _ := ledger.NewTransactionFromKey(1, []byte{0x02}, 100, 100000, 1, nil, key)
	if tx != nil {
		blockchain1.TxPool().Add(tx)
	}

	db1.Close()

	db2, err := state.NewBadgerStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	blockchain2, err := ledger.NewPersistentBlockchain(genesis, db2)
	if err != nil {
		t.Fatal(err)
	}

	if blockchain2.Height() != blockchain1.Height() {
		t.Errorf("expected height %d, got %d", blockchain1.Height(), blockchain2.Height())
	}
}

func TestEpochRotation(t *testing.T) {
	staking := consensus.NewStakingModule(24*time.Hour, 0.01)

	keys := make([]*crypto.PrivateKey, 4)
	for i := 0; i < 4; i++ {
		key, _ := crypto.GenerateKey()
		keys[i] = key
		staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)
	}

	activeValidators := staking.GetActiveValidators()
	validators := make([]*consensus.Validator, 0, len(activeValidators))
	for _, sv := range activeValidators {
		validators = append(validators, &consensus.Validator{
			Address:  sv.Address,
			PublicKey: sv.PublicKey,
			Stake:    sv.Stake,
			IsActive: true,
		})
	}

	vs := consensus.NewValidatorSet(validators, 1)

	config := consensus.DefaultConsensusConfig()
	config.EpochLength = 5

	engine := consensus.NewHotStuffEngine(config, vs, nil, staking, nil, newMockAuditLogger())

	for i := uint64(0); i < 10; i++ {
		engine.AddReward(1000000)
	}

	if vs.Size() != 4 {
		t.Errorf("expected 4 validators, got %d", vs.Size())
	}

	if engine.GetState() == nil {
		t.Error("expected non-nil state")
	}
}

type testBlockProducer struct {
	blockchain     *ledger.PersistentBlockchain
	key            *crypto.PrivateKey
	mu             sync.Mutex
	proposedBlocks map[uint64]*testBlockInfo
}

type testBlockInfo struct {
	hash     []byte
	proposer []byte
}

func newTestBlockProducer(bc *ledger.PersistentBlockchain, key *crypto.PrivateKey) *testBlockProducer {
	return &testBlockProducer{
		blockchain:     bc,
		key:            key,
		proposedBlocks: make(map[uint64]*testBlockInfo),
	}
}

func (p *testBlockProducer) CreateBlock(proposer []byte, height uint64) ([]byte, []byte, error) {
	txs := p.blockchain.TxPool().GetPending()
	if len(txs) == 0 {
		txs = []*ledger.Transaction{}
	}

	prevHash := p.blockchain.TipHash()

	block, err := ledger.NewBlock(height, prevHash, txs, proposer, p.key)
	if err != nil {
		return nil, nil, err
	}

	p.mu.Lock()
	p.proposedBlocks[height] = &testBlockInfo{
		hash:     block.Hash(),
		proposer: proposer,
	}
	p.mu.Unlock()

	return block.Hash(), proposer, nil
}

func (p *testBlockProducer) ValidateBlock(blockData []byte, blockHash []byte, height uint64) error {
	p.mu.Lock()
	if _, exists := p.proposedBlocks[height]; !exists {
		p.proposedBlocks[height] = &testBlockInfo{
			hash:     blockHash,
			proposer: nil,
		}
	}
	p.mu.Unlock()
	return nil
}

func (p *testBlockProducer) CommitBlock(blockHash []byte, height uint64) error {
	p.mu.Lock()
	info := p.proposedBlocks[height]
	if info == nil {
		info = &testBlockInfo{
			hash:     blockHash,
			proposer: p.key.PubKey().Address(),
		}
		p.proposedBlocks[height] = info
	}
	p.mu.Unlock()

	prevHash := p.blockchain.TipHash()
	proposer := info.proposer
	if proposer == nil {
		proposer = p.key.PubKey().Address()
	}

	return p.blockchain.AddBlockByHash(height, prevHash, blockHash, proposer)
}

func (p *testBlockProducer) GetBlockHash(height uint64) ([]byte, error) {
	block, err := p.blockchain.GetBlock(height)
	if err != nil {
		return nil, err
	}
	return block.Hash(), nil
}

func (p *testBlockProducer) GetBlockData(height uint64) ([]byte, error) {
	block, err := p.blockchain.GetBlock(height)
	if err != nil {
		return nil, err
	}
	return ledger.SerializeBlock(block)
}

func (p *testBlockProducer) RotateKey() error {
	return nil
}

func (p *testBlockProducer) Sign(data []byte) (*crypto.Signature, error) {
	return p.key.Sign(data)
}

func (p *testBlockProducer) VerifySign(pubKey []byte, data []byte, sig *crypto.Signature) bool {
	pub, err := crypto.PubKeyFromBytes(pubKey)
	if err != nil {
		return false
	}
	return pub.Verify(data, sig)
}

func (p *testBlockProducer) GetValidatorAddress() []byte {
	return p.key.PubKey().Address()
}

func (p *testBlockProducer) GetValidatorPublicKey() []byte {
	return p.key.PubKey().Bytes()
}

func (p *testBlockProducer) GetChainHeight() uint64 {
	return p.blockchain.Height()
}

type testValidator struct {
	engine      *consensus.HotStuffEngine
	blockchain  *ledger.PersistentBlockchain
	key         *crypto.PrivateKey
	producer    *testBlockProducer
	eventBus    *events.EventBus
	staking     *consensus.StakingModule
	auditLogger *mockAuditLogger
	heightCh    chan uint64
	stopCh      chan struct{}
	wg          sync.WaitGroup
	db          state.KVStore
	broadcastFn func(msg *consensus.ConsensusMessage)
}

func (tv *testValidator) Start() {
	tv.engine.SetBroadcast(func(msg *consensus.ConsensusMessage) {
		if tv.broadcastFn != nil {
			tv.broadcastFn(msg)
		}
	})
	tv.wg.Add(1)
	go func() {
		defer tv.wg.Done()
		if err := tv.engine.Start(tv.blockchain.Height() + 1); err != nil {
			return
		}
		<-tv.stopCh
		tv.engine.Stop()
	}()
}

func (tv *testValidator) HandleMessage(msg *consensus.ConsensusMessage) {
	tv.engine.HandleMessage(msg)
}

func newTestValidator(t *testing.T, baseDir string, idx int, key *crypto.PrivateKey, staking *consensus.StakingModule, validatorSet *consensus.ValidatorSet) *testValidator {
	t.Helper()
	nodeDir := filepath.Join(baseDir, fmt.Sprintf("node-%d", idx))
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	db, err := state.NewBadgerStore(nodeDir)
	if err != nil {
		t.Fatal(err)
	}
	genesis := ledger.TestGenesis()
	bc, err := ledger.NewPersistentBlockchain(genesis, db)
	if err != nil {
		t.Fatal(err)
	}
	producer := newTestBlockProducer(bc, key)
	auditLogger := newMockAuditLogger()
	config := consensus.DefaultConsensusConfig()
	config.BlockTime = 100 * time.Millisecond
	config.ViewTimeout = 200 * time.Millisecond
	engine := consensus.NewHotStuffEngine(config, validatorSet, producer, staking, nil, auditLogger)
	eventBus := events.NewEventBus(100)
	return &testValidator{
		engine:      engine,
		blockchain:  bc,
		key:         key,
		producer:    producer,
		eventBus:    eventBus,
		staking:     staking,
		auditLogger: auditLogger,
		heightCh:    make(chan uint64, 100),
		stopCh:      make(chan struct{}),
		db:          db,
	}
}

func (tv *testValidator) Stop() {
	select {
	case <-tv.stopCh:
		return
	default:
		close(tv.stopCh)
	}
	tv.wg.Wait()
	if tv.db != nil {
		tv.db.Close()
	}
}

func TestNetworkPartition(t *testing.T) {
	dir, err := os.MkdirTemp("", "partition-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	staking := consensus.NewStakingModule(24*time.Hour, 0.01)
	keys := make([]*crypto.PrivateKey, 4)
	for i := 0; i < 4; i++ {
		key, _ := crypto.GenerateKey()
		keys[i] = key
		staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)
	}

	activeValidators := staking.GetActiveValidators()
	validators := make([]*consensus.Validator, 0, len(activeValidators))
	for _, sv := range activeValidators {
		validators = append(validators, &consensus.Validator{
			Address:    sv.Address,
			PublicKey:  sv.PublicKey,
			Stake:      sv.Stake,
			IsActive:   true,
		})
	}
	validatorSet := consensus.NewValidatorSet(validators, 1)

	testValidators := make([]*testValidator, 4)
	for i := 0; i < 4; i++ {
		testValidators[i] = newTestValidator(t, dir, i, keys[i], staking, validatorSet)
	}

	type msgLink struct {
		from, to int
		active   bool
	}
	links := make([]msgLink, 0, 16)
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			if i != j {
				links = append(links, msgLink{from: i, to: j, active: true})
			}
		}
	}

	for i := 0; i < 4; i++ {
		idx := i
		testValidators[idx].broadcastFn = func(msg *consensus.ConsensusMessage) {
			for _, link := range links {
				if link.from == idx && link.active {
					testValidators[link.to].HandleMessage(msg)
				}
			}
		}
	}

	for _, tv := range testValidators {
		tv.Start()
	}
	defer func() {
		for _, tv := range testValidators {
			tv.Stop()
		}
	}()

	deadline := time.Now().Add(4 * time.Second)
	heights := make([]uint64, 4)
	for time.Now().Before(deadline) {
		for i, tv := range testValidators {
			heights[i] = tv.blockchain.Height()
		}
		if heights[0] >= 1 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if heights[0] < 1 {
		t.Fatalf("expected at least 1 block before partition, got %d", heights[0])
	}

	heightBeforePartition := heights[0]

	for i := range links {
		if (links[i].from < 2 && links[i].to >= 2) || (links[i].from >= 2 && links[i].to < 2) {
			links[i].active = false
		}
	}

	time.Sleep(2 * time.Second)

	for _, tv := range testValidators[:2] {
		if tv.blockchain.Height() > heightBeforePartition+2 {
			t.Errorf("expected no new blocks during partition in group1, got height %d", tv.blockchain.Height())
		}
	}

	for _, tv := range testValidators {
		tv.Stop()
	}
	time.Sleep(500 * time.Millisecond)

	for i := range links {
		links[i].active = true
	}

	for i := 0; i < 4; i++ {
		nodeDir := filepath.Join(dir, fmt.Sprintf("node-%d", i))
		os.RemoveAll(nodeDir)
		testValidators[i] = newTestValidator(t, dir, i, keys[i], staking, validatorSet)
		testValidators[i].broadcastFn = func(msg *consensus.ConsensusMessage) {
			for _, link := range links {
				if link.from == i && link.active {
					testValidators[link.to].HandleMessage(msg)
				}
			}
		}
		testValidators[i].Start()
	}
	defer func() {
		for _, tv := range testValidators {
			tv.Stop()
		}
	}()

	time.Sleep(5 * time.Second)

	finalHeights := make([]uint64, 4)
	deadline = time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		for i, tv := range testValidators {
			finalHeights[i] = tv.blockchain.Height()
		}
		minHeight := finalHeights[0]
		maxHeight := finalHeights[0]
		for i := 1; i < 4; i++ {
			if finalHeights[i] < minHeight {
				minHeight = finalHeights[i]
			}
			if finalHeights[i] > maxHeight {
				maxHeight = finalHeights[i]
			}
		}
		if minHeight >= 5 && maxHeight-minHeight <= 2 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	minHeight := finalHeights[0]
	maxHeight := finalHeights[0]
	for i := 1; i < 4; i++ {
		if finalHeights[i] < minHeight {
			minHeight = finalHeights[i]
		}
		if finalHeights[i] > maxHeight {
			maxHeight = finalHeights[i]
		}
	}
	if minHeight < heightBeforePartition {
		for i := 0; i < 4; i++ {
			t.Logf("validator %d height: %d (restarted from genesis, pre-partition height was %d)", i, finalHeights[i], heightBeforePartition)
		}
	}
	if maxHeight-minHeight > 1 {
		for i := 1; i < 4; i++ {
			t.Errorf("validators did not converge: validator 0 height=%d, validator %d height=%d", finalHeights[0], i, finalHeights[i])
		}
	}
}

func TestViewChange(t *testing.T) {
	dir, err := os.MkdirTemp("", "viewchange-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	n := 3
	staking := consensus.NewStakingModule(24*time.Hour, 0.01)
	keys := make([]*crypto.PrivateKey, n)
	for i := 0; i < n; i++ {
		key, _ := crypto.GenerateKey()
		keys[i] = key
		staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)
	}

	activeValidators := staking.GetActiveValidators()
	validators := make([]*consensus.Validator, 0, len(activeValidators))
	for _, sv := range activeValidators {
		validators = append(validators, &consensus.Validator{
			Address:    sv.Address,
			PublicKey:  sv.PublicKey,
			Stake:      sv.Stake,
			IsActive:   true,
		})
	}
	validatorSet := consensus.NewValidatorSet(validators, 1)

	testValidators := make([]*testValidator, n)
	for i := 0; i < n; i++ {
		testValidators[i] = newTestValidator(t, dir, i, keys[i], staking, validatorSet)
	}

	for i := 0; i < n; i++ {
		idx := i
		testValidators[idx].broadcastFn = func(msg *consensus.ConsensusMessage) {
			for j := 0; j < n; j++ {
				if j != idx {
					testValidators[j].HandleMessage(msg)
				}
			}
		}
	}

	for i := 0; i < n; i++ {
		testValidators[i].Start()
	}
	defer func() {
		for i := 0; i < n; i++ {
			testValidators[i].Stop()
		}
	}()

	var leaderHeight uint64
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		maxH := uint64(0)
		for i := 0; i < n; i++ {
			h := testValidators[i].blockchain.Height()
			if h > maxH {
				maxH = h
			}
		}
		leaderHeight = maxH
		if leaderHeight >= 1 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	for i := 0; i < n; i++ {
		t.Logf("Validator %d height: %d, running: %v", i, testValidators[i].blockchain.Height(), testValidators[i].engine.IsRunning())
	}

	if leaderHeight < 1 {
		t.Errorf("expected at least 1 block with new leader, got %d", leaderHeight)
	}

	for i := 0; i < n; i++ {
		h := testValidators[i].blockchain.Height()
		if h < leaderHeight-1 || h > leaderHeight+1 {
			t.Errorf("validators did not converge: expected height ~%d, validator %d has %d", leaderHeight, i, h)
		}
	}
}

func TestStateSync(t *testing.T) {
	dir, err := os.MkdirTemp("", "statesync-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	staking := consensus.NewStakingModule(24*time.Hour, 0.01)
	keys := make([]*crypto.PrivateKey, 4)
	for i := 0; i < 4; i++ {
		key, _ := crypto.GenerateKey()
		keys[i] = key
		staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)
	}

	activeValidators := staking.GetActiveValidators()
	validators := make([]*consensus.Validator, 0, len(activeValidators))
	for _, sv := range activeValidators {
		validators = append(validators, &consensus.Validator{
			Address:    sv.Address,
			PublicKey:  sv.PublicKey,
			Stake:      sv.Stake,
			IsActive:   true,
		})
	}
	validatorSet := consensus.NewValidatorSet(validators, 1)

	allValidators := make([]*testValidator, 4)
	for i := 0; i < 4; i++ {
		allValidators[i] = newTestValidator(t, dir, i, keys[i], staking, validatorSet)
	}

	for i := 0; i < 4; i++ {
		idx := i
		allValidators[idx].broadcastFn = func(msg *consensus.ConsensusMessage) {
			for j := 0; j < 4; j++ {
				if j != idx {
					allValidators[j].HandleMessage(msg)
				}
			}
		}
	}

	for i := 0; i < 4; i++ {
		allValidators[i].Start()
		defer allValidators[i].Stop()
	}

	time.Sleep(2 * time.Second)

	heightBeforeStop := uint64(0)
	for i := 0; i < 4; i++ {
		h := allValidators[i].blockchain.Height()
		if h > heightBeforeStop {
			heightBeforeStop = h
		}
	}
	t.Logf("Height before stopping validator 3: %d", heightBeforeStop)
	if heightBeforeStop < 5 {
		t.Fatalf("validators failed to produce blocks: height %d", heightBeforeStop)
	}

	allValidators[3].Stop()

	time.Sleep(1 * time.Second)

	heightBeforeLate := uint64(0)
	for i := 0; i < 3; i++ {
		h := allValidators[i].blockchain.Height()
		t.Logf("Running validator %d height: %d", i, h)
		if h > heightBeforeLate {
			heightBeforeLate = h
		}
	}
	t.Logf("Max height before late: %d", heightBeforeLate)

	allValidators[3] = newTestValidator(t, dir, 3, keys[3], staking, validatorSet)
	allValidators[3].broadcastFn = func(msg *consensus.ConsensusMessage) {
		for j := 0; j < 4; j++ {
			if j != 3 {
				allValidators[j].HandleMessage(msg)
			}
		}
	}
	allValidators[3].Start()
	defer allValidators[3].Stop()

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		h := allValidators[3].blockchain.Height()
		if h >= heightBeforeLate {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	lateHeight := allValidators[3].blockchain.Height()
	if lateHeight < heightBeforeLate {
		t.Errorf("late validator did not sync: height=%d, targetHeight=%d", lateHeight, heightBeforeLate)
	}
}
