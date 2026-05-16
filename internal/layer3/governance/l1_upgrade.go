package governance

import (
	"fmt"
	"math/big"
	"time"
)

type UpgradeTarget uint8

const (
	UpgradeConsensusConfig   UpgradeTarget = 1
	UpgradeEconomicsConfig   UpgradeTarget = 2
	UpgradeSlashingConfig    UpgradeTarget = 3
	UpgradeProtocolVersion   UpgradeTarget = 4
	UpgradeMaxBlockSize      UpgradeTarget = 5
	UpgradeMaxGasPerBlock    UpgradeTarget = 6
	UpgradeMinValidatorStake UpgradeTarget = 7
)

type ParameterChange struct {
	Target UpgradeTarget
	Key    string
	Value  string
}

type ConsensusParams struct {
	BlockTime         time.Duration
	ViewTimeout       time.Duration
	MaxViewTimeout    time.Duration
	TimeoutIncrease   time.Duration
	EpochLength       uint64
	SlashingFraction  float64
	DowntimeThreshold uint64
	MessageRateLimit  int
}

type EconomicsParams struct {
	BlockReward          string
	HalvingInterval      uint64
	GasTarget            uint64
	GasLimitBound        uint64
	ValidatorShare       uint64
	DeveloperShare       uint64
	BurnShare            uint64
	BaseFeeTarget        uint64
	BaseFeeMaxChangeRate uint64
}

type SlashingParams struct {
	DoubleSignSlashRate  uint64
	DoubleSignJailPeriod uint64
	DowntimeSlashRate    uint64
	DowntimeJailPeriod   uint64
	MaxEvidenceAge       uint64
}

type L1UpgradeExecutor struct {
	consensusCfg interface {
		Apply(params ConsensusParams)
	}
	economicsCfg interface {
		Apply(params EconomicsParams) error
	}
	slashingCfg interface {
		Apply(params SlashingParams)
	}
	protocolVersion *uint64
	onUpgrade       func(UpgradeTarget, interface{})
}

func NewL1UpgradeExecutor(
	consensusApplier interface{ Apply(ConsensusParams) },
	economicsApplier interface{ Apply(EconomicsParams) error },
	slashingApplier interface{ Apply(SlashingParams) },
	protocolVer *uint64,
	onUpgrade func(UpgradeTarget, interface{}),
) *L1UpgradeExecutor {
	return &L1UpgradeExecutor{
		consensusCfg:    consensusApplier,
		economicsCfg:    economicsApplier,
		slashingCfg:     slashingApplier,
		protocolVersion: protocolVer,
		onUpgrade:       onUpgrade,
	}
}

func (e *L1UpgradeExecutor) Execute(proposal *Proposal, changes []ParameterChange) error {
	if proposal.Status != ProposalStatusPassed {
		return fmt.Errorf("proposal %d has not passed (status: %v)", proposal.ID, proposal.Status)
	}

	var consensusChanges ConsensusParams
	var economicsChanges EconomicsParams
	var slashingChanges SlashingParams
	var upgradeProtocol bool
	var newProtocolVersion uint64

	for _, change := range changes {
		switch change.Target {
		case UpgradeConsensusConfig:
			if err := applyConsensusChange(&consensusChanges, change.Key, change.Value); err != nil {
				return fmt.Errorf("change %s=%s: %w", change.Key, change.Value, err)
			}
		case UpgradeEconomicsConfig:
			if err := applyEconomicsChange(&economicsChanges, change.Key, change.Value); err != nil {
				return fmt.Errorf("change %s=%s: %w", change.Key, change.Value, err)
			}
		case UpgradeSlashingConfig:
			if err := applySlashingChange(&slashingChanges, change.Key, change.Value); err != nil {
				return fmt.Errorf("change %s=%s: %w", change.Key, change.Value, err)
			}
		case UpgradeProtocolVersion:
			upgradeProtocol = true
			ver, ok := new(big.Int).SetString(change.Value, 10)
			if !ok || ver == nil {
				return fmt.Errorf("invalid protocol version: %s", change.Value)
			}
			newProtocolVersion = ver.Uint64()
		default:
			return fmt.Errorf("unknown upgrade target: %d", change.Target)
		}
	}

	if e.consensusCfg != nil {
		e.consensusCfg.Apply(consensusChanges)
	}
	if e.economicsCfg != nil {
		if err := e.economicsCfg.Apply(economicsChanges); err != nil {
			return err
		}
	}
	if e.slashingCfg != nil {
		e.slashingCfg.Apply(slashingChanges)
	}
	if upgradeProtocol && e.protocolVersion != nil {
		*e.protocolVersion = newProtocolVersion
	}

	proposal.Status = ProposalStatusExecuted

	if e.onUpgrade != nil {
		for _, change := range changes {
			e.onUpgrade(change.Target, change.Value)
		}
	}

	return nil
}

func applyConsensusChange(c *ConsensusParams, key, value string) error {
	v := new(big.Int)
	_, ok := v.SetString(value, 10)
	if !ok {
		return fmt.Errorf("invalid numeric value: %s", value)
	}
	i := v.Int64()
	d := time.Duration(i)

	switch key {
	case "BlockTime":
		c.BlockTime = d
	case "ViewTimeout":
		c.ViewTimeout = d
	case "MaxViewTimeout":
		c.MaxViewTimeout = d
	case "TimeoutIncrease":
		c.TimeoutIncrease = d
	case "EpochLength":
		c.EpochLength = v.Uint64()
	case "SlashingFraction":
		c.SlashingFraction = float64(i) / 10000
	case "DowntimeThreshold":
		c.DowntimeThreshold = v.Uint64()
	case "MessageRateLimit":
		c.MessageRateLimit = int(i)
	default:
		return fmt.Errorf("unknown consensus param: %s", key)
	}
	return nil
}

func applyEconomicsChange(c *EconomicsParams, key, value string) error {
	v := new(big.Int)
	_, ok := v.SetString(value, 10)
	if !ok {
		return fmt.Errorf("invalid numeric value: %s", value)
	}

	switch key {
	case "BlockReward":
		c.BlockReward = value
	case "HalvingInterval":
		c.HalvingInterval = v.Uint64()
	case "GasTarget":
		c.GasTarget = v.Uint64()
	case "GasLimitBound":
		c.GasLimitBound = v.Uint64()
	case "ValidatorShare":
		c.ValidatorShare = v.Uint64()
	case "DeveloperShare":
		c.DeveloperShare = v.Uint64()
	case "BurnShare":
		c.BurnShare = v.Uint64()
	case "BaseFeeTarget":
		c.BaseFeeTarget = v.Uint64()
	case "BaseFeeMaxChangeRate":
		c.BaseFeeMaxChangeRate = v.Uint64()
	default:
		return fmt.Errorf("unknown economics param: %s", key)
	}
	return nil
}

func applySlashingChange(c *SlashingParams, key, value string) error {
	v := new(big.Int)
	_, ok := v.SetString(value, 10)
	if !ok {
		return fmt.Errorf("invalid numeric value: %s", value)
	}

	switch key {
	case "DoubleSignSlashRate":
		c.DoubleSignSlashRate = v.Uint64()
	case "DoubleSignJailPeriod":
		c.DoubleSignJailPeriod = v.Uint64()
	case "DowntimeSlashRate":
		c.DowntimeSlashRate = v.Uint64()
	case "DowntimeJailPeriod":
		c.DowntimeJailPeriod = v.Uint64()
	case "MaxEvidenceAge":
		c.MaxEvidenceAge = v.Uint64()
	default:
		return fmt.Errorf("unknown slashing param: %s", key)
	}
	return nil
}
