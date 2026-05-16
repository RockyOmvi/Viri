package ledger

import (
	"math/big"
	"sync"
)

type FeeMarket struct {
	mu                sync.Mutex
	baseFee           uint64
	gasTarget         uint64
	gasUsed           uint64
	maxBaseFeeChange  uint64 // numerator / 8 = max change fraction, e.g. 1 = 1/8 = 12.5%
	blockGasLimit     uint64
}

func NewFeeMarket(initialBaseFee, gasTarget, blockGasLimit uint64) *FeeMarket {
	return &FeeMarket{
		baseFee:          initialBaseFee,
		gasTarget:        gasTarget,
		maxBaseFeeChange: 1, // 1/8 = 12.5%
		blockGasLimit:    blockGasLimit,
	}
}

func DefaultFeeMarket() *FeeMarket {
	return &FeeMarket{
		baseFee:          1_000_000_000,
		gasTarget:        15_000_000,
		maxBaseFeeChange: 1, // 1/8 = 12.5%
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

	// Use integer arithmetic via big.Int to avoid float64 precision loss on large baseFee values
	base := new(big.Int).SetUint64(fm.baseFee)
	target := new(big.Int).SetUint64(fm.gasTarget)

	var delta *big.Int
	if gasUsed > fm.gasTarget {
		excess := new(big.Int).SetUint64(gasUsed - fm.gasTarget)
		// delta = baseFee * min(excess, target) / target / 8
		excess = minBig(excess, target)
		delta = new(big.Int).Mul(base, excess)
		delta.Div(delta, target)
		delta.Div(delta, big.NewInt(8))
		fm.baseFee = new(big.Int).Add(base, delta).Uint64()
	} else {
		deficit := new(big.Int).SetUint64(fm.gasTarget - gasUsed)
		deficit = minBig(deficit, target)
		delta = new(big.Int).Mul(base, deficit)
		delta.Div(delta, target)
		delta.Div(delta, big.NewInt(8))
		newBase := new(big.Int).Sub(base, delta)
		if newBase.Sign() < 1 {
			fm.baseFee = 1
		} else {
			fm.baseFee = newBase.Uint64()
		}
	}
}

// minBig returns the smaller of a and b.
func minBig(a, b *big.Int) *big.Int {
	if a.Cmp(b) < 0 {
		return a
	}
	return b
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
