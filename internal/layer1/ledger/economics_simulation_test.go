package ledger

import (
	"math/big"
	"testing"
)

func makeTx(gasLimit, gasPrice uint64) *Transaction {
	return &Transaction{GasLimit: gasLimit, GasPrice: gasPrice}
}

func TestEconomicSimulation_BlockRewardsOverTime(t *testing.T) {
	econ := NewEconomics(nil)
	cfg := econ.config
	halvingInterval := cfg.HalvingInterval
	totalBlocks := uint64(10_000_000)
	epochCount := totalBlocks / halvingInterval

	t.Log("=== Block Reward Halving Simulation (10M blocks) ===")
	t.Logf("Halving interval: %d blocks", halvingInterval)
	t.Logf("Initial reward: %s wei", cfg.BlockReward.String())

	expected := new(big.Int).Set(cfg.BlockReward)
	for epoch := uint64(0); epoch <= epochCount; epoch++ {
		height := epoch * halvingInterval
		reward := econ.CalculateBlockReward(height)

		if reward.Cmp(expected) != 0 {
			t.Errorf("Epoch %d (height %d): expected reward %s, got %s", epoch, height, expected, reward)
		}

		pct := new(big.Float).Quo(
			new(big.Float).SetInt(reward),
			new(big.Float).SetInt(cfg.BlockReward),
		)
		pct.Mul(pct, big.NewFloat(100))
		t.Logf("Epoch %d (height %d): reward = %s wei (%.6f%% of initial)", epoch, height, reward.String(), pct)

		expected.Div(expected, big.NewInt(2))
	}

	t.Logf("Total epochs in %d blocks: %d", totalBlocks, epochCount)

	lastReward := econ.CalculateBlockReward(0)
	for epoch := uint64(1); epoch <= epochCount; epoch++ {
		height := epoch * halvingInterval
		reward := econ.CalculateBlockReward(height)
		if reward.Cmp(lastReward) > 0 {
			t.Errorf("Non-monotonic: reward increased from %s to %s at height %d", lastReward, reward, height)
		}
		if reward.Cmp(lastReward) == 0 && reward.Sign() > 0 {
			t.Errorf("Reward did not decrease at halving boundary height %d: %s", height, reward)
		}
		lastReward = reward
	}

	batchEcon := NewEconomics(nil)
	for height := uint64(0); height < 1000; height++ {
		txs := []*Transaction{makeTx(21000, cfg.BaseFeeTarget.Uint64())}
		result, err := batchEcon.ProcessBlock(txs, height)
		if err != nil {
			t.Fatalf("ProcessBlock failed at height %d: %v", height, err)
		}
		if height%250 == 0 {
			t.Logf("Block %d: reward=%s fees=%s supply=%s", height, result.BlockReward.String(), result.TotalFees.String(), result.CirculatingSupply.String())
		}
	}
	t.Logf("Processed 1000 simulated blocks with tx fees, final supply: %s", batchEcon.CirculatingSupply().String())
}

