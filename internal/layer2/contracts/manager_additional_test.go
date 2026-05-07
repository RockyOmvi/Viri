package contracts

import "testing"

func TestGetContractMissing(t *testing.T) {
	cm := NewContractManager()
	if _, ok := cm.GetContract([]byte("missing")); ok {
		t.Fatalf("unexpected contract")
	}
}
