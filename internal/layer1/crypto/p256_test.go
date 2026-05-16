package crypto

import (
	"bytes"
	"testing"
)

func TestP256GenerateKey(t *testing.T) {
	key, err := GenerateP256Key()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if key == nil {
		t.Fatal("key is nil")
	}
	if len(key.PrivateBytes()) == 0 {
		t.Errorf("private key bytes empty")
	}
}

func TestP256SignAndVerify(t *testing.T) {
	key, err := GenerateP256Key()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	data := []byte("hello p256 world")
	sig, err := key.Sign(data)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if !key.PubKey().Verify(data, sig) {
		t.Errorf("valid signature rejected")
	}

	sigBytes := sig.Bytes()
	if len(sigBytes) != 64 {
		t.Errorf("expected 64-byte signature, got %d", len(sigBytes))
	}

	if !key.PubKey().VerifyMessage(data, sigBytes) {
		t.Errorf("valid raw signature rejected")
	}
}

func TestP256VerifyTampered(t *testing.T) {
	key, err := GenerateP256Key()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	sig, err := key.Sign([]byte("original"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if key.PubKey().Verify([]byte("tampered"), sig) {
		t.Errorf("tampered data should not verify")
	}
}

func TestP256KeyBytes(t *testing.T) {
	key, err := GenerateP256Key()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	privBytes := key.PrivateBytes()
	restored, err := P256PrivateKeyFromBytes(privBytes)
	if err != nil {
		t.Fatalf("restore private key: %v", err)
	}

	data := []byte("roundtrip p256")
	sig, err := restored.Sign(data)
	if err != nil {
		t.Fatalf("restored key sign: %v", err)
	}

	if !restored.PubKey().Verify(data, sig) {
		t.Errorf("restored key verify failed")
	}

	pubBytes := key.PublicBytes()
	pubRestored, err := P256PubKeyFromBytes(pubBytes)
	if err != nil {
		t.Fatalf("restore public key: %v", err)
	}

	if !pubRestored.Verify(data, sig) {
		t.Errorf("restored pubkey verify failed")
	}
}

func TestP256Address(t *testing.T) {
	key, err := GenerateP256Key()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	addr := key.PubKey().Address()
	if len(addr) != 20 {
		t.Errorf("expected 20-byte address, got %d", len(addr))
	}

	addr2 := key.PubKey().Address()
	if !bytes.Equal(addr, addr2) {
		t.Errorf("address not deterministic")
	}
}

func TestP256SignMessageRaw(t *testing.T) {
	key, err := GenerateP256Key()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	sigBytes, err := key.SignMessage([]byte("raw sign"))
	if err != nil {
		t.Fatalf("sign message: %v", err)
	}

	if len(sigBytes) != 64 {
		t.Errorf("expected 64-byte signature, got %d", len(sigBytes))
	}

	if !key.VerifyMessage([]byte("raw sign"), sigBytes) {
		t.Errorf("verify raw sign failed")
	}

	if key.VerifyMessage([]byte("wrong data"), sigBytes) {
		t.Errorf("tampered data should not verify")
	}
}

func TestP256InvalidSignatureLength(t *testing.T) {
	key, err := GenerateP256Key()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	if key.PubKey().VerifyMessage([]byte("test"), []byte{0x01}) {
		t.Errorf("short signature should not verify")
	}
}

func TestP256InvalidKeyBytes(t *testing.T) {
	if _, err := P256PrivateKeyFromBytes(nil); err == nil {
		t.Errorf("nil bytes should error")
	}
	if _, err := P256PubKeyFromBytes(nil); err == nil {
		t.Errorf("nil pubkey bytes should error")
	}
	if _, err := P256PubKeyFromBytes([]byte{0x00}); err == nil {
		t.Errorf("invalid pubkey bytes should error")
	}
}