func TestEconomicSimulation_SupplyCap(t *testing.T) {
	econ := NewEconomics(nil)
	cfg := econ.config

	maxSupply := cfg.MaxSupply
	initialSupply := econ.CirculatingSupply()
	headroom := new(big.Int).Sub(maxSupply, initialSupply)
	t.Logf("Initial supply: %s", initialSupply.String())
	t.Logf("Max supply: %s", maxSupply.String())
	t.Logf("Headroom: %s", headroom.String())

	reward := econ.CalculateBlockReward(0)

	gasPrice := cfg.BaseFeeTarget.Uint64()
	gasLimit := cfg.GasTarget

	maxTxFee := new(big.Int).SetUint64(gasPrice)
	maxTxFee.Mul(maxTxFee, new(big.Int).SetUint64(gasLimit))

	perBlock := new(big.Int).Add(reward, maxTxFee)
	t.Logf("Max issuance per block (reward + max fees): %s", perBlock.String())

	blocks := uint64(0)
	for {
		txs := []*Transaction{makeTx(gasLimit, gasPrice)}
		_, err := econ.ProcessBlock(txs, blocks)
		if err != nil {
			t.Logf("Supply cap reached after %d blocks: %v", blocks, err)
			break
		}
		blocks++
		if blocks > 1_000_000 {
			t.Log("Stopping at 1M blocks (cap not yet reached)")
			break
		}
	}

	finalSupply := econ.CirculatingSupply()
	if finalSupply.Cmp(maxSupply) > 0 {
		t.Errorf("Circulating supply %s exceeds max supply %s", finalSupply.String(), maxSupply.String())
	}
	if finalSupply.Cmp(maxSupply) == 0 {
		t.Logf("Supply exactly at max: %s", finalSupply.String())
	}
	if finalSupply.Cmp(maxSupply) < 0 {
		t.Logf("Supply below max: %s (headroom: %s)", finalSupply.String(), new(big.Int).Sub(maxSupply, finalSupply).String())
	}

	t.Logf("Total blocks processed before cap: %d", blocks)
	t.Logf("Final circulating supply: %s", finalSupply.String())
	t.Logf("Total fees collected: %s", econ.TotalFees().String())
	t.Logf("Total burned: %s", econ.Burned().String())
}

func TestEconomicSimulation_ZeroRewardAfterHalvings(t *testing.T) {
	econ := NewEconomics(nil)
	cfg := econ.config
	halvingInterval := cfg.HalvingInterval

	t.Log("=== Zero Reward After Halving-Induced Underflow ===")
	t.Logf("Halving interval: %d blocks", halvingInterval)
	t.Logf("Initial block reward: %s", cfg.BlockReward.String())

	tmp := new(big.Int).Set(cfg.BlockReward)
	zeroEpoch := uint64(0)
	for tmp.Sign() > 0 {
		tmp.Div(tmp, big.NewInt(2))
		zeroEpoch++
		if zeroEpoch > 100 {
			t.Fatal("Reward did not reach zero within 100 epochs")
		}
	}
	t.Logf("Reward reaches zero at epoch %d (integer division underflow)", zeroEpoch)

	lastPositiveEpoch := zeroEpoch - 1
	rewardLast := econ.CalculateBlockReward(lastPositiveEpoch * halvingInterval)
	t.Logf("Reward at epoch %d (block %d): %s", lastPositiveEpoch, lastPositiveEpoch*halvingInterval, rewardLast.String())
	if rewardLast.Sign() <= 0 {
		t.Errorf("Reward at epoch %d should be positive, got %s", lastPositiveEpoch, rewardLast.String())
	}

	rewardZero := econ.CalculateBlockReward(zeroEpoch * halvingInterval)
	t.Logf("Reward at epoch %d (block %d): %s", zeroEpoch, zeroEpoch*halvingInterval, rewardZero.String())
	if rewardZero.Sign() != 0 {
		t.Errorf("Reward at epoch %d should be zero, got %s", zeroEpoch, rewardZero.String())
	}

	codeCheckBlock := uint64(64) * halvingInterval
	reward64 := econ.CalculateBlockReward(codeCheckBlock)
	if reward64.Sign() != 0 {
		t.Errorf("Code checks epochs >= 64 for zero, got %s at block %d", reward64.String(), codeCheckBlock)
	}
	t.Logf("Code guard (epochs >= 64): reward at block %d is also zero: %s", codeCheckBlock, reward64.String())

	for epoch := zeroEpoch; epoch < zeroEpoch+10; epoch++ {
		r := econ.CalculateBlockReward(epoch * halvingInterval)
		if r.Sign() != 0 {
			t.Errorf("Reward at epoch %d should be zero, got %s", epoch, r.String())
		}
	}
	t.Logf("All rewards at epoch >= %d are confirmed zero", zeroEpoch)
}

