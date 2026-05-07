package agents

import (
	"fmt"
	"sync"
)

type AgentType uint8

const (
	AgentTypeValidator AgentType = iota
	AgentTypeSequencer
	AgentTypeRelayer
	AgentTypeKeeper
)

type Agent struct {
	ID        string
	Type      AgentType
	Address   []byte
	Stake     uint64
	IsActive  bool
	Metadata  map[string]string
}

type AgentManager struct {
	mu    sync.RWMutex
	agents map[string]*Agent
}

func NewAgentManager() *AgentManager {
	return &AgentManager{
		agents: make(map[string]*Agent),
	}
}

func (am *AgentManager) Register(id string, agentType AgentType, address []byte, stake uint64) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.agents[id]; exists {
		return fmt.Errorf("agent already registered")
	}

	am.agents[id] = &Agent{
		ID:       id,
		Type:     agentType,
		Address:  address,
		Stake:    stake,
		IsActive: true,
		Metadata: make(map[string]string),
	}

	return nil
}

func (am *AgentManager) GetAgent(id string) (*Agent, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	agent, exists := am.agents[id]
	if !exists {
		return nil, false
	}

	return agent, true
}

func (am *AgentManager) GetAgentsByType(agentType AgentType) []*Agent {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []*Agent
	for _, agent := range am.agents {
		if agent.Type == agentType && agent.IsActive {
			result = append(result, agent)
		}
	}

	return result
}

func (am *AgentManager) SetMetadata(id string, key, value string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	agent, exists := am.agents[id]
	if !exists {
		return fmt.Errorf("agent not found")
	}

	agent.Metadata[key] = value
	return nil
}

func (am *AgentManager) Deactivate(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	agent, exists := am.agents[id]
	if !exists {
		return fmt.Errorf("agent not found")
	}

	agent.IsActive = false
	return nil
}

func (am *AgentManager) ActiveCount() int {
	am.mu.RLock()
	defer am.mu.RUnlock()

	count := 0
	for _, agent := range am.agents {
		if agent.IsActive {
			count++
		}
	}

	return count
}
