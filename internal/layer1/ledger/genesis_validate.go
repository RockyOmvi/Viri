package ledger

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
)

func ValidateGenesis(config *GenesisConfig) error {
	if config.ChainID == 0 {
		return fmt.Errorf("chain_id must be greater than 0")
	}

	if config.InitialSupply == 0 {
		return fmt.Errorf("initial_supply must be greater than 0")
	}

	maxSupply := new(big.Int).Exp(big.NewInt(10), big.NewInt(26), nil)
	if new(big.Int).SetUint64(config.InitialSupply).Cmp(maxSupply) > 0 {
		return fmt.Errorf("initial_supply exceeds maximum supply")
	}

	if config.BlockTime <= 0 {
		return fmt.Errorf("block_time must be positive")
	}

	if config.BlockTime < 100*time.Millisecond {
		return fmt.Errorf("block_time too low: minimum 100ms")
	}

	if config.MaxBlockSize == 0 {
		return fmt.Errorf("max_block_size must be greater than 0")
	}

	if config.MaxBlockSize > 100*1024*1024 {
		return fmt.Errorf("max_block_size too large: maximum 100MB")
	}

	if config.MaxGasPerBlock == 0 {
		return fmt.Errorf("max_gas_per_block must be greater than 0")
	}

	if len(config.InitialValidators) > 0 {
		validatorAddresses := make(map[string]bool)
		totalStake := uint64(0)

		for i, validator := range config.InitialValidators {
			if len(validator.Address) == 0 {
				return fmt.Errorf("validator %d has empty address", i)
			}

			addrStr := hex.EncodeToString(validator.Address)
			if validatorAddresses[addrStr] {
				return fmt.Errorf("duplicate validator address: %s", addrStr)
			}
			validatorAddresses[addrStr] = true

			if validator.Stake == 0 {
				return fmt.Errorf("validator %d has zero stake", i)
			}

			totalStake += validator.Stake
		}

		if totalStake == 0 {
			return fmt.Errorf("total validator stake is zero")
		}
	}

	return nil
}

func (g *GenesisConfig) ValidateAndSanitize() error {
	if g.BlockTime < 500*time.Millisecond {
		g.BlockTime = 500 * time.Millisecond
	}

	if g.MaxBlockSize > 50*1024*1024 {
		g.MaxBlockSize = 50 * 1024 * 1024
	}

	return ValidateGenesis(g)
}

func (g *GenesisConfig) String() string {
	return fmt.Sprintf(
		"Genesis{ChainID: %d, Network: %s, Validators: %d, Supply: %d, BlockTime: %s}",
		g.ChainID,
		g.NetworkName(),
		len(g.InitialValidators),
		g.InitialSupply,
		g.BlockTime,
	)
}

func (g *GenesisConfig) NetworkName() string {
	switch g.ChainID {
	case 1:
		return "mainnet"
	case 1337:
		return "devnet"
	case 42069:
		return "testnet"
	default:
		return fmt.Sprintf("chain-%d", g.ChainID)
	}
}