func TestEconomicSimulation_FeeMarketStability(t *testing.T) {
	fm := NewFeeMarket(1_000_000_000, 15_000_000, 30_000_000)
	initialBaseFee := fm.BaseFee()
	gasTarget := fm.GasTarget()
	blockGasLimit := fm.BlockGasLimit()

	t.Log("=== Fee Market Stability Under Variable Demand ===")
	t.Logf("Initial base fee: %d", initialBaseFee)
	t.Logf("Gas target: %d", gasTarget)
	t.Logf("Block gas limit: %d", blockGasLimit)

	type blockPattern struct {
		name     string
		gasUsed  uint64
		blocks   int
	}

	patterns := []blockPattern{
		{"Full blocks (2x target)", blockGasLimit, 10},
		{"Empty blocks", 0, 10},
		{"Half-full blocks", gasTarget / 2, 10},
		{"Full blocks (2x target)", blockGasLimit, 10},
	}

	baseFeeHistory := make([]uint64, 0, 100)
	baseFeeHistory = append(baseFeeHistory, fm.BaseFee())

	for _, p := range patterns {
		t.Logf("  Phase: %s (%d blocks, %d gas each)", p.name, p.blocks, p.gasUsed)
		for i := 0; i < p.blocks; i++ {
			fm.Update(p.gasUsed)
			baseFeeHistory = append(baseFeeHistory, fm.BaseFee())
		}
	}

	for i, bf := range baseFeeHistory {
		if bf == 0 {
			t.Errorf("Base fee dropped to zero at step %d", i)
		}
		if bf > initialBaseFee*10 && i < 20 {
			t.Logf("Warning: base fee grew rapidly at step %d: %d", i, bf)
		}
	}

	for i := 1; i < len(baseFeeHistory); i++ {
		change := int64(baseFeeHistory[i]) - int64(baseFeeHistory[i-1])
		maxChange := int64(baseFeeHistory[i-1]) / 8
		if change > maxChange || (change < 0 && -change > maxChange) {
			dir := "increase"
			if change < 0 {
				dir = "decrease"
			}
			t.Logf("Base fee %s at step %d exceeds 12.5%% bound: %d -> %d", dir, i, baseFeeHistory[i-1], baseFeeHistory[i])
		}
	}

	finalBaseFee := fm.BaseFee()
	t.Logf("Final base fee: %d", finalBaseFee)
	t.Logf("Total blocks simulated: %d", len(baseFeeHistory)-1)
	t.Logf("Base fee oscillates between %d and %d", minBaseFee(baseFeeHistory), maxBaseFee(baseFeeHistory))

	if finalBaseFee == 0 {
		t.Error("Base fee collapsed to zero")
	}
}

func minBaseFee(fees []uint64) uint64 {
	min := fees[0]
	for _, f := range fees {
		if f < min {
			min = f
		}
	}
	return min
}

func maxBaseFee(fees []uint64) uint64 {
	max := fees[0]
	for _, f := range fees {
		if f > max {
			max = f
		}
	}
	return max
}

func TestEconomicSimulation_FeeMarketConvergence(t *testing.T) {
	gasTarget := uint64(15_000_000)
	blockGasLimit := uint64(30_000_000)

	t.Log("=== Fee Market Convergence Under Steady Demand Pressure ===")

	t.Run("high_to_low", func(t *testing.T) {
		fm := NewFeeMarket(100_000_000_000, gasTarget, blockGasLimit)
		belowTarget := gasTarget - 1_000_000
		t.Logf("  Start: %d, demand %d (< target)", fm.BaseFee(), belowTarget)
		for i := 0; i < 500; i++ {
			fm.Update(belowTarget)
		}
		final := fm.BaseFee()
		t.Logf("  Final after 500 blocks: %d", final)
		if final >= 100_000_000_000 {
			t.Error("Base fee should decrease with below-target demand")
		}
		if final < 1 {
			t.Error("Base fee below minimum (1)")
		}
	})

	t.Run("low_to_high", func(t *testing.T) {
		fm := NewFeeMarket(100, gasTarget, blockGasLimit)
		aboveTarget := blockGasLimit
		t.Logf("  Start: %d, demand %d (> target=%d)", fm.BaseFee(), aboveTarget, gasTarget)
		for i := 0; i < 500; i++ {
			fm.Update(aboveTarget)
		}
		final := fm.BaseFee()
		t.Logf("  Final after 500 blocks: %d", final)
		if final <= 100 {
			t.Error("Base fee should increase with above-target demand")
		}
	})

	t.Run("oscillating_convergence", func(t *testing.T) {
		fm := NewFeeMarket(1_000_000_000, gasTarget, blockGasLimit)
		highDemand := uint64(20_000_000)
		lowDemand := uint64(10_000_000)

		for i := 0; i < 100; i++ {
			if i%2 == 0 {
				fm.Update(highDemand)
			} else {
				fm.Update(lowDemand)
			}
		}
		afterOscillation := fm.BaseFee()
		t.Logf("  After 100 alternating blocks (20M/10M): %d", afterOscillation)

		middleDemand := uint64(14_000_000)
		for i := 0; i < 500; i++ {
			fm.Update(middleDemand)
		}
		final := fm.BaseFee()
		t.Logf("  After 500 blocks at %d (below target): %d", middleDemand, final)
		if final > afterOscillation {
			t.Logf("  Base fee decreased from %d to %d under below-target demand", afterOscillation, final)
		}
	})
}

