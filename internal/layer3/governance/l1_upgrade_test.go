package governance

import (
	"testing"
	"time"
)

type mockConsensusApplier struct {
	applied ConsensusParams
}

func (m *mockConsensusApplier) Apply(p ConsensusParams) {
	m.applied = p
}

type mockEconomicsApplier struct {
	applied EconomicsParams
	err     error
}

func (m *mockEconomicsApplier) Apply(p EconomicsParams) error {
	m.applied = p
	return m.err
}

type mockSlashingApplier struct {
	applied SlashingParams
}

func (m *mockSlashingApplier) Apply(p SlashingParams) {
	m.applied = p
}

func TestL1UpgradeExecutor_ConsensusParams(t *testing.T) {
	consensusMock := &mockConsensusApplier{}
	exec := NewL1UpgradeExecutor(consensusMock, nil, nil, nil, nil)

	proposal := &Proposal{
		ID:     1,
		Status: ProposalStatusPassed,
	}

	changes := []ParameterChange{
		{Target: UpgradeConsensusConfig, Key: "BlockTime", Value: "2000000000"},
		{Target: UpgradeConsensusConfig, Key: "EpochLength", Value: "5000"},
		{Target: UpgradeConsensusConfig, Key: "SlashingFraction", Value: "200"},
	}

	err := exec.Execute(proposal, changes)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if consensusMock.applied.BlockTime != 2*time.Second {
		t.Errorf("expected BlockTime 2s, got %v", consensusMock.applied.BlockTime)
	}
	if consensusMock.applied.EpochLength != 5000 {
		t.Errorf("expected EpochLength 5000, got %d", consensusMock.applied.EpochLength)
	}
	if consensusMock.applied.SlashingFraction != 0.02 {
		t.Errorf("expected SlashingFraction 0.02, got %f", consensusMock.applied.SlashingFraction)
	}

	if proposal.Status != ProposalStatusExecuted {
		t.Errorf("expected proposal executed, got %v", proposal.Status)
	}
}

func TestL1UpgradeExecutor_RejectsNonPassed(t *testing.T) {
	exec := NewL1UpgradeExecutor(nil, nil, nil, nil, nil)

	proposal := &Proposal{
		ID:     2,
		Status: ProposalStatusActive,
	}

	err := exec.Execute(proposal, []ParameterChange{
		{Target: UpgradeProtocolVersion, Key: "version", Value: "2"},
	})
	if err == nil {
		t.Fatal("expected error for non-passed proposal")
	}
}

