package tee

import (
	"bytes"
	"testing"
)

func TestNewEnclave(t *testing.T) {
	code := []byte("test code")
	enclave, err := NewEnclave(code)
	if err != nil {
		t.Fatalf("NewEnclave failed: %v", err)
	}
	if enclave == nil {
		t.Fatal("expected non-nil enclave")
	}
	if !enclave.initialized {
		t.Fatal("enclave should be initialized")
	}
	var zeroID EnclaveID
	if enclave.id == zeroID {
		t.Fatal("enclave ID should not be zero")
	}
	if enclave.ID() != enclave.id {
		t.Fatal("ID() mismatch")
	}
}

func TestSealUnseal(t *testing.T) {
	enclave, err := NewEnclave([]byte("test"))
	if err != nil {
		t.Fatalf("NewEnclave failed: %v", err)
	}

	plaintext := []byte("sensitive data")
	sealed, err := enclave.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	if sealed.EnclaveID != enclave.id {
		t.Fatal("sealed data enclave ID mismatch")
	}
	if len(sealed.Ciphertext) == 0 {
		t.Fatal("ciphertext should not be empty")
	}
	if len(sealed.Nonce) == 0 {
		t.Fatal("nonce should not be empty")
	}
	if len(sealed.Tag) == 0 {
		t.Fatal("tag should not be empty")
	}

	unsealed, err := enclave.Unseal(sealed)
	if err != nil {
		t.Fatalf("Unseal failed: %v", err)
	}
	if !bytes.Equal(unsealed, plaintext) {
		t.Fatalf("unsealed data mismatch: got %x, want %x", unsealed, plaintext)
	}
}

func TestUnsealWrongEnclave(t *testing.T) {
	e1, _ := NewEnclave([]byte("code1"))
	e2, _ := NewEnclave([]byte("code2"))

	sealed, err := e1.Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	if _, err := e2.Unseal(sealed); err == nil {
		t.Fatal("expected error when unsealing with different enclave")
	}
}

func TestAttestAndVerify(t *testing.T) {
	code := []byte("enclave code")
	enclave, err := NewEnclave(code)
	if err != nil {
		t.Fatalf("NewEnclave failed: %v", err)
	}

	data := []byte("attestation data")
	quote, err := enclave.Attest(data)
	if err != nil {
		t.Fatalf("Attest failed: %v", err)
	}

	if quote.EnclaveID != enclave.id {
		t.Fatal("quote enclave ID mismatch")
	}
	if quote.Measurement != enclave.measurement {
		t.Fatal("quote measurement mismatch")
	}
	if !bytes.Equal(quote.Data, data) {
		t.Fatal("quote data mismatch")
	}
	if len(quote.Signature) == 0 {
		t.Fatal("signature should not be empty")
	}
	if quote.Timestamp == 0 {
		t.Fatal("timestamp should not be zero")
	}

	pubKey := enclave.attestKey.PubKey()
	if !VerifyAttestation(quote, enclave.measurement, pubKey) {
		t.Fatal("VerifyAttestation should return true")
	}
}

func TestVerifyAttestationWrongMeasurement(t *testing.T) {
	enclave, _ := NewEnclave([]byte("code"))
	quote, _ := enclave.Attest([]byte("data"))

	wrongMeas := [32]byte{1, 2, 3}
	pubKey := enclave.attestKey.PubKey()
	if VerifyAttestation(quote, wrongMeas, pubKey) {
		t.Fatal("VerifyAttestation should return false for wrong measurement")
	}
}

func TestVerifyAttestationWrongKey(t *testing.T) {
	enclave, _ := NewEnclave([]byte("code"))
	e2, _ := NewEnclave([]byte("other"))
	quote, _ := enclave.Attest([]byte("data"))

	wrongPub := e2.attestKey.PubKey()
	if VerifyAttestation(quote, enclave.measurement, wrongPub) {
		t.Fatal("VerifyAttestation should return false for wrong key")
	}
}

