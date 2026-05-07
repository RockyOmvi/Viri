package agents

import "testing"

func TestRegisterAndGetAgent(t *testing.T) {
	am := NewAgentManager()

	if err := am.Register("id1", AgentTypeValidator, []byte("addr"), 100); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if err := am.Register("id1", AgentTypeValidator, []byte("addr"), 100); err == nil {
		t.Fatalf("expected duplicate error")
	}

	agent, ok := am.GetAgent("id1")
	if !ok {
		t.Fatalf("agent not found")
	}
	if agent.Stake != 100 || agent.Type != AgentTypeValidator {
		t.Fatalf("agent mismatch")
	}
}

func TestGetAgentsByType(t *testing.T) {
	am := NewAgentManager()
	_ = am.Register("id1", AgentTypeValidator, []byte("a"), 1)
	_ = am.Register("id2", AgentTypeSequencer, []byte("b"), 2)
	_ = am.Register("id3", AgentTypeValidator, []byte("c"), 3)

	list := am.GetAgentsByType(AgentTypeValidator)
	if len(list) != 2 {
		t.Fatalf("expected 2 validators, got %d", len(list))
	}
}

func TestMetadataAndDeactivate(t *testing.T) {
	am := NewAgentManager()
	_ = am.Register("id1", AgentTypeValidator, []byte("a"), 1)

	if err := am.SetMetadata("id1", "k", "v"); err != nil {
		t.Fatalf("metadata failed: %v", err)
	}

	if err := am.SetMetadata("missing", "k", "v"); err == nil {
		t.Fatalf("expected missing agent error")
	}

	if err := am.Deactivate("missing"); err == nil {
		t.Fatalf("expected missing agent error")
	}

	if err := am.Deactivate("id1"); err != nil {
		t.Fatalf("deactivate failed: %v", err)
	}

	if am.ActiveCount() != 0 {
		t.Fatalf("expected 0 active agents")
	}
}

func TestActiveCount(t *testing.T) {
	am := NewAgentManager()
	if am.ActiveCount() != 0 {
		t.Fatalf("expected 0 active")
	}
	_ = am.Register("id1", AgentTypeValidator, []byte("a"), 1)
	_ = am.Register("id2", AgentTypeSequencer, []byte("b"), 1)
	if am.ActiveCount() != 2 {
		t.Fatalf("expected 2 active")
	}
}
