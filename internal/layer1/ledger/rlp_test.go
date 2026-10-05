package ledger

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func TestDecodeEIP1559Transaction(t *testing.T) {
	// Raw EIP-1559 transaction signed by private key e0c6a3a0b5b297e5ea86409765363f71b26f3fa2e0e61f439e7e6e589839f290
	// Expected sender: e7a8df9bac0bd7bdf244bcf150b0d64bf088ad87
	rawHex := "02f87482053901843b9aca00847735940082520894a29539c21ac730f8c0501b719ed7443bb4d16ab188016345785d8a000080c001a0686458087afed4214d0140b71d83e95bbf7f4b9e3ad05070cb8286fcf415d2c9a065e01d8ac156eda6f0a2108f6dbe42df1042bfa7c671a302c0d9ef2c5e1f7746"
	raw, err := hex.DecodeString(rawHex)
	if err != nil {
		t.Fatalf("failed to decode hex: %v", err)
	}

	tx, err := DecodeRLPTransaction(raw)
	if err != nil {
		t.Fatalf("DecodeRLPTransaction failed: %v", err)
	}

	if tx.ChainID != 1337 {
		t.Errorf("expected chainID 1337, got %d", tx.ChainID)
	}
	if tx.Nonce != 1 {
		t.Errorf("expected nonce 1, got %d", tx.Nonce)
	}

	expectedSigningHash, _ := hex.DecodeString("3e01bb6598312e5efcdd2d58606e2a83471cac5743a16325957f8296d1beb30d")
	actualSigningHash := tx.SigningHash()
	if !bytes.Equal(actualSigningHash, expectedSigningHash) {
		t.Errorf("signing hash mismatch:\n  got:  %x\n  want: %x", actualSigningHash, expectedSigningHash)
	}

	expectedTxHash, _ := hex.DecodeString("320190ad06bbfe388eaa174b988951a1cfda73e0d01a1b6b19f47e6f49ff8534")
	actualTxHash := tx.TxHash()
	if !bytes.Equal(actualTxHash, expectedTxHash) {
		t.Errorf("tx hash mismatch:\n  got:  %x\n  want: %x", actualTxHash, expectedTxHash)
	}

	pubBytes, err := tx.RecoverPubKey()
	if err != nil {
		t.Fatalf("RecoverPubKey failed: %v", err)
	}
	pubKey, err := crypto.PubKeyFromBytes(pubBytes)
	if err != nil {
		t.Fatalf("PubKeyFromBytes failed: %v", err)
	}

	expectedAddr, _ := hex.DecodeString("e7a8df9bac0bd7bdf244bcf150b0d64bf088ad87")
	if !bytes.Equal(pubKey.Address(), expectedAddr) {
		t.Errorf("recovered sender address mismatch:\n  got:  %x\n  want: %x", pubKey.Address(), expectedAddr)
	}

	ledgerTx, err := tx.ToTransaction()
	if err != nil {
		t.Fatalf("ToTransaction failed: %v", err)
	}
	if !bytes.Equal(ledgerTx.SenderAddress(), expectedAddr) {
		t.Errorf("ledgerTx.SenderAddress mismatch:\n  got:  %x\n  want: %x", ledgerTx.SenderAddress(), expectedAddr)
	}
	if !ledgerTx.Verify() {
		t.Errorf("ledgerTx.Verify returned false")
	}
}