func TestVerifyAttestationBadSignature(t *testing.T) {
	enclave, _ := NewEnclave([]byte("code"))
	quote, _ := enclave.Attest([]byte("data"))
	quote.Signature = []byte{0, 1, 2}

	pubKey := enclave.attestKey.PubKey()
	if VerifyAttestation(quote, enclave.measurement, pubKey) {
		t.Fatal("VerifyAttestation should return false for bad signature")
	}
}

func TestSealUnsealEmpty(t *testing.T) {
	enclave, _ := NewEnclave([]byte("test"))
	sealed, err := enclave.Seal([]byte{})
	if err != nil {
		t.Fatalf("Seal empty failed: %v", err)
	}

	unsealed, err := enclave.Unseal(sealed)
	if err != nil {
		t.Fatalf("Unseal empty failed: %v", err)
	}
	if len(unsealed) != 0 {
		t.Fatal("unsealed empty data should be empty")
	}
}

func TestSealUnsealLargeData(t *testing.T) {
	enclave, _ := NewEnclave([]byte("test"))
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	sealed, err := enclave.Seal(largeData)
	if err != nil {
		t.Fatalf("Seal large data failed: %v", err)
	}

	unsealed, err := enclave.Unseal(sealed)
	if err != nil {
		t.Fatalf("Unseal large data failed: %v", err)
	}
	if !bytes.Equal(unsealed, largeData) {
		t.Fatal("unsealed large data mismatch")
	}
}

func TestSealUnsealMultiple(t *testing.T) {
	enclave, _ := NewEnclave([]byte("test"))

	for i := 0; i < 10; i++ {
		data := []byte{byte(i), byte(i + 1), byte(i + 2)}
		sealed, err := enclave.Seal(data)
		if err != nil {
			t.Fatalf("Seal %d failed: %v", i, err)
		}
		unsealed, err := enclave.Unseal(sealed)
		if err != nil {
			t.Fatalf("Unseal %d failed: %v", i, err)
		}
		if !bytes.Equal(unsealed, data) {
			t.Fatalf("round-trip %d mismatch", i)
		}
	}
}

func TestEncryptedMemory(t *testing.T) {
	enclave, _ := NewEnclave([]byte("mem"))
	em := NewEncryptedMemory(enclave)

	err := em.Store("key1", []byte("value1"))
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	val, err := em.Load("key1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if string(val) != "value1" {
		t.Fatalf("Load returned wrong value: %s", val)
	}
}

func TestEncryptedMemoryMissingKey(t *testing.T) {
	enclave, _ := NewEnclave([]byte("mem"))
	em := NewEncryptedMemory(enclave)

	if _, err := em.Load("nonexistent"); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestEncryptedMemoryMultipleKeys(t *testing.T) {
	enclave, _ := NewEnclave([]byte("mem"))
	em := NewEncryptedMemory(enclave)

	keys := map[string]string{"a": "1", "b": "2", "c": "3"}
	for k, v := range keys {
		if err := em.Store(k, []byte(v)); err != nil {
			t.Fatalf("Store %s failed: %v", k, err)
		}
	}

	for k, v := range keys {
		val, err := em.Load(k)
		if err != nil {
			t.Fatalf("Load %s failed: %v", k, err)
		}
		if string(val) != v {
			t.Fatalf("Load %s: got %s, want %s", k, val, v)
		}
	}
}

func TestEnclaveConcurrency(t *testing.T) {
	enclave, _ := NewEnclave([]byte("concurrent"))
	done := make(chan bool, 20)

	for i := 0; i < 20; i++ {
		go func(n int) {
			data := []byte{byte(n)}
			sealed, err := enclave.Seal(data)
			if err != nil {
				t.Errorf("concurrent Seal %d failed: %v", n, err)
				done <- false
				return
			}
			unsealed, err := enclave.Unseal(sealed)
			if err != nil {
				t.Errorf("concurrent Unseal %d failed: %v", n, err)
				done <- false
				return
			}
			if !bytes.Equal(unsealed, data) {
				t.Errorf("concurrent round-trip %d mismatch", n)
				done <- false
				return
			}
			done <- true
		}(i)
	}

	for i := 0; i < 20; i++ {
		if !<-done {
			t.Fatal("concurrent test failed")
		}
	}
}
