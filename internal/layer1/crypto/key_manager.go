package crypto

import (
	"encoding/hex"
	"fmt"
	"os"
)

// KeyManager provides a unified interface for signing operations
// with pluggable backends (file-based, HSM, cloud KMS).
type KeyManager interface {
	SignMessage(data []byte) ([]byte, error)
	Public() *PublicKey
	Address() []byte
	Scheme() Scheme
	Close() error
}

// fileKeyManager wraps a PrivateKey loaded from an encrypted keystore or env var.
type fileKeyManager struct {
	key *PrivateKey
}

func (m *fileKeyManager) SignMessage(data []byte) ([]byte, error) {
	return m.key.SignMessage(data)
}

func (m *fileKeyManager) Public() *PublicKey {
	return m.key.PubKey()
}

func (m *fileKeyManager) Address() []byte {
	return m.key.PubKey().Address()
}

func (m *fileKeyManager) Scheme() Scheme {
	return m.key.Scheme()
}

func (m *fileKeyManager) Close() error {
	return nil
}

// KMSConfig holds configuration for an external KMS backend.
type KMSConfig struct {
	Provider string // "aws", "azure", "gcp", "hashicorp"
	KeyID    string
	Region   string
	Endpoint string
	Token    string
}

// kmsKeyManager is a stub for external KMS integration.
// In production, this would use the cloud provider's SDK.
type kmsKeyManager struct {
	config KMSConfig
	pub    *PublicKey
	addr   []byte
}

func (m *kmsKeyManager) SignMessage(data []byte) ([]byte, error) {
	return nil, fmt.Errorf("KMS provider %q not yet implemented; configure a file-based key instead", m.config.Provider)
}

func (m *kmsKeyManager) Public() *PublicKey {
	return m.pub
}

func (m *kmsKeyManager) Address() []byte {
	return m.addr
}

func (m *kmsKeyManager) Scheme() Scheme {
	return SchemeECDSA
}

func (m *kmsKeyManager) Close() error {
	return nil
}

// NewKeyManagerFromKey wraps an existing PrivateKey as a KeyManager.
func NewKeyManagerFromKey(key *PrivateKey) KeyManager {
	return &fileKeyManager{key: key}
}

// NewKeyManagerFromKeystore loads a PrivateKey from an encrypted keystore file.
func NewKeyManagerFromKeystore(keyFile, passphrase string) (KeyManager, error) {
	key, err := LoadKeyOrGenerate(keyFile, passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to load key from keystore: %w", err)
	}
	return &fileKeyManager{key: key}, nil
}

// NewKeyManagerFromHex creates a KeyManager from a hex-encoded private key string.
func NewKeyManagerFromHex(hexKey string) (KeyManager, error) {
	if len(hexKey) == 0 {
		return nil, fmt.Errorf("empty hex key")
	}
	raw := hexKey
	if raw[:2] == "0x" {
		raw = raw[2:]
	}
	data, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid hex key: %w", err)
	}
	key, err := PrivateKeyFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("invalid key bytes: %w", err)
	}
	return &fileKeyManager{key: key}, nil
}

// NewKeyManagerFromFile loads a KeyManager from a raw hex key file (plaintext or encrypted).
func NewKeyManagerFromFile(path string) (KeyManager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}
	return NewKeyManagerFromHex(string(data))
}

// NewKMSKeyManager creates a stub KMS-backed KeyManager.
func NewKMSKeyManager(config KMSConfig) (KeyManager, error) {
	return &kmsKeyManager{config: config}, nil
}
