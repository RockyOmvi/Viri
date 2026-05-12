package consensus

import (
	"fmt"
	"sync"
	"time"
)

type StakingAction uint8

const (
	StakeAction StakingAction = iota
	UnstakeAction
	SlashAction
)

type StakingEvent struct {
	Action    StakingAction
	Validator []byte
	Amount    uint64
	Timestamp time.Time
	TxHash    []byte
}

type StakingModule struct {
	mu              sync.RWMutex
	validators      map[string]*StakeRecord
	delegationPool  map[string][]*Delegation
	events          []*StakingEvent
	totalStaked     uint64
	unbondingPeriod time.Duration
	slashingFraction float64
}

type StakeRecord struct {
	Address      []byte
	PublicKey    []byte
	Stake        uint64
	SelfStake    uint64
	DelegatedStake uint64
	Rewards      uint64
	IsActive     bool
	UnbondingAt  time.Time
	Jailed       bool
	JailedUntil  time.Time
	SlashedAmount uint64
}

type Delegation struct {
	Delegator  []byte
	Validator  []byte
	Amount     uint64
	UnbondingAt time.Time
}

func NewStakingModule(unbondingPeriod time.Duration, slashingFraction float64) *StakingModule {
	return &StakingModule{
		validators:       make(map[string]*StakeRecord),
		delegationPool:   make(map[string][]*Delegation),
		events:           make([]*StakingEvent, 0),
		unbondingPeriod:  unbondingPeriod,
		slashingFraction: slashingFraction,
	}
}

func (sm *StakingModule) Stake(validator []byte, publicKey []byte, amount uint64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if amount == 0 {
		return fmt.Errorf("stake amount must be positive")
	}

	key := string(validator)
	record, exists := sm.validators[key]
	if exists {
		if !record.IsActive {
			return fmt.Errorf("validator is deactivated")
		}
		if record.Jailed {
			return fmt.Errorf("validator is jailed")
		}
		record.Stake += amount
		record.SelfStake += amount
	} else {
		record = &StakeRecord{
			Address:   append([]byte(nil), validator...),
			PublicKey: append([]byte(nil), publicKey...),
			Stake:     amount,
			SelfStake: amount,
			IsActive:  true,
		}
		sm.validators[key] = record
	}

	sm.totalStaked += amount

	sm.events = append(sm.events, &StakingEvent{
		Action:    StakeAction,
		Validator: validator,
		Amount:    amount,
		Timestamp: time.Now(),
	})

	return nil
}

func (sm *StakingModule) Unstake(validator []byte, amount uint64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := string(validator)
	record, exists := sm.validators[key]
	if !exists {
		return fmt.Errorf("validator not found")
	}

	if amount > record.Stake {
		return fmt.Errorf("insufficient stake")
	}
	if amount > record.SelfStake {
		return fmt.Errorf("insufficient self stake")
	}

	record.Stake -= amount
	record.SelfStake -= amount

	if record.Stake == 0 {
		record.IsActive = false
		record.UnbondingAt = time.Now().Add(sm.unbondingPeriod)
	}

	sm.totalStaked -= amount

	sm.events = append(sm.events, &StakingEvent{
		Action:    UnstakeAction,
		Validator: validator,
		Amount:    amount,
		Timestamp: time.Now(),
	})

	return nil
}

func (sm *StakingModule) Slash(validator []byte, fraction float64) (uint64, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := string(validator)
	record, exists := sm.validators[key]
	if !exists {
		return 0, fmt.Errorf("validator not found")
	}

	// Use integer arithmetic to avoid float64 precision loss for large stakes.
	// fraction is expected in [0,1] range, e.g. 0.1 = 10%.
	num := uint64(fraction * 1e9)
	denom := uint64(1e9)
	slashAmount := record.Stake * num / denom
	if slashAmount > record.Stake {
		slashAmount = record.Stake
	}

	record.Stake -= slashAmount
	record.SlashedAmount += slashAmount
	record.Jailed = true
	record.JailedUntil = time.Now().Add(sm.unbondingPeriod)

	sm.totalStaked -= slashAmount

	sm.events = append(sm.events, &StakingEvent{
		Action:    SlashAction,
		Validator: validator,
		Amount:    slashAmount,
		Timestamp: time.Now(),
	})

	return slashAmount, nil
}

func (sm *StakingModule) Jail(validator []byte, duration time.Duration) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := string(validator)
	record, exists := sm.validators[key]
	if !exists {
		return fmt.Errorf("validator not found")
	}

	record.Jailed = true
	record.JailedUntil = time.Now().Add(duration)

	return nil
}

func (sm *StakingModule) Unjail(validator []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	key := string(validator)
	record, exists := sm.validators[key]
	if !exists {
		return fmt.Errorf("validator not found")
	}

	if time.Now().Before(record.JailedUntil) {
		return fmt.Errorf("jail period not expired")
	}

	record.Jailed = false

	return nil
}

func (sm *StakingModule) GetValidator(validator []byte) (*StakeRecord, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	key := string(validator)
	record, exists := sm.validators[key]
	if !exists {
		return nil, false
	}

	return record.Clone(), true
}

func (sm *StakingModule) GetActiveValidators() []*StakeRecord {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var active []*StakeRecord
	for _, record := range sm.validators {
		if record.IsActive && !record.Jailed && record.Stake > 0 {
			active = append(active, record.Clone())
		}
	}

	return active
}

func (sm *StakingModule) GetInactiveValidators() []*StakeRecord {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var inactive []*StakeRecord
	for _, record := range sm.validators {
		if !record.IsActive || record.Jailed {
			inactive = append(inactive, record.Clone())
		}
	}

	return inactive
}

func (sm *StakingModule) TotalStaked() uint64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.totalStaked
}

func (sm *StakingModule) GetEvents(from time.Time) []*StakingEvent {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var events []*StakingEvent
	for _, e := range sm.events {
		if !e.Timestamp.Before(from) {
			events = append(events, e)
		}
	}

	return events
}

func (sm *StakeRecord) Clone() *StakeRecord {
	c := *sm
	c.Address = append([]byte(nil), sm.Address...)
	c.PublicKey = append([]byte(nil), sm.PublicKey...)
	return &c
}
