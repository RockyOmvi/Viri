package ledger

import (
	"testing"
	"time"
)

func TestValidateGenesis(t *testing.T) {
	validGenesis := &GenesisConfig{
		ChainID:     1,
		InitialSupply: 1_000_000_000,
		BlockTime:   time.Second,
		MaxBlockSize: 10 * 1024 * 1024,
		MaxGasPerBlock: 30_000_000,
	}

	if err := ValidateGenesis(validGenesis); err != nil {
		t.Errorf("Valid genesis should pass validation: %v", err)
	}
}

func TestValidateGenesisErrors(t *testing.T) {
	tests := []struct {
		name   string
		config *GenesisConfig
	}{
		{
			name: "zero chain_id",
			config: &GenesisConfig{
				ChainID:     0,
				InitialSupply: 1_000_000_000,
				BlockTime:   time.Second,
				MaxBlockSize: 10 * 1024 * 1024,
				MaxGasPerBlock: 30_000_000,
			},
		},
		{
			name: "zero initial supply",
			config: &GenesisConfig{
				ChainID:     1,
				InitialSupply: 0,
				BlockTime:   time.Second,
				MaxBlockSize: 10 * 1024 * 1024,
				MaxGasPerBlock: 30_000_000,
			},
		},
		{
			name: "zero block time",
			config: &GenesisConfig{
				ChainID:     1,
				InitialSupply: 1_000_000_000,
				BlockTime:   0,
				MaxBlockSize: 10 * 1024 * 1024,
				MaxGasPerBlock: 30_000_000,
			},
		},
		{
			name: "block time too low",
			config: &GenesisConfig{
				ChainID:     1,
				InitialSupply: 1_000_000_000,
				BlockTime:   50 * time.Millisecond,
				MaxBlockSize: 10 * 1024 * 1024,
				MaxGasPerBlock: 30_000_000,
			},
		},
		{
			name: "zero max block size",
			config: &GenesisConfig{
				ChainID:     1,
				InitialSupply: 1_000_000_000,
				BlockTime:   time.Second,
				MaxBlockSize: 0,
				MaxGasPerBlock: 30_000_000,
			},
		},
		{
			name: "max block size too large",
			config: &GenesisConfig{
				ChainID:     1,
				InitialSupply: 1_000_000_000,
				BlockTime:   time.Second,
				MaxBlockSize: 200 * 1024 * 1024,
				MaxGasPerBlock: 30_000_000,
			},
		},
		{
			name: "zero max gas per block",
			config: &GenesisConfig{
				ChainID:     1,
				InitialSupply: 1_000_000_000,
				BlockTime:   time.Second,
				MaxBlockSize: 10 * 1024 * 1024,
				MaxGasPerBlock: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGenesis(tt.config)
			if err == nil {
				t.Error("Expected validation error, got nil")
			}
		})
	}
}

func TestValidateGenesisValidators(t *testing.T) {
	genesis := &GenesisConfig{
		ChainID:     1,
		InitialSupply: 1_000_000_000,
		BlockTime:   time.Second,
		MaxBlockSize: 10 * 1024 * 1024,
		MaxGasPerBlock: 30_000_000,
		InitialValidators: []*ValidatorInfo{
			{
				Address:   []byte("validator-1"),
				PublicKey: []byte("pubkey-1"),
				Stake:     1000,
			},
			{
				Address:   []byte("validator-2"),
				PublicKey: []byte("pubkey-2"),
				Stake:     2000,
			},
		},
	}

	if err := ValidateGenesis(genesis); err != nil {
		t.Errorf("Valid genesis with validators should pass: %v", err)
	}
}

func TestValidateGenesisDuplicateValidator(t *testing.T) {
	genesis := &GenesisConfig{
		ChainID:     1,
		InitialSupply: 1_000_000_000,
		BlockTime:   time.Second,
		MaxBlockSize: 10 * 1024 * 1024,
		MaxGasPerBlock: 30_000_000,
		InitialValidators: []*ValidatorInfo{
			{
				Address:   []byte("validator-1"),
				PublicKey: []byte("pubkey-1"),
				Stake:     1000,
			},
			{
				Address:   []byte("validator-1"),
				PublicKey: []byte("pubkey-2"),
				Stake:     2000,
			},
		},
	}

	if err := ValidateGenesis(genesis); err == nil {
		t.Error("Expected duplicate validator error")
	}
}

func TestGenesisString(t *testing.T) {
	genesis := TestGenesis()
	str := genesis.String()

	if str == "" {
		t.Error("Genesis string should not be empty")
	}
}

func TestGenesisNetworkName(t *testing.T) {
	tests := []struct {
		chainID    uint64
		expectName string
	}{
		{1, "mainnet"},
		{1337, "devnet"},
		{42069, "testnet"},
		{9999, "chain-9999"},
	}

	for _, tt := range tests {
		t.Run(tt.expectName, func(t *testing.T) {
			genesis := &GenesisConfig{ChainID: tt.chainID}
			if name := genesis.NetworkName(); name != tt.expectName {
				t.Errorf("Expected network name %s, got %s", tt.expectName, name)
			}
		})
	}
}

func TestGenesisValidateAndSanitize(t *testing.T) {
	genesis := &GenesisConfig{
		ChainID:     1,
		InitialSupply: 1_000_000_000,
		BlockTime:   100 * time.Millisecond,
		MaxBlockSize: 200 * 1024 * 1024,
		MaxGasPerBlock: 30_000_000,
	}

	if err := genesis.ValidateAndSanitize(); err != nil {
		t.Errorf("Genesis should be valid after sanitization: %v", err)
	}

	if genesis.BlockTime < 500*time.Millisecond {
		t.Error("Block time should be sanitized to minimum 500ms")
	}

	if genesis.MaxBlockSize > 50*1024*1024 {
		t.Error("Max block size should be sanitized to maximum 50MB")
	}
}
