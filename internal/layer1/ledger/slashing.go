package ledger

import (
	"fmt"
	"math/big"
)

type SlashingConfig struct {
	DoubleSignSlashRate  uint64
	DoubleSignJailPeriod uint64
	DowntimeSlashRate    uint64
	DowntimeJailPeriod   uint64
	MaxEvidenceAge       uint64
}

func DefaultSlashingConfig() *SlashingConfig {
	return &SlashingConfig{
		DoubleSignSlashRate:  5000,
		DoubleSignJailPeriod: 10000,
		DowntimeSlashRate:    100,
		DowntimeJailPeriod:   1000,
		MaxEvidenceAge:       100000,
	}
}

type ValidatorState struct {
	TotalStake   *big.Int
	IsJailed     bool
	JailedUntil  uint64
	SlashCount   uint64
}

type SlashingProcessor struct {
	config *SlashingConfig
}

func NewSlashingProcessor(config *SlashingConfig) *SlashingProcessor {
	if config == nil {
		config = DefaultSlashingConfig()
	}
	return &SlashingProcessor{config: config}
}

func (sp *SlashingProcessor) ProcessSlashing(
	record *SlashingRecord,
	valState *ValidatorState,
	currentHeight uint64,
) (*big.Int, error) {
	if valState.IsJailed && valState.JailedUntil > currentHeight {
		return nil, ErrValidatorJailed
	}

	if record.BlockHeight+sp.config.MaxEvidenceAge < currentHeight {
		return nil, fmt.Errorf("evidence too old")
	}

	var slashRate uint64
	var jailPeriod uint64

	switch record.Reason {
	case SlashingDoubleSign:
		slashRate = sp.config.DoubleSignSlashRate
		jailPeriod = sp.config.DoubleSignJailPeriod
	case SlashingDowntime:
		slashRate = sp.config.DowntimeSlashRate
		jailPeriod = sp.config.DowntimeJailPeriod
	case SlashingInvalidBlock:
		slashRate = record.SlashRate
		jailPeriod = record.JailPeriod
	case SlashingMalicious:
		slashRate = record.SlashRate
		jailPeriod = record.JailPeriod
	default:
		return nil, fmt.Errorf("unknown slashing reason: %d", record.Reason)
	}

	if slashRate > 10000 {
		slashRate = 10000
	}

	slashAmount := new(big.Int).Mul(valState.TotalStake, big.NewInt(int64(slashRate)))
	slashAmount.Div(slashAmount, big.NewInt(10000))

	if slashAmount.Cmp(valState.TotalStake) >= 0 {
		valState.TotalStake = big.NewInt(0)
	} else {
		valState.TotalStake = new(big.Int).Sub(valState.TotalStake, slashAmount)
	}

	valState.IsJailed = true
	valState.JailedUntil = currentHeight + jailPeriod
	valState.SlashCount++

	return slashAmount, nil
}

func (sp *SlashingProcessor) Unjail(valState *ValidatorState, currentHeight uint64) error {
	if !valState.IsJailed {
		return fmt.Errorf("validator is not jailed")
	}
	if valState.JailedUntil > currentHeight {
		return fmt.Errorf("validator still jailed until block %d", valState.JailedUntil)
	}
	valState.IsJailed = false
	valState.JailedUntil = 0
	return nil
}
