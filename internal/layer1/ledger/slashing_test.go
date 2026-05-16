package ledger

import (
	"math/big"
	"testing"
)

func TestSlashingDoubleSign(t *testing.T) {
	sp := NewSlashingProcessor(nil)
	valState := &ValidatorState{TotalStake: big.NewInt(1000000)}

	record := &SlashingRecord{
		Reason:      SlashingDoubleSign,
		Validator:   []byte("val1"),
		BlockHeight: 100,
	}

	slashed, err := sp.ProcessSlashing(record, valState, 200)
	if err != nil {
		t.Fatalf("process slashing: %v", err)
	}

	expectedSlash := big.NewInt(500000)
	if slashed.Cmp(expectedSlash) != 0 {
		t.Errorf("expected slashed %d, got %d", expectedSlash, slashed)
	}

	expectedRemaining := big.NewInt(500000)
	if valState.TotalStake.Cmp(expectedRemaining) != 0 {
		t.Errorf("expected remaining %d, got %d", expectedRemaining, valState.TotalStake)
	}

	if !valState.IsJailed {
		t.Errorf("validator should be jailed")
	}
}

func TestSlashingDowntime(t *testing.T) {
	sp := NewSlashingProcessor(nil)
	valState := &ValidatorState{TotalStake: big.NewInt(1000000)}

	record := &SlashingRecord{
		Reason:      SlashingDowntime,
		Validator:   []byte("val2"),
		BlockHeight: 100,
	}

	slashed, err := sp.ProcessSlashing(record, valState, 200)
	if err != nil {
		t.Fatalf("process slashing: %v", err)
	}

	expectedSlash := big.NewInt(10000)
	if slashed.Cmp(expectedSlash) != 0 {
		t.Errorf("expected slashed %d, got %d", expectedSlash, slashed)
	}
}

func TestSlashingAlreadyJailed(t *testing.T) {
	sp := NewSlashingProcessor(nil)
	valState := &ValidatorState{
		TotalStake:  big.NewInt(1000000),
		IsJailed:    true,
		JailedUntil: 500,
	}

	record := &SlashingRecord{
		Reason:      SlashingDoubleSign,
		Validator:   []byte("val1"),
		BlockHeight: 100,
	}

	_, err := sp.ProcessSlashing(record, valState, 200)
	if err != ErrValidatorJailed {
		t.Errorf("expected ErrValidatorJailed, got %v", err)
	}
}

func TestSlashingExpiredEvidence(t *testing.T) {
	sp := NewSlashingProcessor(nil)
	valState := &ValidatorState{TotalStake: big.NewInt(1000000)}

	record := &SlashingRecord{
		Reason:      SlashingDoubleSign,
		Validator:   []byte("val1"),
		BlockHeight: 50,
	}

	_, err := sp.ProcessSlashing(record, valState, 200000)
	if err == nil {
		t.Errorf("expected error for expired evidence")
	}
}

func TestUnjail(t *testing.T) {
	sp := NewSlashingProcessor(nil)
	valState := &ValidatorState{
		TotalStake:  big.NewInt(500000),
		IsJailed:    true,
		JailedUntil: 1000,
	}

	err := sp.Unjail(valState, 500)
	if err == nil {
		t.Errorf("should not allow unjail before period ends")
	}

	err = sp.Unjail(valState, 1500)
	if err != nil {
		t.Fatalf("unjail: %v", err)
	}

	if valState.IsJailed {
		t.Errorf("validator should no longer be jailed")
	}
}

func TestSlashingFullSlash(t *testing.T) {
	sp := NewSlashingProcessor(&SlashingConfig{
		DoubleSignSlashRate:  10000,
		DoubleSignJailPeriod: 10000,
		DowntimeSlashRate:    100,
		DowntimeJailPeriod:   1000,
		MaxEvidenceAge:       100000,
	})

	valState := &ValidatorState{TotalStake: big.NewInt(500000)}

	record := &SlashingRecord{
		Reason:      SlashingDoubleSign,
		Validator:   []byte("val1"),
		BlockHeight: 100,
	}

	slashed, err := sp.ProcessSlashing(record, valState, 200)
	if err != nil {
		t.Fatalf("process slashing: %v", err)
	}

	if slashed.Cmp(big.NewInt(500000)) != 0 {
		t.Errorf("expected full slash 500000, got %d", slashed)
	}

	if valState.TotalStake.Sign() != 0 {
		t.Errorf("expected zero stake, got %d", valState.TotalStake)
	}
}