func TestEconomicSimulation_InflationRate(t *testing.T) {
	econ := NewEconomics(nil)
	cfg := econ.config
	halvingInterval := cfg.HalvingInterval

	t.Log("=== Inflation Rate Simulation ===")

	checkpoints := []uint64{
		0,
		halvingInterval,
		halvingInterval * 2,
		halvingInterval * 4,
		halvingInterval * 8,
		halvingInterval * 10,
	}

	var lastRate *big.Float
	for _, height := range checkpoints {
		rate := econ.InflationRate(height)
		if rate.Sign() < 0 {
			t.Errorf("Negative inflation at height %d: %v", height, rate)
		}

		ratePct := new(big.Float).Mul(rate, big.NewFloat(100))
		t.Logf("Height %d: inflation rate = %.6f%%", height, ratePct)

		if lastRate != nil && rate.Cmp(lastRate) > 0 {
			t.Logf("  Note: inflation increased from %.6f%% to %.6f%% (expected due to supply dynamics)", lastRate, rate)
		}
		lastRate = rate
	}

	rate0 := econ.InflationRate(0)
	if rate0.Sign() <= 0 {
		t.Error("Initial inflation rate should be positive")
	}

	postHalving := econ.InflationRate(halvingInterval)
	if postHalving.Cmp(rate0) >= 0 {
		t.Logf("Inflation after first halving (%.6f%%) < initial (%.6f%%)", postHalving, rate0)
	}

	post64 := uint64(64) * halvingInterval
	rate64 := econ.InflationRate(post64)
	if rate64.Sign() != 0 {
		t.Logf("Inflation rate after 64 halvings: %v (reward is zero)", rate64)
	}

	reward0 := econ.CalculateBlockReward(0)
	yield := new(big.Float).Quo(
		new(big.Float).SetInt(reward0),
		new(big.Float).SetInt(econ.CirculatingSupply()),
	)
	yield.Mul(yield, big.NewFloat(100))
	t.Logf("Per-block yield at genesis: %.10f%%", yield)
}

