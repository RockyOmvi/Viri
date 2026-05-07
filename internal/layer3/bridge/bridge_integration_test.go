package bridge

import "testing"

func TestGetPendingTransfers(t *testing.T) {
	br := NewChainBridge(1)
	br.RegisterChain("a", "A", "a")
	br.RegisterChain("b", "B", "b")
	_, _ = br.InitiateTransfer("a", "b", []byte("s"), []byte("r"), 1, []byte("T"))

	pending := br.GetPendingTransfers()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending")
	}
}
