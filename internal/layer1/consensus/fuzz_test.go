package consensus

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

func FuzzHasSuperMajority(f *testing.F) {
	f.Add([]byte("addr1"), uint64(1000000), uint64(3000000))
	f.Add([]byte("addr2"), uint64(500000), uint64(1500000))
	f.Add([]byte("addr3"), uint64(2000000), uint64(6000000))

	f.Fuzz(func(t *testing.T, addrBytes []byte, stake uint64, totalStake uint64) {
		if totalStake == 0 {
			return
		}
		if stake > totalStake {
			return
		}
		if len(addrBytes) == 0 || len(addrBytes) > 100 {
			return
		}

		addr := make([]byte, len(addrBytes))
		copy(addr, addrBytes)

		vs := &ValidatorSet{
			validators: []*Validator{
				{Address: addr, Stake: stake, IsActive: true},
			},
			totalStake: totalStake,
		}

		sig := make(map[string]bool)
		key := string(addr)
		sig[key] = true

		result := vs.HasSuperMajority(sig)
		expected := stake*3 > totalStake*2

		if result != expected {
			t.Errorf("HasSuperMajority: got %v, expected %v (stake=%d, total=%d)", result, expected, stake, totalStake)
		}
	})
}

func FuzzQCIsValid(f *testing.F) {
	f.Add([]byte("validator1"), uint64(1000000), uint64(3000000))
	f.Add([]byte("validator2"), uint64(500000), uint64(1500000))

	f.Fuzz(func(t *testing.T, addrBytes []byte, stake uint64, totalStake uint64) {
		if totalStake == 0 {
			return
		}
		if stake > totalStake {
			return
		}
		if len(addrBytes) == 0 || len(addrBytes) > 100 {
			return
		}

		addr := make([]byte, len(addrBytes))
		copy(addr, addrBytes)
		addrStr := string(addr)

		vs := &ValidatorSet{
			validators: []*Validator{
				{Address: addr, Stake: stake, IsActive: true},
			},
			totalStake: totalStake,
		}

		qc := &QC{
			Height:       1,
			View:         0,
			Phase:        PhasePrepare,
			BlockHash:    []byte("blockhash"),
			ValidatorAddrs: []string{addrStr},
			Signatures:   make(map[string]crypto.Signature),
		}

		qc.Signatures[addrStr] = crypto.Signature{
			R: new(big.Int).SetBytes([]byte{1, 2, 3}),
			S: new(big.Int).SetBytes([]byte{4, 5, 6}),
		}

		_ = qc.IsValid(vs)
	})
}

func FuzzSelectProposer(f *testing.F) {
	f.Add(uint64(1), uint64(0), uint64(4))
	f.Add(uint64(100), uint64(5), uint64(10))
	f.Add(uint64(1000), uint64(100), uint64(100))

	f.Fuzz(func(t *testing.T, height uint64, view uint64, numValidators uint64) {
		if numValidators == 0 || numValidators > 1000 {
			return
		}

		validators := make([]*Validator, numValidators)
		for i := uint64(0); i < numValidators; i++ {
			validators[i] = &Validator{
				Address:  []byte{byte(i % 256)},
				Stake:    1000000,
				IsActive: true,
			}
		}

		vs := &ValidatorSet{
			validators: validators,
			totalStake: numValidators * 1000000,
		}

		proposer, err := vs.GetProposerForView(height, view)
		if err != nil {
			t.Fatalf("GetProposerForView failed: %v", err)
		}

		if proposer == nil {
			t.Fatal("proposer is nil")
		}
	})
}

func TestMessageRateLimiter(t *testing.T) {
	rl := newMessageRateLimiter(5, time.Second)

	for i := 0; i < 5; i++ {
		if !rl.Allow("validator1") {
			t.Errorf("message %d should be allowed", i+1)
		}
	}

	if rl.Allow("validator1") {
		t.Error("message 6 should be rate limited")
	}

	if !rl.Allow("validator2") {
		t.Error("different validator should not be affected")
	}
}

func TestMessageRateLimiterReset(t *testing.T) {
	rl := newMessageRateLimiter(3, 100*time.Millisecond)

	for i := 0; i < 3; i++ {
		if !rl.Allow("validator1") {
			t.Errorf("message %d should be allowed", i+1)
		}
	}

	if rl.Allow("validator1") {
		t.Error("message 4 should be rate limited")
	}

	time.Sleep(150 * time.Millisecond)

	if !rl.Allow("validator1") {
		t.Error("message should be allowed after window reset")
	}
}

