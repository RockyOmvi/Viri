package appchain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type AppChainType uint8

const (
	AppChainTypeGaming AppChainType = iota
	AppChainTypeDeFi
	AppChainTypeSocial
	AppChainTypeIdentity
	AppChainTypeCustom
)

type AppChainStatus uint8

const (
	AppChainStatusDeploying AppChainStatus = iota
	AppChainStatusActive
	AppChainStatusPaused
	AppChainStatusDecommissioned
)

type ValidatorConfig struct {
	Address []byte
	Stake   uint64
	PubKey  []byte
}

func (v *ValidatorConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Address string `json:"address"`
		Stake   uint64 `json:"stake"`
		PubKey  string `json:"pub_key"`
	}{
		Address: hex.EncodeToString(v.Address),
		Stake:   v.Stake,
		PubKey:  hex.EncodeToString(v.PubKey),
	})
}

type AppChainConfig struct {
	ChainID        string
	Name           string
	Type           AppChainType
	Status         AppChainStatus
	Owner          []byte
	Validators     []ValidatorConfig
	GasLimit       uint64
	BlockTime      time.Duration
	MaxValidators  int
	CreatedAt      time.Time
}

func (c *AppChainConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ChainID       string           `json:"chain_id"`
		Name          string           `json:"name"`
		Type          AppChainType     `json:"chain_type"`
		Status        AppChainStatus   `json:"status"`
		Owner         string           `json:"owner"`
		Validators    []ValidatorConfig `json:"validators"`
		GasLimit      uint64           `json:"gas_limit"`
		BlockTimeNano int64            `json:"block_time_ns"`
		MaxValidators int              `json:"max_validators"`
		CreatedAt     time.Time        `json:"created_at"`
	}{
		ChainID:       c.ChainID,
		Name:          c.Name,
		Type:          c.Type,
		Status:        c.Status,
		Owner:         hex.EncodeToString(c.Owner),
		Validators:    c.Validators,
		GasLimit:      c.GasLimit,
		BlockTimeNano: c.BlockTime.Nanoseconds(),
		MaxValidators: c.MaxValidators,
		CreatedAt:     c.CreatedAt,
	})
}

type AppChainManager struct {
	mu        sync.RWMutex
	chains    map[string]*AppChainConfig
	chainsByOwner map[string][]string
}

func NewAppChainManager() *AppChainManager {
	return &AppChainManager{
		chains:        make(map[string]*AppChainConfig),
		chainsByOwner: make(map[string][]string),
	}
}

func (m *AppChainManager) CreateAppChain(config AppChainConfig) (*AppChainConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.chains[config.ChainID]; exists {
		return nil, fmt.Errorf("app chain already exists")
	}

	if len(config.Validators) > config.MaxValidators {
		return nil, fmt.Errorf("too many validators")
	}

	if config.GasLimit == 0 {
		config.GasLimit = 30_000_000
	}

	if config.BlockTime == 0 {
		config.BlockTime = time.Second
	}

	config.Status = AppChainStatusActive
	config.CreatedAt = time.Now()

	m.chains[config.ChainID] = &config

	ownerKey := string(config.Owner)
	m.chainsByOwner[ownerKey] = append(m.chainsByOwner[ownerKey], config.ChainID)

	return &config, nil
}

func (m *AppChainManager) AddValidator(chainID string, validator ValidatorConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	chain, exists := m.chains[chainID]
	if !exists {
		return fmt.Errorf("app chain not found")
	}

	if len(chain.Validators) >= chain.MaxValidators {
		return fmt.Errorf("max validators reached")
	}

	chain.Validators = append(chain.Validators, validator)
	return nil
}

func (m *AppChainManager) RemoveValidator(chainID string, address []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	chain, exists := m.chains[chainID]
	if !exists {
		return fmt.Errorf("app chain not found")
	}

	for i, v := range chain.Validators {
		if string(v.Address) == string(address) {
			chain.Validators = append(chain.Validators[:i], chain.Validators[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("validator not found")
}

func (m *AppChainManager) PauseChain(chainID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	chain, exists := m.chains[chainID]
	if !exists {
		return fmt.Errorf("app chain not found")
	}

	chain.Status = AppChainStatusPaused
	return nil
}

func (m *AppChainManager) ResumeChain(chainID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	chain, exists := m.chains[chainID]
	if !exists {
		return fmt.Errorf("app chain not found")
	}

	if chain.Status != AppChainStatusPaused {
		return fmt.Errorf("chain not paused")
	}

	chain.Status = AppChainStatusActive
	return nil
}

func (m *AppChainManager) GetAppChain(chainID string) (*AppChainConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chain, exists := m.chains[chainID]
	if !exists {
		return nil, false
	}

	return chain, true
}

func (m *AppChainManager) GetOwnerChains(owner []byte) []*AppChainConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ownerKey := string(owner)
	chainIDs, exists := m.chainsByOwner[ownerKey]
	if !exists {
		return nil
	}

	var chains []*AppChainConfig
	for _, id := range chainIDs {
		if chain, exists := m.chains[id]; exists {
			chains = append(chains, chain)
		}
	}

	return chains
}

func (m *AppChainManager) GetActiveChains() []*AppChainConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []*AppChainConfig
	for _, chain := range m.chains {
		if chain.Status == AppChainStatusActive {
			active = append(active, chain)
		}
	}

	return active
}

func (m *AppChainManager) ChainCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.chains)
}
