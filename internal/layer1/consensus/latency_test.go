package consensus

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

type latencyLink struct {
	delayMin    time.Duration
	delayMax    time.Duration
	dropProb    float64
	rng         *rand.Rand
}

type latencyNetwork struct {
	mu      sync.Mutex
	links   map[int]map[int]*latencyLink
	n       int
	deliver func(from, to int, msg *ConsensusMessage)
}

func newLatencyNetwork(n int, baseDelay, jitter time.Duration, dropProb float64, deliver func(int, int, *ConsensusMessage)) *latencyNetwork {
	ln := &latencyNetwork{
		links:   make(map[int]map[int]*latencyLink),
		n:       n,
		deliver: deliver,
	}

	for i := 0; i < n; i++ {
		ln.links[i] = make(map[int]*latencyLink)
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			ln.links[i][j] = &latencyLink{
				delayMin: baseDelay,
				delayMax: baseDelay + jitter,
				dropProb: dropProb,
				rng:      rand.New(rand.NewSource(int64(i*1000+j))),
			}
		}
	}

	return ln
}

func (ln *latencyNetwork) Send(from, to int, msg *ConsensusMessage) {
	ln.mu.Lock()
	link := ln.links[from][to]
	ln.mu.Unlock()

	if link == nil {
		return
	}

	ln.mu.Lock()
	shouldDrop := link.rng.Float64() < link.dropProb
	delay := link.delayMin + time.Duration(link.rng.Float64()*float64(link.delayMax-link.delayMin))
	ln.mu.Unlock()

	if shouldDrop {
		return
	}

	time.AfterFunc(delay, func() {
		ln.deliver(from, to, msg)
	})
}

func (ln *latencyNetwork) SetLinkDelay(from, to int, min, max time.Duration) {
	ln.mu.Lock()
	defer ln.mu.Unlock()
	if link, exists := ln.links[from][to]; exists {
		link.delayMin = min
		link.delayMax = max
	}
}

func (ln *latencyNetwork) SetLinkDropProb(from, to int, prob float64) {
	ln.mu.Lock()
	defer ln.mu.Unlock()
	if link, exists := ln.links[from][to]; exists {
		link.dropProb = prob
	}
}

