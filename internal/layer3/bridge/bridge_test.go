package bridge

import (
	"math/big"
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

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

func TestBridgeStateCryptoSignatureVerification(t *testing.T) {
	bs := NewBridgeState()

	privKey1, _ := crypto.GenerateKey()
	privKey2, _ := crypto.GenerateKey()

	vs := &ValidatorSet{
		Validators: []*BridgeValidator{
			{
				Address:   privKey1.PubKey().Address(),
				PublicKey: privKey1.PubKey().Bytes(),
				Stake:     100,
				IsActive:  true,
			},
			{
				Address:   privKey2.PubKey().Address(),
				PublicKey: privKey2.PubKey().Bytes(),
				Stake:     100,
				IsActive:  true,
			},
		},
		Threshold: 2,
	}

	bs.RegisterValidatorSet(Viri, vs)

	msg, err := bs.CreateBridgeMessage(Viri, Ethereum, []byte("sender"), []byte("receiver"), []byte("token"), big.NewInt(1000), nil)
	if err != nil {
		t.Fatalf("create message failed: %v", err)
	}

	// 1. Invalid signature should be rejected
	badSig := make([]byte, 64)
	if err := bs.AddSignature(msg.ID, 0, badSig); err == nil {
		t.Fatalf("expected error adding invalid signature")
	}

	// 2. Valid signature from validator 0
	sig1, err := privKey1.Sign(msg.ID)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	if err := bs.AddSignature(msg.ID, 0, sig1.Bytes()); err != nil {
		t.Fatalf("failed to add valid signature 0: %v", err)
	}

	// Message should still be Pending because threshold 2/3 of 200 stake = 134 stake required
	if msg.Status != Pending {
		t.Fatalf("expected status Pending after 1 signature, got %v", msg.Status)
	}

	// 3. Valid signature from validator 1
	sig2, err := privKey2.Sign(msg.ID)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	if err := bs.AddSignature(msg.ID, 1, sig2.Bytes()); err != nil {
		t.Fatalf("failed to add valid signature 1: %v", err)
	}

	// Status should now be Confirmed (200 >= 134)
	if msg.Status != Confirmed {
		t.Fatalf("expected status Confirmed after 2 signatures, got %v", msg.Status)
	}

	// 4. Verify message via VerifyMessage
	if err := bs.VerifyMessage(msg); err != nil {
		t.Fatalf("VerifyMessage failed: %v", err)
	}

	// 5. Tampered signature in VerifyMessage should fail
	msg.Signatures[0] = badSig
	if err := bs.VerifyMessage(msg); err == nil {
		t.Fatalf("expected VerifyMessage to fail with tampered signature")
	}
}
