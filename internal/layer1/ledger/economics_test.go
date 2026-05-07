package ledger

import (
	"math/big"
	"testing"
)

func TestEconomicsBlockReward(t *testing.T) {
	econ := NewEconomics(nil)

	reward0 := econ.CalculateBlockReward(0)
	reward1 := econ.CalculateBlockReward(2_100_000)

	if reward0.Cmp(reward1) <= 0 {
		t.Error("Reward should halve after first epoch")
	}

	expectedHalf := new(big.Int).Div(econ.config.BlockReward, big.NewInt(2))
	if reward1.Cmp(expectedHalf) != 0 {
		t.Errorf("Expected half reward %s, got %s", expectedHalf.String(), reward1.String())
	}
}

func TestEconomicsCalculateFees(t *testing.T) {
	econ := NewEconomics(nil)

	txs := []*Transaction{
		{GasLimit: 1000, GasPrice: 10},
		{GasLimit: 2000, GasPrice: 20},
	}

	totalFees, validatorFees, burnFees := econ.CalculateFees(txs)

	expectedTotal := big.NewInt(50_000)
	if totalFees.Cmp(expectedTotal) != 0 {
		t.Errorf("Expected total fees %s, got %s", expectedTotal.String(), totalFees.String())
	}

	expectedValidator := new(big.Int).Div(
		new(big.Int).Mul(totalFees, econ.config.ValidatorShare),
		big.NewInt(100),
	)
	if validatorFees.Cmp(expectedValidator) != 0 {
		t.Errorf("Expected validator fees %s, got %s", expectedValidator.String(), validatorFees.String())
	}

	expectedBurn := new(big.Int).Div(
		new(big.Int).Mul(totalFees, econ.config.BurnShare),
		big.NewInt(100),
	)
	if burnFees.Cmp(expectedBurn) != 0 {
		t.Errorf("Expected burn fees %s, got %s", expectedBurn.String(), burnFees.String())
	}
}

func TestEconomicsProcessBlock(t *testing.T) {
	econ := NewEconomics(nil)

	txs := []*Transaction{
		{GasLimit: 1000, GasPrice: 10},
	}

	result, err := econ.ProcessBlock(txs, 0)
	if err != nil {
		t.Fatalf("ProcessBlock failed: %v", err)
	}

	if result.BlockReward.Cmp(econ.config.BlockReward) != 0 {
		t.Errorf("Block reward mismatch")
	}

	if result.BlockHeight != 0 {
		t.Errorf("Expected block height 0, got %d", result.BlockHeight)
	}
}

func TestEconomicsCirculatingSupply(t *testing.T) {
	econ := NewEconomics(nil)

	initialSupply := econ.CirculatingSupply()
	if initialSupply.Cmp(econ.config.InitialSupply) != 0 {
		t.Errorf("Initial supply mismatch: expected %s, got %s", econ.config.InitialSupply.String(), initialSupply.String())
	}

	txs := []*Transaction{
		{GasLimit: 1000, GasPrice: 10},
	}

	econ.ProcessBlock(txs, 0)

	newSupply := econ.CirculatingSupply()
	if newSupply.Cmp(initialSupply) <= 0 {
		t.Error("Circulating supply should increase after block processing")
	}
}

func TestEconomicsInflationRate(t *testing.T) {
	econ := NewEconomics(nil)

	rate := econ.InflationRate(0)
	if rate.Sign() <= 0 {
		t.Error("Inflation rate should be positive")
	}
}

func TestEconomicsBurned(t *testing.T) {
	econ := NewEconomics(nil)

	if econ.Burned().Sign() != 0 {
		t.Error("Initial burned should be 0")
	}

	txs := []*Transaction{
		{GasLimit: 1000, GasPrice: 10},
	}

	econ.ProcessBlock(txs, 0)

	if econ.Burned().Sign() <= 0 {
		t.Error("Burned should be positive after processing block")
	}
}

func TestEconomicsTotalFees(t *testing.T) {
	econ := NewEconomics(nil)

	if econ.TotalFees().Sign() != 0 {
		t.Error("Initial total fees should be 0")
	}

	txs := []*Transaction{
		{GasLimit: 1000, GasPrice: 10},
	}

	econ.ProcessBlock(txs, 0)

	if econ.TotalFees().Sign() <= 0 {
		t.Error("Total fees should be positive after processing block")
	}
}
