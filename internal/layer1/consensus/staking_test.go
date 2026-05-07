package consensus

import (
	"testing"
	"time"
)

func TestStakingModuleStake(t *testing.T) {
	sm := NewStakingModule(21*24*time.Hour, 0.01)

	validator := []byte("validator1")
	pubKey := []byte("pubkey1")

	err := sm.Stake(validator, pubKey, 1000)
	if err != nil {
		t.Fatalf("Stake failed: %v", err)
	}

	record, exists := sm.GetValidator(validator)
	if !exists {
		t.Fatal("Validator not found after staking")
	}

	if record.Stake != 1000 {
		t.Errorf("Expected stake 1000, got %d", record.Stake)
	}

	if record.SelfStake != 1000 {
		t.Errorf("Expected self stake 1000, got %d", record.SelfStake)
	}

	if !record.IsActive {
		t.Error("Validator should be active")
	}
}

func TestStakingModuleUnstake(t *testing.T) {
	sm := NewStakingModule(21*24*time.Hour, 0.01)

	validator := []byte("validator1")
	pubKey := []byte("pubkey1")

	sm.Stake(validator, pubKey, 1000)

	err := sm.Unstake(validator, 500)
	if err != nil {
		t.Fatalf("Unstake failed: %v", err)
	}

	record, _ := sm.GetValidator(validator)
	if record.Stake != 500 {
		t.Errorf("Expected stake 500, got %d", record.Stake)
	}
}

func TestStakingModuleFullUnstake(t *testing.T) {
	sm := NewStakingModule(21*24*time.Hour, 0.01)

	validator := []byte("validator1")
	pubKey := []byte("pubkey1")

	sm.Stake(validator, pubKey, 1000)

	err := sm.Unstake(validator, 1000)
	if err != nil {
		t.Fatalf("Unstake failed: %v", err)
	}

	record, _ := sm.GetValidator(validator)
	if record.IsActive {
		t.Error("Validator should be inactive after full unstake")
	}
}

func TestStakingModuleSlash(t *testing.T) {
	sm := NewStakingModule(21*24*time.Hour, 0.01)

	validator := []byte("validator1")
	pubKey := []byte("pubkey1")

	sm.Stake(validator, pubKey, 1000)

	slashAmount, err := sm.Slash(validator, 0.1)
	if err != nil {
		t.Fatalf("Slash failed: %v", err)
	}

	if slashAmount != 100 {
		t.Errorf("Expected slash amount 100, got %d", slashAmount)
	}

	record, _ := sm.GetValidator(validator)
	if !record.Jailed {
		t.Error("Validator should be jailed after slashing")
	}

	if record.Stake != 900 {
		t.Errorf("Expected stake 900, got %d", record.Stake)
	}
}

func TestStakingModuleJail(t *testing.T) {
	sm := NewStakingModule(21*24*time.Hour, 0.01)

	validator := []byte("validator1")
	pubKey := []byte("pubkey1")

	sm.Stake(validator, pubKey, 1000)

	err := sm.Jail(validator, 1*time.Hour)
	if err != nil {
		t.Fatalf("Jail failed: %v", err)
	}

	record, _ := sm.GetValidator(validator)
	if !record.Jailed {
		t.Error("Validator should be jailed")
	}

	active := sm.GetActiveValidators()
	if len(active) != 0 {
		t.Error("Jailed validator should not be in active list")
	}
}

func TestStakingModuleTotalStaked(t *testing.T) {
	sm := NewStakingModule(21*24*time.Hour, 0.01)

	sm.Stake([]byte("v1"), []byte("pk1"), 1000)
	sm.Stake([]byte("v2"), []byte("pk2"), 2000)

	total := sm.TotalStaked()
	if total != 3000 {
		t.Errorf("Expected total staked 3000, got %d", total)
	}

	sm.Unstake([]byte("v1"), 500)
	total = sm.TotalStaked()
	if total != 2500 {
		t.Errorf("Expected total staked 2500, got %d", total)
	}
}

func TestStakingModuleEvents(t *testing.T) {
	sm := NewStakingModule(21*24*time.Hour, 0.01)

	before := time.Now()
	sm.Stake([]byte("v1"), []byte("pk1"), 1000)

	events := sm.GetEvents(before)
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	if events[0].Action != StakeAction {
		t.Errorf("Expected stake action, got %v", events[0].Action)
	}
}
