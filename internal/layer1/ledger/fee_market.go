package ledger

import (
	"math"
	"sync"
)

type FeeMarket struct {
	mu                sync.Mutex
	baseFee           uint64
	gasTarget         uint64
	gasUsed           uint64
	maxBaseFeeChange  float64
	blockGasLimit     uint64
}

func NewFeeMarket(initialBaseFee, gasTarget, blockGasLimit uint64) *FeeMarket {
	return &FeeMarket{
		baseFee:          initialBaseFee,
		gasTarget:        gasTarget,
		maxBaseFeeChange: 0.125,
		blockGasLimit:    blockGasLimit,
	}
}

func DefaultFeeMarket() *FeeMarket {
	return &FeeMarket{
		baseFee:          1_000_000_000,
		gasTarget:        15_000_000,
		maxBaseFeeChange: 0.125,
		blockGasLimit:    30_000_000,
	}
}

func (fm *FeeMarket) Update(gasUsed uint64) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.gasUsed = gasUsed

	if gasUsed == fm.gasTarget {
		return
	}

	if gasUsed > fm.gasTarget {
		excess := gasUsed - fm.gasTarget
		ratio := float64(excess) / float64(fm.gasTarget)
		increase := float64(fm.baseFee) * math.Min(ratio, fm.maxBaseFeeChange)
		fm.baseFee = fm.baseFee + uint64(increase)
	} else {
		deficit := fm.gasTarget - gasUsed
		ratio := float64(deficit) / float64(fm.gasTarget)
		decrease := float64(fm.baseFee) * math.Min(ratio, fm.maxBaseFeeChange)
		newBaseFee := fm.baseFee - uint64(decrease)
		if newBaseFee < 1 {
			newBaseFee = 1
		}
		fm.baseFee = newBaseFee
	}
}

func (fm *FeeMarket) BaseFee() uint64 {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	return fm.baseFee
}

func (fm *FeeMarket) SetBaseFee(fee uint64) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.baseFee = fee
}

func (fm *FeeMarket) AcceptableGasPrice(gasPrice uint64) bool {
	return gasPrice >= fm.BaseFee()
}

func (fm *FeeMarket) CalculateTip(gasPrice uint64) uint64 {
	baseFee := fm.BaseFee()
	if gasPrice <= baseFee {
		return 0
	}
	return gasPrice - baseFee
}

func (fm *FeeMarket) GasTarget() uint64 {
	return fm.gasTarget
}

func (fm *FeeMarket) BlockGasLimit() uint64 {
	return fm.blockGasLimit
}

func (fm *FeeMarket) Reset() {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.gasUsed = 0
}
