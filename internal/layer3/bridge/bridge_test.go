package bridge

import "testing"

func TestRegisterChainAndInitiateTransfer(t *testing.T) {
	br := NewChainBridge(2)
	br.RegisterChain("a", "A", "a")
	br.RegisterChain("b", "B", "b")

	if _, err := br.InitiateTransfer("a", "c", []byte("s"), []byte("r"), 1, []byte("T")); err == nil {
		t.Fatalf("expected missing dest error")
	}

	if _, err := br.InitiateTransfer("c", "b", []byte("s"), []byte("r"), 1, []byte("T")); err == nil {
		t.Fatalf("expected missing source error")
	}

	transfer, err := br.InitiateTransfer("a", "b", []byte("s"), []byte("r"), 1, []byte("T"))
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if transfer.Status != TransferStatusPending {
		t.Fatalf("expected pending")
	}
}

func TestLockCompleteAndSignatures(t *testing.T) {
	br := NewChainBridge(2)
	br.RegisterChain("a", "A", "a")
	br.RegisterChain("b", "B", "b")
	br.RegisterValidator("v1")
	br.RegisterValidator("v2")

	transfer, _ := br.InitiateTransfer("a", "b", []byte("s"), []byte("r"), 1, []byte("T"))

	if err := br.LockTokens("missing", []byte("tx")); err == nil {
		t.Fatalf("expected missing transfer error")
	}
	if err := br.LockTokens(transfer.ID, []byte("tx")); err != nil {
		t.Fatalf("lock failed: %v", err)
	}
	if transfer.Status != TransferStatusLocked {
		t.Fatalf("expected locked status")
	}

	if err := br.AddValidatorSignature(transfer.ID, "unknown"); err == nil {
		t.Fatalf("expected unknown validator error")
	}
	if err := br.AddValidatorSignature(transfer.ID, "v1"); err != nil {
		t.Fatalf("sig failed: %v", err)
	}
	if err := br.AddValidatorSignature(transfer.ID, "v2"); err != nil {
		t.Fatalf("sig failed: %v", err)
	}
	if transfer.Status != TransferStatusCompleted {
		t.Fatalf("expected completed after signatures")
	}

	if err := br.CompleteTransfer(transfer.ID, []byte("mint")); err != nil {
		t.Fatalf("complete failed: %v", err)
	}
}

func TestCompleteTransferInsufficientSigs(t *testing.T) {
	br := NewChainBridge(2)
	br.RegisterChain("a", "A", "a")
	br.RegisterChain("b", "B", "b")
	br.RegisterValidator("v1")

	transfer, _ := br.InitiateTransfer("a", "b", []byte("s"), []byte("r"), 1, []byte("T"))
	if err := br.AddValidatorSignature(transfer.ID, "v1"); err != nil {
		t.Fatalf("sig failed: %v", err)
	}
	if err := br.CompleteTransfer(transfer.ID, []byte("mint")); err == nil {
		t.Fatalf("expected insufficient sigs error")
	}
}

func TestGetTransferAndPending(t *testing.T) {
	br := NewChainBridge(1)
	br.RegisterChain("a", "A", "a")
	br.RegisterChain("b", "B", "b")
	transfer, _ := br.InitiateTransfer("a", "b", []byte("s"), []byte("r"), 1, []byte("T"))

	if _, ok := br.GetTransfer("missing"); ok {
		t.Fatalf("unexpected transfer")
	}

	loaded, ok := br.GetTransfer(transfer.ID)
	if !ok || loaded.ID != transfer.ID {
		t.Fatalf("transfer not found")
	}

	pending := br.GetPendingTransfers()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending")
	}
}
