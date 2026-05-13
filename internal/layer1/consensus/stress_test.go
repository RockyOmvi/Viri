package consensus

import (
	"encoding/hex"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

func TestStressHundredValidators(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	n := 100
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
	t.Logf("Validator set size: %d, total stake: %d", vs.Size(), vs.TotalStake())

	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	sigs := make(map[string]bool)
	for i := 0; i < n; i++ {
		addr := hex.EncodeToString(keys[i].PubKey().Address())
		sigs[addr] = true
	}

	iterations := 10000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_ = vs.HasSuperMajority(sigs)
	}
	elapsed := time.Since(start)

	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	t.Logf("100-validator HasSuperMajority: %d iterations in %v (%.0f ops/sec)", iterations, elapsed, float64(iterations)/elapsed.Seconds())
	t.Logf("Memory: Alloc=%d MB, TotalAlloc=%d MB, Sys=%d MB",
		(memAfter.Alloc-memBefore.Alloc)/1024/1024,
		memAfter.TotalAlloc/1024/1024,
		memAfter.Sys/1024/1024)

	if !vs.HasSuperMajority(sigs) {
		t.Error("expected supermajority with all validators signing")
	}

	quorum := vs.CalculateQuorumStake()
	if quorum == 0 {
		t.Error("expected non-zero quorum stake")
	}

	_, err := vs.GetProposer(1)
	if err != nil {
		t.Errorf("failed to get proposer: %v", err)
	}
}

func TestStressTwentyValidatorsConvergence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	n := 20
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

	bcs := make([]*ledger.Blockchain, n)
	bps := make([]*testBP, n)
	engines := make([]*HotStuffEngine, n)

	for i := 0; i < n; i++ {
		genesis := ledger.TestGenesis()
		bc, err := ledger.NewBlockchain(genesis)
		if err != nil {
			t.Fatal(err)
		}
		bcs[i] = bc
		bps[i] = &testBP{bc: bc, k: keys[i]}

		config := DefaultConsensusConfig()
		config.BlockTime = 500 * time.Millisecond
		config.ViewTimeout = 3 * time.Second
		config.TimeoutIncrease = 0
		config.MaxViewTimeout = 3 * time.Second
		config.MessageRateLimit = 10000
		config.MessageRateWindow = 2 * time.Second

		engines[i] = NewHotStuffEngine(config, vs, bps[i], staking, nil, &noopAudit3{})
	}

	for i := 0; i < n; i++ {
		idx := i
		engines[i].SetBroadcast(func(msg *ConsensusMessage) {
			go func() {
				for j := 0; j < n; j++ {
					if j != idx {
						if engines[j].IsRunning() {
							engines[j].HandleMessage(msg)
						}
					}
				}
			}()
		})
	}

	for i := 0; i < n; i++ {
		if err := engines[i].Start(bcs[i].Height() + 1); err != nil {
			t.Fatal(err)
		}
	}

	time.Sleep(1 * time.Second)

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		heights := make([]uint64, n)
		for i := 0; i < n; i++ {
			heights[i] = bcs[i].Height()
		}
		sort.Slice(heights, func(i, j int) bool {
			return heights[i] < heights[j]
		})
		if heights[0] >= 2 && heights[n-1]-heights[0] <= 3 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	heights := make([]uint64, n)
	for i := 0; i < n; i++ {
		engines[i].Stop()
		heights[i] = bcs[i].Height()
	}

	sort.Slice(heights, func(i, j int) bool {
		return heights[i] < heights[j]
	})
	minHeight := heights[0]
	maxHeight := heights[n-1]

	t.Logf("%d-validator test: min_height=%d max_height=%d spread=%d", n, minHeight, maxHeight, maxHeight-minHeight)

	if minHeight < 2 {
		t.Errorf("expected min height >= 2 with %d validators, got %d", n, minHeight)
	}

	if maxHeight-minHeight > 3 {
		t.Errorf("validators diverged too much: min=%d max=%d", minHeight, maxHeight)
	}
}

func TestStressMessageThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

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

	genesis := ledger.TestGenesis()
	bcs := make([]*ledger.Blockchain, n)
	bps := make([]*testBP, n)
	engines := make([]*HotStuffEngine, n)
	msgCounts := make([]int, n)
	var mu sync.Mutex

	for i := 0; i < n; i++ {
		bc, err := ledger.NewBlockchain(genesis)
		if err != nil {
			t.Fatal(err)
		}
		bcs[i] = bc
		bps[i] = &testBP{bc: bc, k: keys[i]}

		config := DefaultConsensusConfig()
		config.BlockTime = 500 * time.Millisecond
		config.ViewTimeout = 3 * time.Second
		config.MessageRateLimit = 5000
		config.MessageRateWindow = 2 * time.Second

		engines[i] = NewHotStuffEngine(config, vs, bps[i], staking, nil, &noopAudit3{})
	}

	for i := 0; i < n; i++ {
		idx := i
		engines[i].SetBroadcast(func(msg *ConsensusMessage) {
			mu.Lock()
			msgCounts[idx]++
			mu.Unlock()
			for j := 0; j < n; j++ {
				if j != idx {
					if engines[j].IsRunning() {
						j := j
						go engines[j].HandleMessage(msg)
					}
				}
			}
		})
	}

	for i := 0; i < n; i++ {
		if err := engines[i].Start(bcs[i].Height() + 1); err != nil {
			t.Fatal(err)
		}
	}

	start := time.Now()
	time.Sleep(4 * time.Second)
	elapsed := time.Since(start)

	for i := 0; i < n; i++ {
		engines[i].Stop()
	}

	totalMsgs := 0
	for _, c := range msgCounts {
		totalMsgs += c
	}

	msgsPerSec := float64(totalMsgs) / elapsed.Seconds()
	blocksPerSec := float64(bcs[0].Height()) / elapsed.Seconds()

	t.Logf("Throughput: %d total messages, %.0f msgs/sec, %.2f blocks/sec", totalMsgs, msgsPerSec, blocksPerSec)

	for i := 0; i < n; i++ {
		if bcs[i].Height() < 2 {
			t.Errorf("validator %d: expected height >= 2, got %d", i, bcs[i].Height())
		}
	}
}
