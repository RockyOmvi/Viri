package tee

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

type EnclaveID [32]byte

type SealedData struct {
	EnclaveID  EnclaveID
	Ciphertext []byte
	Nonce      []byte
	Tag        []byte
}

type AttestationQuote struct {
	EnclaveID   EnclaveID
	Measurement [32]byte
	Data        []byte
	Signature   []byte
	Timestamp   int64
}

type Enclave struct {
	mu          sync.Mutex
	id          EnclaveID
	codeHash    [32]byte
	sealedKey   []byte
	attestKey   *crypto.PrivateKey
	measurement [32]byte
	initialized bool
}

func NewEnclave(code []byte) (*Enclave, error) {
	id := EnclaveID(sha256.Sum256(append(code, time.Now().AppendFormat(nil, time.RFC3339Nano)...)))
	codeHash := sha256.Sum256(code)

	attestKey, err := crypto.GenerateKey()
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

func (e *Enclave) ID() EnclaveID { return e.id }

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

func (e *Enclave) Attest(data []byte) (*AttestationQuote, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now().Unix()
	payload := append(e.measurement[:], data...)
	payload = append(payload, e.id[:]...)
	payload = binary.LittleEndian.AppendUint64(payload, uint64(now))

	hash := sha256.Sum256(payload)
	sig, err := e.attestKey.SignHash(hash[:])
	if err != nil {
		return nil, err
	}

	return &AttestationQuote{
		EnclaveID:   e.id,
		Measurement: e.measurement,
		Data:        data,
		Signature:   sig.Bytes(),
		Timestamp:   now,
	}, nil
}

func VerifyAttestation(quote *AttestationQuote, expectedMeasurement [32]byte, pubKey *crypto.PublicKey) bool {
	if quote.Measurement != expectedMeasurement {
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
	sig := &crypto.Signature{R: r, S: s}

	return pubKey.VerifyHash(hash[:], sig)
}

type EncryptedMemory struct {
	mu      sync.Mutex
	data    map[string]*SealedData
	enclave *Enclave
}

func NewEncryptedMemory(enclave *Enclave) *EncryptedMemory {
	return &EncryptedMemory{
		data:    make(map[string]*SealedData),
		enclave: enclave,
	}
}

func (em *EncryptedMemory) Store(key string, value []byte) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	sealed, err := em.enclave.Seal(value)
	if err != nil {
		return err
	}

	em.data[key] = sealed
	return nil
}

func (em *EncryptedMemory) Load(key string) ([]byte, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	sealed, ok := em.data[key]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}

	return em.enclave.Unseal(sealed)
}