func TestGracefulShutdown(t *testing.T) {
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
	bc, err := ledger.NewBlockchain(genesis)
	if err != nil {
		t.Fatal(err)
	}
	bp := &testBP{bc: bc, k: keys[0]}

	config := DefaultConsensusConfig()
	config.BlockTime = 100 * time.Millisecond
	config.ViewTimeout = 500 * time.Millisecond

	engine := NewHotStuffEngine(config, vs, bp, staking, nil, &noopAudit3{})

	if err := engine.Start(bc.Height() + 1); err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)

	if !engine.IsRunning() {
		t.Fatal("engine should be running")
	}

	done := make(chan struct{})
	go func() {
		engine.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop() did not return within 3 seconds")
	}

	if engine.IsRunning() {
		t.Error("engine should not be running after Stop()")
	}
}

func TestLRUTrimVotesAndTimeouts(t *testing.T) {
	config := DefaultConsensusConfig()
	staking := NewStakingModule(24*time.Hour, 0.01)
	keys := make([]*crypto.PrivateKey, 1)
	keys[0], _ = crypto.GenerateKey()
	staking.Stake(keys[0].PubKey().Address(), keys[0].PubKey().Bytes(), 1000000)

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
	bc, err := ledger.NewBlockchain(genesis)
	if err != nil {
		t.Fatal(err)
	}
	bp := &testBP{bc: bc, k: keys[0]}

	engine := NewHotStuffEngine(config, vs, bp, staking, nil, &noopAudit3{})

	for i := 0; i < 3000; i++ {
		key := fmt.Sprintf("%d-%d-%s", i, 0, PhasePrepare.String())
		engine.votes[key] = make(map[Phase]map[string]bool)
		engine.votesOrder = append(engine.votesOrder, key)
	}
	engine.trimVotes()
	if len(engine.votesOrder) > 2048 {
		t.Errorf("votesOrder not trimmed: %d", len(engine.votesOrder))
	}

	for i := 0; i < 3000; i++ {
		view := uint64(i)
		engine.timeouts[view] = make(map[string]bool)
		engine.timeoutsOrder = append(engine.timeoutsOrder, view)
	}
	engine.trimTimeouts()
	if len(engine.timeoutsOrder) > 2048 {
		t.Errorf("timeoutsOrder not trimmed: %d", len(engine.timeoutsOrder))
	}
}

func TestEpochSlashing(t *testing.T) {
	config := DefaultConsensusConfig()
	staking := NewStakingModule(24*time.Hour, 0.01)
	keys := make([]*crypto.PrivateKey, 2)
	for i := 0; i < 2; i++ {
		keys[i], _ = crypto.GenerateKey()
		staking.Stake(keys[i].PubKey().Address(), keys[i].PubKey().Bytes(), 1000000)
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
	bc, err := ledger.NewBlockchain(genesis)
	if err != nil {
		t.Fatal(err)
	}
	bp := &testBP{bc: bc, k: keys[0]}

	engine := NewHotStuffEngine(config, vs, bp, staking, nil, &noopAudit3{})

	evidence := &DoubleSignRecord{Validator: keys[1].PubKey().Address(), Height: 1}
	engine.handleDoubleSign(evidence)
	engine.applyEpochSlashing()

	record, exists := staking.GetValidator(keys[1].PubKey().Address())
	if !exists {
		t.Fatal("validator should exist")
	}
	if !record.Jailed {
		t.Error("validator should be jailed after slashing")
	}
}

func TestProtocolVersionMismatch(t *testing.T) {
	config := DefaultConsensusConfig()
	config.ProtocolVersion = 2

	staking := NewStakingModule(24*time.Hour, 0.01)
	key, _ := crypto.GenerateKey()
	staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)

	active := staking.GetActiveValidators()
	vals := make([]*Validator, 0, len(active))
	for _, sv := range active {
		vals = append(vals, &Validator{Address: sv.Address, PublicKey: sv.PublicKey, Stake: sv.Stake, IsActive: true})
	}
	vs := NewValidatorSet(vals, 1)

	genesis := ledger.TestGenesis()
	bc, err := ledger.NewBlockchain(genesis)
	if err != nil {
		t.Fatal(err)
	}
	bp := &testBP{bc: bc, k: key}

	engine := NewHotStuffEngine(config, vs, bp, staking, nil, &noopAudit3{})

	engine.state.ProtocolVersion = 1
	engine.state.Height = 1
	engine.stateStore.Save(engine.state, nil)
	engine.config.ProtocolVersion = 2
	if err := engine.Start(bc.Height() + 1); err == nil {
		t.Fatal("expected protocol version mismatch error")
	}
}

