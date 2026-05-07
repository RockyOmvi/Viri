package ledger

import (
	"math"
	"testing"
)

func TestFeeMarket_BaseFeeAdjustmentUpward(t *testing.T) {
	fm := NewFeeMarket(1000000000, 15000000, 30000000)

	gasUsed := uint64(20000000)
	fm.Update(gasUsed)

	newBaseFee := fm.BaseFee()
	if newBaseFee <= 1000000000 {
		t.Errorf("base fee should increase when gas used > target, got %d", newBaseFee)
	}
}

func TestFeeMarket_BaseFeeAdjustmentDownward(t *testing.T) {
	fm := NewFeeMarket(1000000000, 15000000, 30000000)

	gasUsed := uint64(10000000)
	fm.Update(gasUsed)

	newBaseFee := fm.BaseFee()
	if newBaseFee >= 1000000000 {
		t.Errorf("base fee should decrease when gas used < target, got %d", newBaseFee)
	}
}

func TestFeeMarket_BaseFeeStaysSame(t *testing.T) {
	fm := NewFeeMarket(1000000000, 15000000, 30000000)

	gasUsed := uint64(15000000)
	fm.Update(gasUsed)

	newBaseFee := fm.BaseFee()
	if newBaseFee != 1000000000 {
		t.Errorf("base fee should stay same when gas used = target, got %d", newBaseFee)
	}
}

func TestFeeMarket_BaseFeeMinimum(t *testing.T) {
	fm := NewFeeMarket(1000000000, 15000000, 30000000)

	for i := 0; i < 100; i++ {
		fm.Update(0)
	}

	newBaseFee := fm.BaseFee()
	if newBaseFee < 1 {
		t.Errorf("base fee should not go below 1, got %d", newBaseFee)
	}
}

func TestFeeMarket_BaseFeeMaximum(t *testing.T) {
	fm := NewFeeMarket(1000000000, 15000000, 30000000)

	for i := 0; i < 100; i++ {
		fm.Update(30000000)
	}

	newBaseFee := fm.BaseFee()
	maxExpected := uint64(float64(1000000000) * math.Pow(1.125, 100))
	if newBaseFee > uint64(maxExpected) {
		t.Errorf("base fee grew beyond expected bounds: %d", newBaseFee)
	}
}

func TestFeeMarket_HundredBlocksVaryingGas(t *testing.T) {
	fm := NewFeeMarket(1000000000, 15000000, 30000000)

	baseFees := make([]uint64, 100)
	baseFees[0] = fm.BaseFee()

	for i := 1; i < 100; i++ {
		var gasUsed uint64
		switch i % 3 {
		case 0:
			gasUsed = 20000000
		case 1:
			gasUsed = 10000000
		case 2:
			gasUsed = 15000000
		}

		fm.Update(gasUsed)
		baseFees[i] = fm.BaseFee()
	}

	for i := 1; i < 100; i++ {
		if baseFees[i] == 0 {
			t.Errorf("base fee should not be zero at block %d", i)
		}
	}
}

func TestFeeMarket_CalculateTip(t *testing.T) {
	fm := NewFeeMarket(1000000000, 15000000, 30000000)

	tests := []struct {
		gasPrice  uint64
		expectedTip uint64
	}{
		{1000000000, 0},
		{1500000000, 500000000},
		{2000000000, 1000000000},
	}

	for _, tt := range tests {
		tip := fm.CalculateTip(tt.gasPrice)
		if tip != tt.expectedTip {
			t.Errorf("gasPrice %d: expected tip %d, got %d", tt.gasPrice, tt.expectedTip, tip)
		}
	}
}

func TestFeeMarket_AcceptableGasPrice(t *testing.T) {
	fm := NewFeeMarket(1000000000, 15000000, 30000000)

	tests := []struct {
		gasPrice uint64
		acceptable bool
	}{
		{1000000000, true},
		{500000000, false},
		{1500000000, true},
	}

	for _, tt := range tests {
		acceptable := fm.AcceptableGasPrice(tt.gasPrice)
		if acceptable != tt.acceptable {
			t.Errorf("gasPrice %d: expected acceptable=%v, got %v", tt.gasPrice, tt.acceptable, acceptable)
		}
	}
}

func TestFeeMarket_SetBaseFee(t *testing.T) {
	fm := NewFeeMarket(1000000000, 15000000, 30000000)

	fm.SetBaseFee(2000000000)
	if fm.BaseFee() != 2000000000 {
		t.Errorf("expected base fee 2000000000, got %d", fm.BaseFee())
	}
}

func TestFeeMarket_Reset(t *testing.T) {
	fm := NewFeeMarket(1000000000, 15000000, 30000000)

	fm.Update(20000000)
	fm.Reset()

	if fm.BaseFee() == 0 {
		t.Error("base fee should not be zero after reset")
	}
}

func TestFeeMarket_GasTarget(t *testing.T) {
	fm := NewFeeMarket(1000000000, 15000000, 30000000)

	if fm.GasTarget() != 15000000 {
		t.Errorf("expected gas target 15000000, got %d", fm.GasTarget())
	}
}

func TestFeeMarket_BlockGasLimit(t *testing.T) {
	fm := NewFeeMarket(1000000000, 15000000, 30000000)

	if fm.BlockGasLimit() != 30000000 {
		t.Errorf("expected block gas limit 30000000, got %d", fm.BlockGasLimit())
	}
}

func TestFeeMarket_DefaultFeeMarket(t *testing.T) {
	fm := DefaultFeeMarket()

	if fm.BaseFee() != 1000000000 {
		t.Errorf("expected default base fee 1000000000, got %d", fm.BaseFee())
	}
	if fm.GasTarget() != 15000000 {
		t.Errorf("expected default gas target 15000000, got %d", fm.GasTarget())
	}
	if fm.BlockGasLimit() != 30000000 {
		t.Errorf("expected default block gas limit 30000000, got %d", fm.BlockGasLimit())
	}
}
