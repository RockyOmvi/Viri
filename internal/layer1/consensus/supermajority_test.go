package consensus

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func TestHasSuperMajoritySingle(t *testing.T) {
	key, _ := crypto.GenerateKey()
	staking := NewStakingModule(24*time.Hour, 0.01)
	staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)

	activeValidators := staking.GetActiveValidators()
	validators := make([]*Validator, 0, len(activeValidators))
	for _, sv := range activeValidators {
		validators = append(validators, &Validator{
			Address:   sv.Address,
			PublicKey: sv.PublicKey,
			Stake:     sv.Stake,
			IsActive:  true,
		})
	}

	vs := NewValidatorSet(validators, 1)

	validatorAddr := hex.EncodeToString(key.PubKey().Address())
	t.Logf("Validator addr hex: %s (len=%d)", validatorAddr, len(validatorAddr))
	t.Logf("Total stake: %d", vs.TotalStake())

	sigs := map[string]bool{
		validatorAddr: true,
	}

	result := vs.HasSuperMajority(sigs)
	t.Logf("HasSuperMajority: %v", result)

	if !result {
		t.Fatal("expected supermajority with single validator")
	}
}

func TestHasSuperMajoritySingleFromSelfVote(t *testing.T) {
	key, _ := crypto.GenerateKey()
	staking := NewStakingModule(24*time.Hour, 0.01)
	staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)

	activeValidators := staking.GetActiveValidators()
	validators := make([]*Validator, 0, len(activeValidators))
	for _, sv := range activeValidators {
		validators = append(validators, &Validator{
			Address:   sv.Address,
			PublicKey: sv.PublicKey,
			Stake:     sv.Stake,
			IsActive:  true,
		})
	}

	vs := NewValidatorSet(validators, 1)

	validatorAddr := hex.EncodeToString(vs.validators[0].Address)
	t.Logf("Validator addr from vs: %s", validatorAddr)

	votes := make(map[string]map[Phase]map[string]bool)
	heightKey := "0-0-PREPARE"
	votes[heightKey] = make(map[Phase]map[string]bool)
	votes[heightKey][PhasePrepare] = make(map[string]bool)
	votes[heightKey][PhasePrepare][validatorAddr] = true

	result := vs.HasSuperMajority(votes[heightKey][PhasePrepare])
	t.Logf("HasSuperMajority with key '%s': %v", heightKey, result)

	if !result {
		t.Fatal("expected supermajority")
	}
}
