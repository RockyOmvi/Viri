package gas

import (
	"testing"
)

func TestNewGasOracle(t *testing.T) {
	oracle := NewGasOracle(DefaultGasConfig())

	if oracle == nil {
		t.Fatalf("oracle should not be nil")
	}

	if oracle.GetBaseFee() != 1_000_000_000 {
		t.Errorf("expected base fee 1_000_000_000, got %d", oracle.GetBaseFee())
	}
}

func TestDefaultGasConfig(t *testing.T) {
	config := DefaultGasConfig()

	if config.InitialBaseFee != 1_000_000_000 {
		t.Errorf("wrong initial base fee: %d", config.InitialBaseFee)
	}
	if config.TargetGasPerBlock != 15_000_000 {
		t.Errorf("wrong target gas: %d", config.TargetGasPerBlock)
	}
	if config.MaxGasPerBlock != 30_000_000 {
		t.Errorf("wrong max gas: %d", config.MaxGasPerBlock)
	}
}

func TestEstimateGas(t *testing.T) {
	oracle := NewGasOracle(DefaultGasConfig())

	estimate := oracle.EstimateGas(nil)

	if estimate.BaseFee != 1_000_000_000 {
		t.Errorf("wrong base fee: %d", estimate.BaseFee)
	}

	if estimate.PriorityFee == 0 {
		t.Errorf("priority fee should not be zero")
	}

	if estimate.TotalEstimate == 0 {
		t.Errorf("total estimate should not be zero")
	}
}

func TestProcessBlock(t *testing.T) {
	oracle := NewGasOracle(DefaultGasConfig())

	block := BlockGasInfo{
		BlockNumber:  1,
		GasUsed:      20_000_000,
		GasLimit:     30_000_000,
		BaseFee:      1_000_000_000,
		Timestamp:    1000,
		PriorityFees: []uint64{1_500_000_000, 2_000_000_000},
	}

	if err := oracle.ProcessBlock(block); err != nil {
		t.Fatalf("process block failed: %v", err)
	}

	history := oracle.GetGasPriceHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 block in history, got %d", len(history))
	}
}

func TestBaseFeeAdjustment(t *testing.T) {
	config := DefaultGasConfig()
	config.InitialBaseFee = 1_000_000_000
	oracle := NewGasOracle(config)

	initialBaseFee := oracle.GetBaseFee()

	block := BlockGasInfo{
		BlockNumber:  1,
		GasUsed:      25_000_000,
		GasLimit:     30_000_000,
		BaseFee:      initialBaseFee,
		Timestamp:    1000,
		PriorityFees: []uint64{1_500_000_000},
	}

	if err := oracle.ProcessBlock(block); err != nil {
		t.Fatalf("process block failed: %v", err)
	}

	newBaseFee := oracle.GetBaseFee()

	if newBaseFee <= initialBaseFee {
		t.Errorf("base fee should increase when gas used > target, was %d now %d", initialBaseFee, newBaseFee)
	}
}

func TestBaseFeeDecrease(t *testing.T) {
	config := DefaultGasConfig()
	config.InitialBaseFee = 1_000_000_000
	oracle := NewGasOracle(config)

	initialBaseFee := oracle.GetBaseFee()

	block := BlockGasInfo{
		BlockNumber:  1,
		GasUsed:      5_000_000,
		GasLimit:     30_000_000,
		BaseFee:      initialBaseFee,
		Timestamp:    1000,
		PriorityFees: []uint64{500_000_000},
	}

	if err := oracle.ProcessBlock(block); err != nil {
		t.Fatalf("process block failed: %v", err)
	}

	newBaseFee := oracle.GetBaseFee()

	if newBaseFee >= initialBaseFee {
		t.Errorf("base fee should decrease when gas used < target, was %d now %d", initialBaseFee, newBaseFee)
	}
}