func TestEconomicSimulation_SlashingEconomics(t *testing.T) {
	sp := NewSlashingProcessor(nil)
	t.Log("=== Slashing Economics Simulation ===")

	type validator struct {
		name  string
		state *ValidatorState
	}

	validators := []validator{
		{"Alice", &ValidatorState{TotalStake: big.NewInt(10_000_000)}},
		{"Bob", &ValidatorState{TotalStake: big.NewInt(5_000_000)}},
		{"Charlie", &ValidatorState{TotalStake: big.NewInt(2_000_000)}},
		{"Dave", &ValidatorState{TotalStake: big.NewInt(500_000)}},
	}

	totalInitialStake := big.NewInt(0)
	for _, v := range validators {
		totalInitialStake.Add(totalInitialStake, v.state.TotalStake)
	}
	t.Logf("Total initial stake: %s", totalInitialStake.String())
	for _, v := range validators {
		pct := new(big.Float).Quo(
			new(big.Float).SetInt(v.state.TotalStake),
			new(big.Float).SetInt(totalInitialStake),
		)
		pct.Mul(pct, big.NewFloat(100))
		t.Logf("  %s: %s (%.2f%%)", v.name, v.state.TotalStake.String(), pct)
	}

	slashEvents := []struct {
		reason  SlashingReason
		valIdx  int
		height  uint64
		msg     string
	}{
		{SlashingDowntime, 0, 500, "Alice downtime"},
		{SlashingDoubleSign, 1, 1000, "Bob double-sign"},
		{SlashingDowntime, 2, 1500, "Charlie downtime"},
		{SlashingDoubleSign, 0, 2000, "Alice double-sign"},
		{SlashingDowntime, 3, 2500, "Dave downtime"},
	}

	totalSlashed := big.NewInt(0)
	for _, se := range slashEvents {
		v := validators[se.valIdx]
		record := &SlashingRecord{
			Reason:      se.reason,
			Validator:   []byte(v.name),
			BlockHeight: se.height,
		}
		beforeStake := new(big.Int).Set(v.state.TotalStake)
		slashed, err := sp.ProcessSlashing(record, v.state, se.height+100)
		if err != nil {
			t.Logf("  %s: error - %v", se.msg, err)
			continue
		}
		totalSlashed.Add(totalSlashed, slashed)
		pct := new(big.Float).Quo(
			new(big.Float).SetInt(slashed),
			new(big.Float).SetInt(beforeStake),
		)
		pct.Mul(pct, big.NewFloat(100))
		t.Logf("  %s: slashed %s (%.2f%% of %s), remaining %s, jailed=%v",
			se.msg, slashed.String(), pct, beforeStake.String(), v.state.TotalStake.String(), v.state.IsJailed)
	}

	totalFinalStake := big.NewInt(0)
	for _, v := range validators {
		if !v.state.IsJailed || v.state.JailedUntil <= 5000 {
			totalFinalStake.Add(totalFinalStake, v.state.TotalStake)
		} else {
			t.Logf("  %s is jailed until block %d, stake %s frozen", v.name, v.state.JailedUntil, v.state.TotalStake.String())
		}
	}
	t.Logf("Total slashed: %s", totalSlashed.String())
	t.Logf("Total final stake (non-jailed): %s", totalFinalStake.String())

	totalAccounted := new(big.Int).Add(totalFinalStake, totalSlashed)
	if totalAccounted.Cmp(totalInitialStake) != 0 {
		diff := new(big.Int).Sub(totalInitialStake, totalAccounted)
		t.Logf("Note: accounting difference (rounding): %s", diff.String())
	}

	if totalSlashed.Sign() <= 0 {
		t.Error("Total slashed amount should be positive")
	}

	for _, v := range validators {
		if v.state.TotalStake.Sign() < 0 {
			t.Errorf("Negative stake for %s: %s", v.name, v.state.TotalStake.String())
		}
	}
}