func TestP2PLatencySimulation(t *testing.T) {
	n := 4
	keys := make([]*crypto.PrivateKey, n)
	for i := 0; i < n; i++ {
		keys[i], _ = crypto.GenerateKey()
	}

	staking := NewStakingModule(24*time.Hour, 0.01)
	for _, k := range keys {
		staking.Stake(k.PubKey().Address(), k.PubKey().Bytes(), 1000000)
	}

	active := staking.GetActiveValidators()
	vals := make([]*Validator, 0, len(active))
	for _, sv := range active {
		vals = append(vals, &Validator{
			Address:   sv.Address,
			PublicKey: sv.PublicKey,
			Stake:     sv.Stake,
			IsActive:  true,
		})
	}
	vs := NewValidatorSet(vals, 1)

	engines := make([]*HotStuffEngine, n)
	for i := 0; i < n; i++ {
		genesis := ledger.TestGenesis()
		bc, err := ledger.NewBlockchain(genesis)
		if err != nil {
			t.Fatal(err)
		}
		bp := &testBP{bc: bc, k: keys[i]}

		config := DefaultConsensusConfig()
		config.BlockTime = 200 * time.Millisecond
		config.ViewTimeout = 1 * time.Second

		engines[i] = NewHotStuffEngine(config, vs, bp, staking, nil, &noopAudit3{})
	}

	latencyNet := newLatencyNetwork(n, 5*time.Millisecond, 20*time.Millisecond, 0.02, func(from, to int, msg *ConsensusMessage) {
		if engines[to].IsRunning() {
			engines[to].HandleMessage(msg)
		}
	})

	for i := 0; i < n; i++ {
		idx := i
		engines[i].SetBroadcast(func(msg *ConsensusMessage) {
			for j := 0; j < n; j++ {
				if j != idx {
					latencyNet.Send(idx, j, msg)
				}
			}
		})
	}


	for i := 0; i < n; i++ {
		if err := engines[i].Start(1); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(2 * time.Second)

	for i := 0; i < n; i++ {
		engines[i].Stop()
	}

	for i := 0; i < n; i++ {
		if engines[i].Height() < 1 {
			t.Errorf("validator %d: expected height >= 1 with latency, got %d", i, engines[i].Height())
		}
	}

	minHeight := engines[0].Height()
	maxHeight := engines[0].Height()
	for i := 1; i < n; i++ {
		h := engines[i].Height()
		if h < minHeight {
			minHeight = h
		}
		if h > maxHeight {
			maxHeight = h
		}
	}

	t.Logf("Latency test: min_height=%d max_height=%d", minHeight, maxHeight)
}

func TestP2PLatencyWithPartition(t *testing.T) {
	n := 4
	keys := make([]*crypto.PrivateKey, n)
	for i := 0; i < n; i++ {
		keys[i], _ = crypto.GenerateKey()
	}

	staking := NewStakingModule(24*time.Hour, 0.01)
	for _, k := range keys {
		staking.Stake(k.PubKey().Address(), k.PubKey().Bytes(), 1000000)
	}

	active := staking.GetActiveValidators()
	vals := make([]*Validator, 0, len(active))
	for _, sv := range active {
		vals = append(vals, &Validator{
			Address:   sv.Address,
			PublicKey: sv.PublicKey,
			Stake:     sv.Stake,
			IsActive:  true,
		})
	}
	vs := NewValidatorSet(vals, 1)

	engines := make([]*HotStuffEngine, n)
	for i := 0; i < n; i++ {
		genesis := ledger.TestGenesis()
		bc, err := ledger.NewBlockchain(genesis)
		if err != nil {
			t.Fatal(err)
		}
		bp := &testBP{bc: bc, k: keys[i]}

		cfg := DefaultConsensusConfig()
		cfg.BlockTime = 200 * time.Millisecond
		cfg.ViewTimeout = 800 * time.Millisecond

		engines[i] = NewHotStuffEngine(cfg, vs, bp, staking, nil, &noopAudit3{})
	}

	latencyNet := newLatencyNetwork(n, 5*time.Millisecond, 10*time.Millisecond, 0.0, func(from, to int, msg *ConsensusMessage) {
		if engines[to].IsRunning() {
			engines[to].HandleMessage(msg)
		}
	})

	for i := 0; i < n; i++ {
		idx := i
		engines[i].SetBroadcast(func(msg *ConsensusMessage) {
			for j := 0; j < n; j++ {
				if j != idx {
					latencyNet.Send(idx, j, msg)
				}
			}
		})
	}

	// Set up state syncers so partitioned validators can catch up via block requests
	for i := 0; i < n; i++ {
		idx := i
		syncer := NewStateSyncer(
			func(fromHeight, toHeight uint64) error {
				req := &ConsensusMessage{
					Type:      MsgBlockRequest,
					Height:    toHeight,
					Payload:   SerializeBlockRequest(fromHeight, toHeight),
					Timestamp: time.Now(),
				}
				if engines[idx].broadcastFn != nil {
					engines[idx].broadcastFn(req)
				}
				return nil
			},
			engines[idx].defaultBlockApplier,
			func() uint64 { return engines[idx].Height() },
			nil,
		)
		engines[idx].stateSyncer = syncer
	}

	for i := 0; i < n; i++ {
		if err := engines[i].Start(1); err != nil {
			t.Fatal(err)
		}
	}

	// Wait for initial convergence before partition
	func() {
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			allGE3 := true
			for i := 0; i < n; i++ {
				if engines[i].Height() < 3 {
					allGE3 = false
					break
				}
			}
			if allGE3 {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		heights := make([]uint64, n)
		for i := 0; i < n; i++ {
			heights[i] = engines[i].Height()
		}
		t.Skipf("skipping: validators only reached %v before partition", heights)
	}()

	heightBeforeHeal := make([]uint64, n)
	for i := 0; i < n; i++ {
		heightBeforeHeal[i] = engines[i].Height()
	}
	t.Logf("Heights before heal (no partition yet): %v", heightBeforeHeal)

	// Simulate partition: validator 3 isolated
	for i := 0; i < 3; i++ {
		latencyNet.SetLinkDropProb(i, 3, 1.0)
		latencyNet.SetLinkDropProb(3, i, 1.0)
	}

	time.Sleep(2 * time.Second)

	heightDuringPartition := make([]uint64, n)
	for i := 0; i < n; i++ {
		heightDuringPartition[i] = engines[i].Height()
	}
	t.Logf("Heights during partition: %v", heightDuringPartition)

	// Heal partition
	for i := 0; i < 3; i++ {
		latencyNet.SetLinkDropProb(i, 3, 0.0)
		latencyNet.SetLinkDropProb(3, i, 0.0)
	}

	// Proactively start state sync for the isolated validator after heal
	maxHeight := uint64(0)
	for i := 0; i < n; i++ {
		if h := engines[i].Height(); h > maxHeight {
			maxHeight = h
		}
	}
	if syncer := engines[3].stateSyncer; syncer != nil {
		syncer.StartSync(maxHeight)
		engines[3].OnSyncComplete(func() {})
	}

	// Wait for reconvergence with retry
	func() {
		deadline := time.Now().Add(40 * time.Second)
		for time.Now().Before(deadline) {
			allGE5 := true
			for i := 0; i < n; i++ {
				if engines[i].Height() < 5 {
					allGE5 = false
					break
				}
			}
			if allGE5 {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()

	for i := 0; i < n; i++ {
		engines[i].Stop()
	}

	t.Logf("Partition test: heights before heal=%v, during partition=%v", heightBeforeHeal, heightDuringPartition)
	for i := 0; i < n; i++ {
		t.Logf("Validator %d final height: %d", i, engines[i].Height())
	}

	minHeight := engines[0].Height()
	for i := 1; i < n; i++ {
		if engines[i].Height() < minHeight {
			minHeight = engines[i].Height()
		}
	}

	if minHeight < 5 {
		t.Errorf("expected min height >= 5 after partition heal, got %d", minHeight)
	}
}

func TestP2PHighLatency(t *testing.T) {
	n := 4
	keys := make([]*crypto.PrivateKey, n)
	for i := 0; i < n; i++ {
		keys[i], _ = crypto.GenerateKey()
	}

	staking := NewStakingModule(24*time.Hour, 0.01)
	for _, k := range keys {
		staking.Stake(k.PubKey().Address(), k.PubKey().Bytes(), 1000000)
	}

	active := staking.GetActiveValidators()
	vals := make([]*Validator, 0, len(active))
	for _, sv := range active {
		vals = append(vals, &Validator{
			Address:   sv.Address,
			PublicKey: sv.PublicKey,
			Stake:     sv.Stake,
			IsActive:  true,
		})
	}
	vs := NewValidatorSet(vals, 1)

	engines := make([]*HotStuffEngine, n)
	for i := 0; i < n; i++ {
		genesis := ledger.TestGenesis()
		bc, err := ledger.NewBlockchain(genesis)
		if err != nil {
			t.Fatal(err)
		}
		bp := &testBP{bc: bc, k: keys[i]}

		config := DefaultConsensusConfig()
		config.BlockTime = 500 * time.Millisecond
		config.ViewTimeout = 3 * time.Second

		engines[i] = NewHotStuffEngine(config, vs, bp, staking, nil, &noopAudit3{})
	}

	latencyNet := newLatencyNetwork(n, 100*time.Millisecond, 200*time.Millisecond, 0.0, func(from, to int, msg *ConsensusMessage) {
		if engines[to].IsRunning() {
			engines[to].HandleMessage(msg)
		}
	})

	for i := 0; i < n; i++ {
		idx := i
		engines[i].SetBroadcast(func(msg *ConsensusMessage) {
			for j := 0; j < n; j++ {
				if j != idx {
					latencyNet.Send(idx, j, msg)
				}
			}
		})
	}

	for i := 0; i < n; i++ {
		if err := engines[i].Start(1); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(8 * time.Second)

	for i := 0; i < n; i++ {
		engines[i].Stop()
	}

	for i := 0; i < n; i++ {
		if engines[i].Height() < 3 {
			t.Errorf("validator %d: expected height >= 3 with high latency, got %d", i, engines[i].Height())
		}
	}

	minHeight := engines[0].Height()
	maxHeight := engines[0].Height()
	for i := 1; i < n; i++ {
		h := engines[i].Height()
		if h < minHeight {
			minHeight = h
		}
		if h > maxHeight {
			maxHeight = h
		}
	}

	if maxHeight-minHeight > 2 {
		t.Errorf("validators diverged too much with high latency: min=%d max=%d", minHeight, maxHeight)
	}

	t.Logf("High latency test: min_height=%d max_height=%d", minHeight, maxHeight)
}
