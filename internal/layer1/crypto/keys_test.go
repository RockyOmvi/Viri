package crypto

import (
	"testing"
)

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	if key == nil {
		t.Fatal("Key is nil")
	}
	if key.PubKey() == nil {
		t.Fatal("Public key is nil")
	}
}

func TestSignAndVerify(t *testing.T) {
	key, _ := GenerateKey()
	data := []byte("test message")

	sig, err := key.Sign(data)
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	if !key.PubKey().Verify(data, sig) {
		t.Fatal("Signature verification failed")
	}
}

func TestInvalidSignature(t *testing.T) {
	key, _ := GenerateKey()
	data := []byte("test message")

	sig, _ := key.Sign(data)

	if key.PubKey().Verify([]byte("wrong data"), sig) {
		t.Fatal("Should have failed verification for wrong data")
	}
}

func TestAddressGeneration(t *testing.T) {
	key, _ := GenerateKey()
	addr := key.PubKey().Address()

	if len(addr) == 0 {
		t.Fatal("Address is empty")
	}
}

func TestSHA256(t *testing.T) {
	hash := SHA256([]byte("test"))
	if len(hash) != 32 {
		t.Fatalf("Expected 32 bytes, got %d", len(hash))
	}

	hash1 := SHA256([]byte("test"))
	hash2 := SHA256([]byte("test"))

	for i := range hash1 {
		if hash1[i] != hash2[i] {
			t.Fatal("Same input should produce same hash")
		}
	}
}

func TestDoubleSHA256(t *testing.T) {
	hash := DoubleSHA256([]byte("test"))
	if len(hash) != 32 {
		t.Fatalf("Expected 32 bytes, got %d", len(hash))
	}
}