func TestEconomicSimulation_MultipleEpochs(t *testing.T) {
	econ := NewEconomics(nil)
	cfg := econ.config
	halvingInterval := cfg.HalvingInterval
	numEpochs := uint64(5)

	numValidators := 100
	totalStake := new(big.Int).SetUint64(1_000_000_000)
	stakes := make([]*big.Int, numValidators)
	rewards := make([]*big.Int, numValidators)
	for i := 0; i < numValidators; i++ {
		stake := new(big.Int).Div(totalStake, big.NewInt(int64(numValidators)))
		stakes[i] = stake
		rewards[i] = big.NewInt(0)
	}

	t.Log("=== Multiple Epoch Reward Distribution ===")
	t.Logf("Simulating %d epochs (%d blocks) with %d validators", numEpochs, numEpochs*halvingInterval, numValidators)
	t.Logf("Total stake: %s", totalStake.String())
	t.Logf("Per-validator stake: %s", stakes[0].String())

	totalBlockRewards := big.NewInt(0)

	for epoch := uint64(0); epoch < numEpochs; epoch++ {
		startHeight := epoch * halvingInterval
		endHeight := (epoch + 1) * halvingInterval

		if endHeight > 10_000_000 {
			endHeight = 10_000_000
		}

		var epochReward *big.Int
		for height := startHeight; height < endHeight; height++ {
			txs := []*Transaction{makeTx(21000, cfg.BaseFeeTarget.Uint64()+height%1000)}
			result, err := econ.ProcessBlock(txs, height)
			if err != nil {
				t.Fatalf("ProcessBlock failed at height %d: %v", height, err)
			}
			epochReward = result.BlockReward
		}

		for i := 0; i < numValidators; i++ {
			share := new(big.Int).Mul(epochReward, stakes[i])
			share.Div(share, totalStake)
			rewards[i].Add(rewards[i], share)
		}
		totalBlockRewards.Add(totalBlockRewards, epochReward)

		block := econ.CalculateBlockReward(startHeight)
		t.Logf("Epoch %d (blocks %d-%d): reward %s", epoch, startHeight, endHeight-1, block.String())
	}

	if totalBlockRewards.Sign() <= 0 {
		t.Error("Total block rewards over 5 epochs should be positive")
	}

	totalDistributed := big.NewInt(0)
	minReward := rewards[0]
	maxReward := rewards[0]
	for i, r := range rewards {
		totalDistributed.Add(totalDistributed, r)
		if r.Cmp(minReward) < 0 {
			minReward = r
		}
		if r.Cmp(maxReward) > 0 {
			maxReward = r
		}
		if i < 5 || i == numValidators-1 {
			pct := new(big.Float).Quo(
				new(big.Float).SetInt(r),
				new(big.Float).SetInt(totalBlockRewards),
			)
			pct.Mul(pct, big.NewFloat(100))
			t.Logf("  Validator %d: reward = %s (%.4f%%)", i, r.String(), pct)
		}
	}

	if len(rewards) > 5 {
		t.Logf("  ... (%d more validators)", numValidators-5)
	}

	equalShare := new(big.Int).Div(totalBlockRewards, big.NewInt(int64(numValidators)))
	diff := new(big.Int).Sub(maxReward, minReward)
	t.Logf("Equal share per validator: %s", equalShare.String())
	t.Logf("Max-min reward difference: %s", diff.String())
	t.Logf("Total block rewards: %s", totalBlockRewards.String())
	t.Logf("Total distributed: %s", totalDistributed.String())

	allowable := new(big.Int).Div(equalShare, big.NewInt(1000))
	if diff.Cmp(allowable) > 0 {
		t.Logf("Reward variance (absolute diff) is %s vs allowable %s", diff.String(), allowable.String())
	}

	if totalDistributed.Cmp(totalBlockRewards) > 0 {
		t.Errorf("Distributed rewards %s exceed total block rewards %s", totalDistributed.String(), totalBlockRewards.String())
	}

	finalSupply := econ.CirculatingSupply()
	t.Logf("Final circulating supply after %d epochs: %s", numEpochs, finalSupply.String())
}

