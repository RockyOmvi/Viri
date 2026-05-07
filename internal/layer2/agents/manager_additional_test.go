package agents

import "testing"

func TestGetAgentMissing(t *testing.T) {
	am := NewAgentManager()
	if _, ok := am.GetAgent("missing"); ok {
		t.Fatalf("unexpected agent")
	}
}