func TestBaseFeeMinMax(t *testing.T) {
	config := DefaultGasConfig()
	config.MinBaseFee = 100_000_000
	config.MaxBaseFee = 2_000_000_000
	config.InitialBaseFee = 1_500_000_000
	oracle := NewGasOracle(config)

	for i := 0; i < 100; i++ {
		block := BlockGasInfo{
			BlockNumber:  uint64(i + 1),
			GasUsed:      25_000_000,
			GasLimit:     30_000_000,
			BaseFee:      oracle.GetBaseFee(),
			Timestamp:    1000,
			PriorityFees: []uint64{1_500_000_000},
		}
		_ = oracle.ProcessBlock(block)
	}

	baseFee := oracle.GetBaseFee()
	if baseFee > config.MaxBaseFee {
		t.Errorf("base fee %d exceeds max %d", baseFee, config.MaxBaseFee)
	}

	config2 := DefaultGasConfig()
	config2.MinBaseFee = 500_000_000
	config2.InitialBaseFee = 600_000_000
	oracle2 := NewGasOracle(config2)

	for i := 0; i < 100; i++ {
		block := BlockGasInfo{
			BlockNumber:  uint64(i + 1),
			GasUsed:      1_000_000,
			GasLimit:     30_000_000,
			BaseFee:      oracle2.GetBaseFee(),
			Timestamp:    1000,
			PriorityFees: []uint64{100_000_000},
		}
		_ = oracle2.ProcessBlock(block)
	}

	baseFee2 := oracle2.GetBaseFee()
	if baseFee2 < config2.MinBaseFee {
		t.Errorf("base fee %d below min %d", baseFee2, config2.MinBaseFee)
	}
}

func TestNetworkUtilization(t *testing.T) {
	oracle := NewGasOracle(DefaultGasConfig())

	if utilization := oracle.GetNetworkUtilization(); utilization != 0.0 {
		t.Errorf("expected 0.0 utilization with no blocks, got %f", utilization)
	}

	for i := 0; i < 5; i++ {
		block := BlockGasInfo{
			BlockNumber:  uint64(i + 1),
			GasUsed:      15_000_000,
			GasLimit:     30_000_000,
			BaseFee:      1_000_000_000,
			Timestamp:    1000,
			PriorityFees: []uint64{1_000_000_000},
		}
		_ = oracle.ProcessBlock(block)
	}

	utilization := oracle.GetNetworkUtilization()

	expectedUtilization := 0.5
	if utilization < expectedUtilization-0.01 || utilization > expectedUtilization+0.01 {
		t.Errorf("expected utilization ~0.5, got %f", utilization)
	}
}

func TestPriorityFeePercentiles(t *testing.T) {
	oracle := NewGasOracle(DefaultGasConfig())

	for i := 0; i < 10; i++ {
		fees := make([]uint64, 5)
		for j := range fees {
			fees[j] = uint64(1_000_000_000 + i*100_000_000 + j*50_000_000)
		}

		block := BlockGasInfo{
			BlockNumber:  uint64(i + 1),
			GasUsed:      15_000_000,
			GasLimit:     30_000_000,
			BaseFee:      1_000_000_000,
			Timestamp:    1000,
			PriorityFees: fees,
		}
		_ = oracle.ProcessBlock(block)
	}

	slow := oracle.GetSlowPriorityFee()
	recommended := oracle.GetRecommendedPriorityFee()
	fast := oracle.GetFastPriorityFee()

	if slow >= recommended {
		t.Errorf("slow fee %d should be <= recommended %d", slow, recommended)
	}

	if recommended >= fast {
		t.Errorf("recommended fee %d should be <= fast %d", recommended, fast)
	}
}

func TestBaseFeeTrend(t *testing.T) {
	oracle := NewGasOracle(DefaultGasConfig())

	trend := oracle.GetBaseFeeTrend()
	if trend != "stable" {
		t.Errorf("expected stable trend with no blocks, got %s", trend)
	}

	for i := 0; i < 10; i++ {
		block := BlockGasInfo{
			BlockNumber:  uint64(i + 1),
			GasUsed:      25_000_000,
			GasLimit:     30_000_000,
			BaseFee:      oracle.GetBaseFee(),
			Timestamp:    1000,
			PriorityFees: []uint64{1_500_000_000},
		}
		_ = oracle.ProcessBlock(block)
	}

	trend = oracle.GetBaseFeeTrend()
	if trend != "increasing" {
		t.Errorf("expected increasing trend, got %s", trend)
	}

	oracle.Reset()

	for i := 0; i < 10; i++ {
		block := BlockGasInfo{
			BlockNumber:  uint64(i + 1),
			GasUsed:      1_000_000,
			GasLimit:     30_000_000,
			BaseFee:      oracle.GetBaseFee(),
			Timestamp:    1000,
			PriorityFees: []uint64{100_000_000},
		}
		_ = oracle.ProcessBlock(block)
	}

	trend = oracle.GetBaseFeeTrend()
	if trend != "decreasing" {
		t.Errorf("expected decreasing trend, got %s", trend)
	}
}

