package ledger

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"time"
)

func DefaultGenesis() *GenesisConfig {
	return &GenesisConfig{
		ChainID:          1,
		InitialValidators: make([]*ValidatorInfo, 0),
		InitialSupply:     1_000_000_000,
		BlockTime:         1 * time.Second,
		MaxBlockSize:      10 * 1024 * 1024,
		MaxGasPerBlock:    30_000_000,
	}
}

type rawGenesis struct {
	ChainID        uint64           `json:"chain_id"`
	Network        string           `json:"network_name,omitempty"`
	GenesisTime    string           `json:"genesis_time,omitempty"`
	Validators     []rawValidator   `json:"validators"`
	TotalStake     uint64           `json:"total_stake,omitempty"`
	BlockTime      string           `json:"block_time,omitempty"`
	MaxBlockSize   uint64           `json:"max_block_size,omitempty"`
	MaxGasPerBlock uint64           `json:"max_gas_per_block,omitempty"`
}

type rawValidator struct {
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
	Stake     uint64 `json:"stake"`
	Name      string `json:"name,omitempty"`
}

func hexDecode(s string) []byte {
	s = strings.TrimPrefix(s, "0x")
	b, _ := hex.DecodeString(s)
	return b
}

func LoadGenesis(path string) (*GenesisConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw rawGenesis
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	config := &GenesisConfig{
		ChainID:        raw.ChainID,
		Network:        raw.Network,
		GenesisTime:    raw.GenesisTime,
		InitialSupply:  raw.TotalStake,
		MaxBlockSize:   raw.MaxBlockSize,
		MaxGasPerBlock: raw.MaxGasPerBlock,
	}

	if raw.BlockTime != "" {
		config.BlockTime, _ = time.ParseDuration(raw.BlockTime)
	}
	if config.BlockTime == 0 {
		config.BlockTime = time.Second
	}
	if config.MaxBlockSize == 0 {
		config.MaxBlockSize = 10 * 1024 * 1024
	}
	if config.MaxGasPerBlock == 0 {
		config.MaxGasPerBlock = 30_000_000
	}

	for _, rv := range raw.Validators {
		config.InitialValidators = append(config.InitialValidators, &ValidatorInfo{
			Address:   hexDecode(rv.Address),
			PublicKey: hexDecode(rv.PublicKey),
			Stake:     rv.Stake,
			Name:      rv.Name,
		})
	}

	return config, nil
}

func (g *GenesisConfig) Save(path string) error {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func TestGenesis() *GenesisConfig {
	genesis := DefaultGenesis()
	genesis.ChainID = 1337
	genesis.BlockTime = 500 * time.Millisecond
	genesis.MaxGasPerBlock = 10_000_000
	return genesis
}
