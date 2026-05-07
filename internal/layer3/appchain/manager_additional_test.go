package appchain

import "testing"

func TestGetAppChainMissing(t *testing.T) {
	mgr := NewAppChainManager()
	if _, ok := mgr.GetAppChain("missing"); ok {
		t.Fatalf("unexpected chain")
	}
}