func TestExportImportState(t *testing.T) {
	oracle := NewGasOracle(DefaultGasConfig())

	for i := 0; i < 5; i++ {
		block := BlockGasInfo{
			BlockNumber:  uint64(i + 1),
			GasUsed:      15_000_000,
			GasLimit:     30_000_000,
			BaseFee:      oracle.GetBaseFee(),
			Timestamp:    1000,
			PriorityFees: []uint64{1_000_000_000},
		}
		_ = oracle.ProcessBlock(block)
	}

	state, err := oracle.ExportState()
	if err != nil {
		t.Fatalf("export state failed: %v", err)
	}

	if len(state) == 0 {
		t.Errorf("state should not be empty")
	}

	oracle2 := NewGasOracle(DefaultGasConfig())
	if err := oracle2.ImportState(state); err != nil {
		t.Fatalf("import state failed: %v", err)
	}
}

func TestValidateGasParams(t *testing.T) {
	oracle := NewGasOracle(DefaultGasConfig())

	if err := oracle.ValidateGasParams(0, 1_000_000_000, 2_000_000_000, 1_000_000_000); err == nil {
		t.Errorf("zero gas limit should fail validation")
	}

	if err := oracle.ValidateGasParams(31_000_000, 1_000_000_000, 2_000_000_000, 1_000_000_000); err == nil {
		t.Errorf("gas limit exceeding max should fail validation")
	}

	if err := oracle.ValidateGasParams(1_000_000, 50_000_000, 2_000_000_000, 1_000_000_000); err == nil {
		t.Errorf("gas price below minimum should fail validation")
	}

	if err := oracle.ValidateGasParams(1_000_000, 1_500_000_000, 500_000_000, 1_000_000_000); err == nil {
		t.Errorf("max fee below base fee should fail validation")
	}

	if err := oracle.ValidateGasParams(1_000_000, 1_500_000_000, 2_000_000_000, 1_000_000_000); err != nil {
		t.Errorf("valid params should pass validation: %v", err)
	}
}

func TestCalculateMaxFee(t *testing.T) {
	oracle := NewGasOracle(DefaultGasConfig())

	maxFee := oracle.CalculateMaxFee(1_500_000_000, 1.5)

	if maxFee < 1_500_000_000 {
		t.Errorf("max fee should be >= priority fee")
	}

	maxFeeCapped := oracle.CalculateMaxFee(1_500_000_000, 100.0)

	if maxFeeCapped > oracle.maxBaseFee {
		t.Errorf("max fee should be capped at maxBaseFee")
	}
}

func TestReset(t *testing.T) {
	oracle := NewGasOracle(DefaultGasConfig())

	for i := 0; i < 10; i++ {
		block := BlockGasInfo{
			BlockNumber:  uint64(i + 1),
			GasUsed:      25_000_000,
			GasLimit:     30_000_000,
			BaseFee:      oracle.GetBaseFee(),
			Timestamp:    1000,
			PriorityFees: []uint64{1_500_000_000},
		}
		_ = oracle.ProcessBlock(block)
	}

	if len(oracle.GetGasPriceHistory()) == 0 {
		t.Errorf("expected history before reset")
	}

	oracle.Reset()

	if oracle.GetBaseFee() != 1_000_000_000 {
		t.Errorf("base fee should be reset to initial value")
	}

	if len(oracle.GetGasPriceHistory()) != 0 {
		t.Errorf("history should be empty after reset")
	}
}

func TestBlockGasLimitExceeded(t *testing.T) {
	oracle := NewGasOracle(DefaultGasConfig())

	block := BlockGasInfo{
		BlockNumber:  1,
		GasUsed:      35_000_000,
		GasLimit:     35_000_000,
		BaseFee:      1_000_000_000,
		Timestamp:    1000,
		PriorityFees: []uint64{1_500_000_000},
	}

	if err := oracle.ProcessBlock(block); err == nil {
		t.Errorf("block exceeding max gas limit should fail")
	}
}

func TestEstimateGasWithPercentiles(t *testing.T) {
	oracle := NewGasOracle(DefaultGasConfig())

	for i := 0; i < 20; i++ {
		fees := make([]uint64, 10)
		for j := range fees {
			fees[j] = uint64(1_000_000_000 + i*50_000_000 + j*10_000_000)
		}

		block := BlockGasInfo{
			BlockNumber:  uint64(i + 1),
			GasUsed:      15_000_000,
			GasLimit:     30_000_000,
			BaseFee:      oracle.GetBaseFee(),
			Timestamp:    1000,
			PriorityFees: fees,
		}
		_ = oracle.ProcessBlock(block)
	}

	estimate := oracle.EstimateGas([]uint64{10, 25, 50, 75, 90})

	if estimate.PriorityFee == 0 {
		t.Errorf("priority fee should not be zero with sufficient history")
	}

	if estimate.MaxFee == 0 {
		t.Errorf("max fee should not be zero")
	}
}