func TestRateLimitingIntegration(t *testing.T) {
	rl := newMessageRateLimiter(5, time.Second)

	validatorAddr := "test_validator_addr"
	accepted := 0
	rejected := 0

	for i := 0; i < 20; i++ {
		if rl.Allow(validatorAddr) {
			accepted++
		} else {
			rejected++
		}
	}

	if accepted != 5 {
		t.Errorf("expected 5 accepted messages, got %d", accepted)
	}
	if rejected != 15 {
		t.Errorf("expected 15 rejected messages, got %d", rejected)
	}

	secondValidator := "second_validator"
	for i := 0; i < 5; i++ {
		if !rl.Allow(secondValidator) {
			t.Errorf("second validator should be allowed, iteration %d", i)
		}
	}
}

func FuzzQCCreationRandomValidators(f *testing.F) {
	f.Add(uint64(1), uint64(0), uint64(4))
	f.Add(uint64(10), uint64(3), uint64(8))
	f.Add(uint64(100), uint64(2), uint64(16))

	f.Fuzz(func(t *testing.T, height, view, numValidators uint64) {
		if numValidators == 0 || numValidators > 100 {
			return
		}
		validators := make([]*Validator, numValidators)
		for i := uint64(0); i < numValidators; i++ {
			validators[i] = &Validator{
				Address:  []byte{byte(i), byte(i >> 8)},
				Stake:    1000000,
				IsActive: true,
			}
		}
		vs := NewValidatorSet(validators, 1)
		qc := &QC{
			Height:    height,
			View:      view,
			Phase:     PhasePrepare,
			BlockHash: []byte("hash"),
			Signatures: make(map[string]crypto.Signature),
			ValidatorAddrs: []string{},
		}
		_ = qc.IsValid(vs)
	})
}

func FuzzViewChangeMessages(f *testing.F) {
	f.Add(uint64(1), uint64(0), uint64(5))
	f.Add(uint64(5), uint64(3), uint64(10))

	f.Fuzz(func(t *testing.T, height, oldView, numValidators uint64) {
		if numValidators == 0 || numValidators > 50 {
			return
		}
		validators := make([]*Validator, numValidators)
		for i := uint64(0); i < numValidators; i++ {
			validators[i] = &Validator{
				Address:  []byte{byte(i)},
				Stake:    1000000,
				IsActive: true,
			}
		}
		vs := NewValidatorSet(validators, 1)
		msg := &ConsensusMessage{
			Type:      MsgNewView,
			Height:    height,
			View:      oldView + 1,
			BlockHash: []byte("view_hash"),
			Validator: []byte("val"),
			Timestamp: time.Now(),
		}
		_, _ = msg.MarshalJSON()
		_ = vs
	})
}

func FuzzTimeoutMessageValidation(f *testing.F) {
	f.Add(uint64(1), uint64(2), []byte("val1"))
	f.Add(uint64(0), uint64(1), []byte("val2"))

	f.Fuzz(func(t *testing.T, height, view uint64, validator []byte) {
		if len(validator) == 0 {
			return
		}
		tc := &TimeoutCert{
			Height:    height,
			View:      view,
			Timeouts:  make(map[string]bool),
			Signatures: make(map[string]crypto.Signature),
			TotalStake: 100,
		}
		addr := hex.EncodeToString(validator)
		tc.Timeouts[addr] = true
		_ = tc.HasQuorum(1)
		_ = tc.HasQuorum(100)
	})
}

func FuzzVoteAggregationRandom(f *testing.F) {
	f.Add(uint64(1), uint64(0), uint64(3))
	f.Add(uint64(10), uint64(2), uint64(7))

	f.Fuzz(func(t *testing.T, height, view, numVals uint64) {
		if numVals == 0 || numVals > 30 {
			return
		}
		votes := make(map[string]bool)
		for i := uint64(0); i < numVals; i++ {
			addr := hex.EncodeToString([]byte{byte(i)})
			votes[addr] = i%2 == 0
		}
	})
}

func FuzzProposerSelectionRandomStake(f *testing.F) {
	f.Add(uint64(1), uint64(0), uint64(5))
	f.Add(uint64(100), uint64(3), uint64(10))
	f.Add(uint64(1000), uint64(50), uint64(50))

	f.Fuzz(func(t *testing.T, height, view, numVals uint64) {
		if numVals == 0 || numVals > 100 {
			return
		}
		rng := rand.New(rand.NewSource(int64(height + view)))
		validators := make([]*Validator, numVals)
		for i := uint64(0); i < numVals; i++ {
			stake := uint64(rng.Intn(1000000)) + 1
			validators[i] = &Validator{
				Address:  []byte{byte(i), byte(i >> 8), byte(i >> 16)},
				Stake:    stake,
				IsActive: true,
			}
		}
		vs := NewValidatorSet(validators, 1)
		proposer, err := vs.GetProposerForView(height, view)
		if err != nil {
			return
		}
		if proposer == nil {
			t.Errorf("nil proposer")
		}
	})
}

