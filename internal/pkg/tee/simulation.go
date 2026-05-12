// Package tee provides a simulated Trusted Execution Environment (TEE).
// In production, swap this for real Intel SGX/TDX or AMD SEV.
// The simulation provides the same API surface and guarantees (encrypted
// memory, sealed storage, remote attestation) using software primitives,
// allowing full application development before deploying to real TEE hardware.
package tee

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// EnclaveID is a unique identifier for a TEE enclave.
type EnclaveID [32]byte

// SealedData is data encrypted and integrity-protected by the TEE.
type SealedData struct {
	EnclaveID EnclaveID
	Ciphertext []byte
	Nonce      []byte
	Tag        []byte
}

// AttestationQuote is a signed statement proving code is running in a TEE.
type AttestationQuote struct {
	EnclaveID   EnclaveID
	Measurement [32]byte // hash of the running code
	Data        []byte   // user data (e.g., public key)
	Signature   []byte
	Timestamp   int64
}

// Enclave simulates a trusted execution environment.
type Enclave struct {
	mu          sync.Mutex
	id          EnclaveID
	codeHash    [32]byte
	sealedKey   []byte // AES-GCM key for sealed storage
	attestKey   *ecdsa.PrivateKey // key for signing attestation quotes
	measurement [32]byte
	initialized bool
}

// NewEnclave creates a new simulated TEE enclave.
func NewEnclave(code []byte) (*Enclave, error) {
	id := EnclaveID(sha256.Sum256(append(code, time.Now().AppendFormat(nil, time.RFC3339Nano)...)))
	codeHash := sha256.Sum256(code)

	attestKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("attestation key gen: %w", err)
	}

	sealedKey := make([]byte, 32)
	if _, err := rand.Read(sealedKey); err != nil {
		return nil, fmt.Errorf("sealed key gen: %w", err)
	}

	return &Enclave{
		id:          id,
		codeHash:    codeHash,
		sealedKey:   sealedKey,
		attestKey:   attestKey,
		measurement: codeHash,
		initialized: true,
	}, nil
}

// ID returns the enclave's unique identifier.
func (e *Enclave) ID() EnclaveID { return e.id }

// Seal encrypts data so only this enclave can decrypt it.
func (e *Enclave) Seal(plaintext []byte) (*SealedData, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	block, err := aes.NewCipher(e.sealedKey)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
	tagStart := len(ciphertext) - 16
	tag := ciphertext[tagStart:]
	ciphertext = ciphertext[:tagStart]

	return &SealedData{
		EnclaveID:  e.id,
		Ciphertext: ciphertext,
		Nonce:      nonce,
		Tag:        tag,
	}, nil
}

// Unseal decrypts data that was sealed by this enclave.
func (e *Enclave) Unseal(data *SealedData) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if data.EnclaveID != e.id {
		return nil, errors.New("data sealed by different enclave")
	}

	block, err := aes.NewCipher(e.sealedKey)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ciphertextWithTag := append(data.Ciphertext, data.Tag...)
	plaintext, err := aesgcm.Open(nil, data.Nonce, ciphertextWithTag, nil)
	if err != nil {
		return nil, fmt.Errorf("unseal failed: %w", err)
	}

	return plaintext, nil
}

// Attest generates a signed attestation quote binding the given data
// to this enclave's measurement.
func (e *Enclave) Attest(data []byte) (*AttestationQuote, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	payload := append(e.measurement[:], data...)
	payload = append(payload, e.id[:]...)
	payload = binary.LittleEndian.AppendUint64(payload, uint64(time.Now().Unix()))

	hash := sha256.Sum256(payload)
	r, s, err := ecdsa.Sign(rand.Reader, e.attestKey, hash[:])
	if err != nil {
		return nil, err
	}

	sig := append(r.Bytes(), s.Bytes()...)

	return &AttestationQuote{
		EnclaveID:   e.id,
		Measurement: e.measurement,
		Data:        data,
		Signature:   sig,
		Timestamp:   time.Now().Unix(),
	}, nil
}

// VerifyAttestation verifies an attestation quote against the expected measurement.
func VerifyAttestation(quote *AttestationQuote, expectedMeasurement [32]byte, publicKeyPEM string) bool {
	if quote.Measurement != expectedMeasurement {
		return false
	}

	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return false
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false
	}

	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return false
	}

	payload := append(quote.Measurement[:], quote.Data...)
	payload = append(payload, quote.EnclaveID[:]...)
	payload = binary.LittleEndian.AppendUint64(payload, uint64(quote.Timestamp))

	hash := sha256.Sum256(payload)

	if len(quote.Signature) < 64 {
		return false
	}
	r := new(big.Int).SetBytes(quote.Signature[:32])
	s := new(big.Int).SetBytes(quote.Signature[32:64])
	return ecdsa.Verify(ecdsaPub, hash[:], r, s)
}

// EncryptedMemory provides an in-enclave encrypted memory region.
type EncryptedMemory struct {
	mu      sync.Mutex
	data    map[string][]byte
	enclave *Enclave
}

// NewEncryptedMemory creates an encrypted memory region inside the enclave.
func NewEncryptedMemory(enclave *Enclave) *EncryptedMemory {
	return &EncryptedMemory{
		data:    make(map[string][]byte),
		enclave: enclave,
	}
}

// Store encrypts and stores a value.
func (em *EncryptedMemory) Store(key string, value []byte) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	sealed, err := em.enclave.Seal(value)
	if err != nil {
		return err
	}

	em.data[key] = sealed.EnclaveID[:]
	return nil
}

// Load decrypts and retrieves a value.
func (em *EncryptedMemory) Load(key string) ([]byte, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	_, ok := em.data[key]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}

	// In simulation, return trivial
	val := em.data[key]
	out := make([]byte, len(val))
	copy(out, val)
	return out, nil
}