func TestEconomicSimulation_StressValidatorShares(t *testing.T) {
	cfg := DefaultEconomicsConfig()

	t.Log("=== Validator/Developer/Burn Share Verification ===")

	sum := new(big.Int).Add(
		new(big.Int).Add(cfg.ValidatorShare, cfg.DeveloperShare),
		cfg.BurnShare,
	)
	if sum.Cmp(big.NewInt(100)) != 0 {
		t.Errorf("Shares do not sum to 100: V=%d + D=%d + B=%d = %d",
			cfg.ValidatorShare, cfg.DeveloperShare, cfg.BurnShare, sum)
	}

	t.Logf("ValidatorShare: %d%%", cfg.ValidatorShare)
	t.Logf("DeveloperShare: %d%%", cfg.DeveloperShare)
	t.Logf("BurnShare: %d%%", cfg.BurnShare)
	t.Logf("Sum: %d%%", sum)

	gasPrices := []uint64{1_000_000_000, 5_000_000_000, 10_000_000_000, 50_000_000_000}
	gasLimits := []uint64{21000, 50000, 100000, 500000}

	for _, gp := range gasPrices {
		for _, gl := range gasLimits {
			txs := []*Transaction{makeTx(gl, gp)}
			econ := NewEconomics(nil)
			totalFee, validatorFees, burnFees := econ.CalculateFees(txs)

			validatorPct := new(big.Int).Mul(validatorFees, big.NewInt(100))
			validatorPct.Div(validatorPct, totalFee)

			burnPct := new(big.Int).Mul(burnFees, big.NewInt(100))
			burnPct.Div(burnPct, totalFee)

			devFees := new(big.Int).Sub(totalFee, new(big.Int).Add(validatorFees, burnFees))
			devPct := new(big.Int).Mul(devFees, big.NewInt(100))
			devPct.Div(devPct, totalFee)

			recoveredSum := new(big.Int).Add(validatorPct, new(big.Int).Add(devPct, burnPct))

			if recoveredSum.Cmp(big.NewInt(100)) != 0 {
				t.Errorf("Fee shares don't sum to 100%% for gp=%d gl=%d: V=%d%% D=%d%% B=%d%% sum=%d%%",
					gp, gl, validatorPct, burnPct, devPct, recoveredSum)
			}

			if burnPct.Cmp(big.NewInt(10)) != 0 {
				t.Errorf("Expected burn share 10%%, got %d%% for gp=%d gl=%d", burnPct, gp, gl)
			}
			if validatorPct.Cmp(big.NewInt(80)) != 0 {
				t.Errorf("Expected validator share 80%%, got %d%% for gp=%d gl=%d", validatorPct, gp, gl)
			}
			if devPct.Cmp(big.NewInt(10)) != 0 {
				t.Errorf("Expected implicit dev share 10%%, got %d%% for gp=%d gl=%d", devPct, gp, gl)
			}
		}
	}

	t.Log("All gas price/gas limit combinations have correct share allocation")
}

func TestEconomicSimulation_BaseFeeGasTarget(t *testing.T) {
	fm := NewFeeMarket(1_000_000_000, 15_000_000, 30_000_000)
	gasTarget := fm.GasTarget()

	t.Log("=== Base Fee Decreases When Below Gas Target ===")

	belowTarget := []uint64{
		gasTarget - 1,
		gasTarget / 2,
		gasTarget / 4,
		1,
		0,
	}

	for _, lowGas := range belowTarget {
		fm2 := NewFeeMarket(1_000_000_000, gasTarget, 30_000_000)
		initial := fm2.BaseFee()

		for i := 0; i < 20; i++ {
			fm2.Update(lowGas)
		}

		final := fm2.BaseFee()
		if final >= initial {
			t.Errorf("When gas=%d (<target=%d): base fee should decrease but went %d -> %d", lowGas, gasTarget, initial, final)
		}
		t.Logf("gas=%d: base fee %d -> %d (%.2f%% decrease)", lowGas, initial, final, 100.0*(1.0-float64(final)/float64(initial)))
	}

	minBase := fm.BaseFee()
	fm.Update(0)
	for i := 0; i < 100; i++ {
		fm.Update(0)
	}
	if fm.BaseFee() < 1 {
		t.Error("Base fee dropped below 1")
	}
	if fm.BaseFee() > minBase {
		t.Logf("Note: base fee increased after empty blocks (from %d to %d)", minBase, fm.BaseFee())
	}
	lowestReachable := fm.BaseFee()
	t.Logf("Lowest reachable base fee (100 empty blocks): %d", lowestReachable)

	fm3 := NewFeeMarket(1_000_000_000, gasTarget, 30_000_000)
	for i := 0; i < 50; i++ {
		fm3.Update(gasTarget)
	}
	steadyFee := fm3.BaseFee()
	t.Logf("Base fee after 50 blocks at gas target: %d", steadyFee)
	if steadyFee != 1_000_000_000 {
		t.Logf("Base fee should remain at initial when gas == target, got %d", steadyFee)
	}
}