func TestL1UpgradeExecutor_ProtocolVersion(t *testing.T) {
	ver := uint64(1)
	exec := NewL1UpgradeExecutor(nil, nil, nil, &ver, nil)

	proposal := &Proposal{
		ID:     3,
		Status: ProposalStatusPassed,
	}

	err := exec.Execute(proposal, []ParameterChange{
		{Target: UpgradeProtocolVersion, Key: "version", Value: "2"},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if ver != 2 {
		t.Errorf("expected protocol version 2, got %d", ver)
	}
}

func TestL1UpgradeExecutor_EconomicsParams(t *testing.T) {
	econMock := &mockEconomicsApplier{}
	exec := NewL1UpgradeExecutor(nil, econMock, nil, nil, nil)

	proposal := &Proposal{
		ID:     4,
		Status: ProposalStatusPassed,
	}

	changes := []ParameterChange{
		{Target: UpgradeEconomicsConfig, Key: "GasTarget", Value: "20000000"},
		{Target: UpgradeEconomicsConfig, Key: "ValidatorShare", Value: "85"},
		{Target: UpgradeEconomicsConfig, Key: "BurnShare", Value: "5"},
	}

	err := exec.Execute(proposal, changes)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if econMock.applied.GasTarget != 20000000 {
		t.Errorf("expected GasTarget 20000000, got %d", econMock.applied.GasTarget)
	}
	if econMock.applied.ValidatorShare != 85 {
		t.Errorf("expected ValidatorShare 85, got %d", econMock.applied.ValidatorShare)
	}
	if econMock.applied.BurnShare != 5 {
		t.Errorf("expected BurnShare 5, got %d", econMock.applied.BurnShare)
	}
}

func TestL1UpgradeExecutor_SlashingParams(t *testing.T) {
	slashMock := &mockSlashingApplier{}
	exec := NewL1UpgradeExecutor(nil, nil, slashMock, nil, nil)

	proposal := &Proposal{
		ID:     5,
		Status: ProposalStatusPassed,
	}

	changes := []ParameterChange{
		{Target: UpgradeSlashingConfig, Key: "DoubleSignSlashRate", Value: "10000"},
		{Target: UpgradeSlashingConfig, Key: "DoubleSignJailPeriod", Value: "50000"},
	}

	err := exec.Execute(proposal, changes)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if slashMock.applied.DoubleSignSlashRate != 10000 {
		t.Errorf("expected DoubleSignSlashRate 10000, got %d", slashMock.applied.DoubleSignSlashRate)
	}
	if slashMock.applied.DoubleSignJailPeriod != 50000 {
		t.Errorf("expected DoubleSignJailPeriod 50000, got %d", slashMock.applied.DoubleSignJailPeriod)
	}
}

func TestL1UpgradeExecutor_InvalidParam(t *testing.T) {
	exec := NewL1UpgradeExecutor(nil, nil, nil, nil, nil)

	proposal := &Proposal{
		ID:     6,
		Status: ProposalStatusPassed,
	}

	err := exec.Execute(proposal, []ParameterChange{
		{Target: UpgradeConsensusConfig, Key: "NonExistentParam", Value: "100"},
	})
	if err == nil {
		t.Fatal("expected error for unknown param")
	}
}

func TestL1UpgradeExecutor_InvalidValue(t *testing.T) {
	exec := NewL1UpgradeExecutor(nil, nil, nil, nil, nil)

	proposal := &Proposal{
		ID:     7,
		Status: ProposalStatusPassed,
	}

	err := exec.Execute(proposal, []ParameterChange{
		{Target: UpgradeProtocolVersion, Key: "version", Value: "not-a-number"},
	})
	if err == nil {
		t.Fatal("expected error for invalid value")
	}
}

func TestL1UpgradeExecutor_OnUpgradeCallback(t *testing.T) {
	var called bool
	var calledTarget UpgradeTarget
	exec := NewL1UpgradeExecutor(nil, nil, nil, nil, func(target UpgradeTarget, val interface{}) {
		called = true
		calledTarget = target
	})

	proposal := &Proposal{
		ID:     8,
		Status: ProposalStatusPassed,
	}

	exec.Execute(proposal, []ParameterChange{
		{Target: UpgradeProtocolVersion, Key: "version", Value: "3"},
	})

	if !called {
		t.Fatal("onUpgrade callback not called")
	}
	if calledTarget != UpgradeProtocolVersion {
		t.Errorf("expected UpgradeProtocolVersion, got %d", calledTarget)
	}
}

func TestL1UpgradeExecutor_MultipleTargets(t *testing.T) {
	consensusMock := &mockConsensusApplier{}
	econMock := &mockEconomicsApplier{}
	slashMock := &mockSlashingApplier{}
	ver := uint64(1)

	exec := NewL1UpgradeExecutor(consensusMock, econMock, slashMock, &ver, nil)

	proposal := &Proposal{
		ID:     9,
		Status: ProposalStatusPassed,
	}

	changes := []ParameterChange{
		{Target: UpgradeConsensusConfig, Key: "BlockTime", Value: "1000000000"},
		{Target: UpgradeEconomicsConfig, Key: "HalvingInterval", Value: "1000000"},
		{Target: UpgradeSlashingConfig, Key: "DowntimeJailPeriod", Value: "500"},
		{Target: UpgradeProtocolVersion, Key: "version", Value: "4"},
	}

	err := exec.Execute(proposal, changes)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if consensusMock.applied.BlockTime != time.Second {
		t.Errorf("expected BlockTime 1s, got %v", consensusMock.applied.BlockTime)
	}
	if econMock.applied.HalvingInterval != 1000000 {
		t.Errorf("expected HalvingInterval 1000000, got %d", econMock.applied.HalvingInterval)
	}
	if slashMock.applied.DowntimeJailPeriod != 500 {
		t.Errorf("expected DowntimeJailPeriod 500, got %d", slashMock.applied.DowntimeJailPeriod)
	}
	if ver != 4 {
		t.Errorf("expected protocol version 4, got %d", ver)
	}
}

func TestL1UpgradeExecutor_UnknownTarget(t *testing.T) {
	exec := NewL1UpgradeExecutor(nil, nil, nil, nil, nil)

	proposal := &Proposal{
		ID:     10,
		Status: ProposalStatusPassed,
	}

	err := exec.Execute(proposal, []ParameterChange{
		{Target: UpgradeTarget(99), Key: "x", Value: "1"},
	})
	if err == nil {
		t.Fatal("expected error for unknown target")
	}
}
