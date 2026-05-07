package genesis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"
)

type GenesisConfig struct {
	ChainID       uint64             `json:"chain_id"`
	Network       string             `json:"network"`
	Timestamp     uint64             `json:"timestamp"`
	Validators    []GenesisValidator `json:"validators"`
	Accounts      []GenesisAccount   `json:"accounts"`
	Parameters    GenesisParameters  `json:"parameters"`
	Allocations   map[string]*big.Int `json:"allocations"`
}

type GenesisValidator struct {
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
	Stake     uint64 `json:"stake"`
	Name      string `json:"name"`
}

type GenesisAccount struct {
	Address string `json:"address"`
	Balance string `json:"balance"`
	Code    string `json:"code,omitempty"`
	Storage map[string]string `json:"storage,omitempty"`
}

type GenesisParameters struct {
	BlockGasLimit  uint64 `json:"block_gas_limit"`
	MaxTxSize      uint64 `json:"max_tx_size"`
	EpochLength    uint64 `json:"epoch_length"`
	MinValidators  int    `json:"min_validators"`
	MaxValidators  int    `json:"max_validators"`
	BlockTime      uint64 `json:"block_time"`
	BaseFee        string `json:"base_fee"`
}

type GenesisParticipant struct {
	Address    string `json:"address"`
	PublicKey  string `json:"public_key"`
	Stake      uint64 `json:"stake"`
	Signature  string `json:"signature"`
	Committed  bool   `json:"committed"`
}

type GenesisCeremony struct {
	Participants []GenesisParticipant
	Required     int
	Config       GenesisConfig
	Phase        CeremonyPhase
}

type CeremonyPhase int

const (
	PhaseRegistration CeremonyPhase = iota
	PhaseCommitment
	PhaseReveal
	PhaseFinalization
)

func NewGenesisCeremony(required int) *GenesisCeremony {
	return &GenesisCeremony{
		Participants: make([]GenesisParticipant, 0),
		Required:     required,
		Config: GenesisConfig{
			ChainID:     7777777,
			Network:     "viri-mainnet",
			Timestamp:   uint64(time.Now().Unix()),
			Allocations: make(map[string]*big.Int),
		},
		Phase: PhaseRegistration,
	}
}

func (gc *GenesisCeremony) RegisterParticipant(participant GenesisParticipant) error {
	if gc.Phase != PhaseRegistration {
		return fmt.Errorf("registration phase closed")
	}

	for _, p := range gc.Participants {
		if p.Address == participant.Address {
			return fmt.Errorf("participant already registered")
		}
	}

	gc.Participants = append(gc.Participants, participant)
	return nil
}

func (gc *GenesisCeremony) Commit(participantAddr string, commitment []byte) error {
	if gc.Phase != PhaseCommitment {
		return fmt.Errorf("not in commitment phase")
	}

	for i, p := range gc.Participants {
		if p.Address == participantAddr {
			gc.Participants[i].Committed = true
			return nil
		}
	}

	return fmt.Errorf("participant not found")
}

func (gc *GenesisCeremony) Reveal(participantAddr string, data []byte) error {
	if gc.Phase != PhaseReveal {
		return fmt.Errorf("not in reveal phase")
	}

	return nil
}

func (gc *GenesisCeremony) AddValidator(addr, pubkey string, stake uint64) {
	gc.Config.Validators = append(gc.Config.Validators, GenesisValidator{
		Address:   addr,
		PublicKey: pubkey,
		Stake:     stake,
	})
}

func (gc *GenesisCeremony) AddAccount(address string, balance *big.Int) {
	gc.Config.Allocations[address] = balance
	gc.Config.Accounts = append(gc.Config.Accounts, GenesisAccount{
		Address: address,
		Balance: balance.String(),
	})
}

func (gc *GenesisCeremony) SetParameters(params GenesisParameters) {
	gc.Config.Parameters = params
}

func (gc *GenesisCeremony) Finalize() (*GenesisConfig, error) {
	if len(gc.Participants) < gc.Required {
		return nil, fmt.Errorf("insufficient participants: %d < %d", len(gc.Participants), gc.Required)
	}

	if len(gc.Config.Validators) == 0 {
		return nil, fmt.Errorf("no validators configured")
	}

	gc.Config.Timestamp = uint64(time.Now().Unix())
	gc.Phase = PhaseFinalization

	return &gc.Config, nil
}

func (gc *GenesisConfig) Validate() error {
	if gc.ChainID == 0 {
		return fmt.Errorf("chain_id must be positive")
	}

	if len(gc.Validators) == 0 {
		return fmt.Errorf("at least one validator required")
	}

	for _, v := range gc.Validators {
		if v.Address == "" {
			return fmt.Errorf("validator address required")
		}
		if v.PublicKey == "" {
			return fmt.Errorf("validator public key required")
		}
		if v.Stake == 0 {
			return fmt.Errorf("validator stake required")
		}
	}

	for _, acc := range gc.Accounts {
		if acc.Address == "" {
			return fmt.Errorf("account address required")
		}
		if _, err := hex.DecodeString(acc.Address); err != nil {
			return fmt.Errorf("invalid account address format: %s", acc.Address)
		}
	}

	if gc.Parameters.BlockGasLimit == 0 {
		return fmt.Errorf("block_gas_limit must be positive")
	}

	return nil
}

func (gc *GenesisConfig) ComputeHash() []byte {
	h := sha256.New()

	data, _ := json.Marshal(gc)
	h.Write(data)

	return h.Sum(nil)
}

func (gc *GenesisConfig) SaveToFile(path string) error {
	data, err := json.MarshalIndent(gc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func LoadGenesisFromFile(path string) (*GenesisConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read genesis file: %w", err)
	}

	var config GenesisConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse genesis: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid genesis config: %w", err)
	}

	return &config, nil
}

func DefaultGenesis() *GenesisConfig {
	return &GenesisConfig{
		ChainID:   7777777,
		Network:   "viri-devnet",
		Timestamp: uint64(time.Now().Unix()),
		Validators: []GenesisValidator{
			{
				Address:   "0x0000000000000000000000000000000000000001",
				PublicKey: "0x01",
				Stake:     1000000,
				Name:      "genesis-validator",
			},
		},
		Accounts: []GenesisAccount{
			{
				Address: "0x0000000000000000000000000000000000000001",
				Balance: "1000000000000000000000",
			},
		},
		Parameters: GenesisParameters{
			BlockGasLimit: 30000000,
			MaxTxSize:     128000,
			EpochLength:   1000,
			MinValidators: 1,
			MaxValidators: 100,
			BlockTime:     2,
			BaseFee:       "1000000000",
		},
		Allocations: make(map[string]*big.Int),
	}
}