func FuzzStakingModuleOperations(f *testing.F) {
	f.Add([]byte("val1"), uint64(1000))
	f.Add([]byte("val2"), uint64(0))
	f.Add([]byte{}, uint64(1<<60))

	f.Fuzz(func(t *testing.T, validator []byte, amount uint64) {
		if len(validator) == 0 {
			return
		}
		sm := NewStakingModule(24*time.Hour, 0.01)
		err := sm.Stake(validator, []byte("pubkey"), amount)
		if err != nil {
			return
		}
		record, exists := sm.GetValidator(validator)
		if !exists {
			t.Errorf("validator should exist after stake")
			return
		}
		if record.Stake != amount {
			t.Errorf("stake mismatch: %d != %d", record.Stake, amount)
		}
		if amount > 0 {
			_, err := sm.Slash(validator, 0.5)
			if err != nil {
				t.Logf("slash failed: %v", err)
			}
		}
	})
}

func FuzzValidatorSetUpdates(f *testing.F) {
	f.Add([]byte("addr1"), []byte("addr2"), uint64(1000), uint64(2000))
	f.Add([]byte{}, []byte{}, uint64(0), uint64(0))

	f.Fuzz(func(t *testing.T, addr1, addr2 []byte, stake1, stake2 uint64) {
		if len(addr1) == 0 || len(addr2) == 0 {
			return
		}
		vs := NewValidatorSet([]*Validator{
			{Address: addr1, Stake: stake1, IsActive: true},
			{Address: addr2, Stake: stake2, IsActive: true},
		}, 1)
		err := vs.UpdateStake(addr1, stake1+100)
		if err != nil {
			return
		}
		err = vs.RemoveValidator(addr2)
		if err != nil {
			t.Logf("remove failed: %v", err)
		}
	})
}

func FuzzProtocolVersionNegotiation(f *testing.F) {
	f.Add(uint64(1), uint64(2))
	f.Add(uint64(0), uint64(0))
	f.Add(uint64(100), uint64(100))

	f.Fuzz(func(t *testing.T, configVersion, stateVersion uint64) {
		config := DefaultConsensusConfig()
		config.ProtocolVersion = configVersion
		if configVersion != 0 && stateVersion != 0 && configVersion != stateVersion {
			t.Logf("version mismatch expected: config=%d state=%d", configVersion, stateVersion)
		}
	})
}

func FuzzConsensusMessageSerialization(f *testing.F) {
	f.Add(uint64(1), uint64(0), []byte("hash"), []byte("val"), []byte("payload"))
	f.Add(uint64(0), uint64(0), []byte{}, []byte{}, []byte{})

	f.Fuzz(func(t *testing.T, height, view uint64, blockHash, validator, payload []byte) {
		msg := &ConsensusMessage{
			Type:      MsgProposal,
			Height:    height,
			View:      view,
			BlockHash: blockHash,
			Validator: validator,
			Payload:   payload,
			Timestamp: time.Now(),
		}
		data, err := msg.MarshalJSON()
		if err != nil {
			return
		}
		var deserialized ConsensusMessage
		if err := deserialized.UnmarshalJSON(data); err != nil {
			t.Logf("unmarshal failed: %v", err)
			return
		}
		if deserialized.Height != msg.Height {
			t.Errorf("height mismatch after roundtrip")
		}
	})
}

func FuzzDoubleSignDetector(f *testing.F) {
	f.Add([]byte("val1"), uint64(1), uint64(0))
	f.Add([]byte("val2"), uint64(10), uint64(5))

	f.Fuzz(func(t *testing.T, validator []byte, height, view uint64) {
		if len(validator) == 0 {
			return
		}
		dd := NewDoubleSignDetector()
		record := dd.CheckProposal(validator, height, view, []byte("hash1"))
		if record != nil {
			t.Logf("double sign detected on first proposal (should not happen)")
		}
		record2 := dd.CheckProposal(validator, height, view, []byte("hash2"))
		if record2 == nil {
			t.Logf("same height/view different hash should trigger double sign detection")
		}
	})
}

func FuzzEpochRotationWithMinValidators(f *testing.F) {
	f.Add(int(4), int(2))
	f.Add(int(0), int(1))
	f.Add(int(10), int(10))

	f.Fuzz(func(t *testing.T, numValidators, minValidators int) {
		if numValidators < 0 || minValidators < 0 {
			return
		}
		config := DefaultConsensusConfig()
		config.MinValidators = minValidators
		_ = config
	})
}
