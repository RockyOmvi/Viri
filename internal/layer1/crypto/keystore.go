package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/scrypt"
)

const (
	scryptN      = 1 << 18 // CPU/memory cost parameter
	scryptR      = 8
	scryptP      = 1
	scryptKeyLen = 32
	saltLen      = 32
	nonceLen     = 12
)

// EncryptedKeystore represents an encrypted private key file.
type EncryptedKeystore struct {
	Version    int    `json:"version"`
	Address    string `json:"address"`
	CipherText string `json:"cipher_text"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	ScryptN    int    `json:"scrypt_n"`
	ScryptR    int    `json:"scrypt_r"`
	ScryptP    int    `json:"scrypt_p"`
}

// EncryptKey encrypts a private key with a passphrase and saves it to a file.
func EncryptKey(key *PrivateKey, passphrase string, filePath string) error {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	derivedKey, err := scrypt.Key([]byte(passphrase), salt, scryptN, scryptR, scryptP, scryptKeyLen)
	if err != nil {
		return fmt.Errorf("failed to derive key: %w", err)
	}

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	plaintext := key.D.Bytes()
	// Left-pad to 32 bytes
	paddedKey := make([]byte, 32)
	copy(paddedKey[32-len(plaintext):], plaintext)

	ciphertext := aesGCM.Seal(nil, nonce, paddedKey, nil)

	keystore := &EncryptedKeystore{
		Version:    1,
		Address:    hex.EncodeToString(key.PubKey().Address()),
		CipherText: hex.EncodeToString(ciphertext),
		Salt:       hex.EncodeToString(salt),
		Nonce:      hex.EncodeToString(nonce),
		ScryptN:    scryptN,
		ScryptR:    scryptR,
		ScryptP:    scryptP,
	}

	data, err := json.MarshalIndent(keystore, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal keystore: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(filePath, data, 0600)
}

// DecryptKey decrypts a private key from an encrypted keystore file.
func DecryptKey(filePath string, passphrase string) (*PrivateKey, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read keystore: %w", err)
	}

	var keystore EncryptedKeystore
	if err := json.Unmarshal(data, &keystore); err != nil {
		return nil, fmt.Errorf("failed to parse keystore: %w", err)
	}

	salt, err := hex.DecodeString(keystore.Salt)
	if err != nil {
		return nil, fmt.Errorf("invalid salt: %w", err)
	}

	nonce, err := hex.DecodeString(keystore.Nonce)
	if err != nil {
		return nil, fmt.Errorf("invalid nonce: %w", err)
	}

	ciphertext, err := hex.DecodeString(keystore.CipherText)
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext: %w", err)
	}

	derivedKey, err := scrypt.Key([]byte(passphrase), salt, keystore.ScryptN, keystore.ScryptR, keystore.ScryptP, scryptKeyLen)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt key (wrong passphrase?): %w", err)
	}

	return PrivateKeyFromBytes(plaintext)
}

// LoadKeyOrGenerate loads an existing encrypted key or generates a new one.
// For automated/dev environments, an empty passphrase uses a default.
func LoadKeyOrGenerate(keyFile string, passphrase string) (*PrivateKey, error) {
	// Try loading existing encrypted keystore
	encKeyFile := keyFile + ".enc"
	if _, err := os.Stat(encKeyFile); err == nil {
		key, err := DecryptKey(encKeyFile, passphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt existing key: %w", err)
		}
		return key, nil
	}

	// Try loading legacy raw hex key and migrate it
	if data, err := os.ReadFile(keyFile); err == nil {
		keyBytes, err := hex.DecodeString(string(data))
		if err == nil {
			key, err := PrivateKeyFromBytes(keyBytes)
			if err == nil {
				// Migrate to encrypted format
				if encErr := EncryptKey(key, passphrase, encKeyFile); encErr == nil {
					os.Remove(keyFile) // Remove unencrypted key after migration
				}
				return key, nil
			}
		}
	}

	// Generate new key
	key, err := GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	if err := EncryptKey(key, passphrase, encKeyFile); err != nil {
		return nil, fmt.Errorf("failed to save encrypted key: %w", err)
	}

	return key, nil
}
