package contracts

import (
	"testing"
)

func TestDeployAndGetContract(t *testing.T) {
	cm := NewContractManager()
	owner := []byte("owner")
	code := []byte{0x00, 0x01}

	contract, err := cm.Deploy(owner, code, 1)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	if len(contract.Address) != 20 {
		t.Fatalf("expected 20-byte address")
	}

	loaded, ok := cm.GetContract(contract.Address)
	if !ok {
		t.Fatalf("contract not found")
	}
	if string(loaded.Code) != string(code) {
		t.Fatalf("code mismatch")
	}
}

func TestExecuteMissingContract(t *testing.T) {
	cm := NewContractManager()
	if _, err := cm.Execute([]byte("missing"), []byte("input"), 1); err == nil {
		t.Fatalf("expected missing contract error")
	}
}

func TestHasContract(t *testing.T) {
	cm := NewContractManager()
	owner := []byte("owner")
	code := []byte{0x00}
	contract, _ := cm.Deploy(owner, code, 1)
	if !cm.HasContract(contract.Address) {
		t.Fatalf("expected contract to exist")
	}
	if cm.HasContract([]byte("missing")) {
		t.Fatalf("unexpected contract")
	}
}

func TestBalances(t *testing.T) {
	cm := NewContractManager()
	owner := []byte("owner")
	contract, _ := cm.Deploy(owner, []byte{0x00}, 1)

	if err := cm.SetBalance([]byte("missing"), 10); err == nil {
		t.Fatalf("expected missing contract error")
	}

	if err := cm.SetBalance(contract.Address, 42); err != nil {
		t.Fatalf("set balance failed: %v", err)
	}

	bal, err := cm.GetBalance(contract.Address)
	if err != nil {
		t.Fatalf("get balance failed: %v", err)
	}
	if bal != 42 {
		t.Fatalf("expected balance 42, got %d", bal)
	}

	if _, err := cm.GetBalance([]byte("missing")); err == nil {
		t.Fatalf("expected missing contract error")
	}
}

func TestContractClone(t *testing.T) {
	cm := NewContractManager()
	owner := []byte("owner")
	contract, _ := cm.Deploy(owner, []byte{0x01, 0x02}, 1)
	clone, _ := cm.GetContract(contract.Address)

	clone.Owner[0] = 'x'
	clone.Code[0] = 0xFF

	orig, _ := cm.GetContract(contract.Address)
	if orig.Owner[0] == 'x' || orig.Code[0] == 0xFF {
		t.Fatalf("clone should not mutate original")
	}
}
