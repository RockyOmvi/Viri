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
	Scheme     string `json:"scheme"`
	Address    string `json:"address"`
	CipherText string `json:"cipher_text"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	ScryptN    int    `json:"scrypt_n"`
	ScryptR    int    `json:"scrypt_r"`
	ScryptP    int    `json:"scrypt_p"`
}

// EncryptKey encrypts a private key with a passphrase and saves it to a file.
func EncryptKey(key KeyPair, passphrase string, filePath string) error {
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

	plaintext := key.PrivateBytes()
	// Left-pad to 32 bytes for ECDSA keys
	var paddedKey []byte
	if key.Scheme() == SchemeECDSA {
		paddedKey = make([]byte, 32)
		copy(paddedKey[32-len(plaintext):], plaintext)
	} else {
		paddedKey = plaintext
	}

	ciphertext := aesGCM.Seal(nil, nonce, paddedKey, nil)

	keystore := &EncryptedKeystore{
		Version:    1,
		Scheme:     key.Scheme().String(),
		Address:    hex.EncodeToString(deriveAddress(key)),
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

	scheme, _ := ParseScheme(keystore.Scheme)
	switch scheme {
	case SchemeECDSA:
		return PrivateKeyFromBytes(plaintext)
	case SchemeMLDSA44, SchemeMLDSA65, SchemeMLDSA87, SchemeSPHINCS:
		return nil, fmt.Errorf("keystore does not support loading %s keys through this path; use NewSignerFromPrivateBytes", keystore.Scheme)
	default:
		return PrivateKeyFromBytes(plaintext)
	}
}

// DecryptKeyToSigner decrypts any scheme key from a keystore into a Signer.
func DecryptKeyToSigner(filePath string, passphrase string) (Signer, error) {
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

	scheme, _ := ParseScheme(keystore.Scheme)
	return NewSignerFromPrivateBytes(scheme, plaintext)
}

// deriveAddress extracts the canonical address from any KeyPair.

// deriveAddress extracts the canonical address from any KeyPair.
// For ECDSA it uses the Keccak256 address; for others it uses SHA256 of public key.
func deriveAddress(key KeyPair) []byte {
	switch key.Scheme() {
	case SchemeECDSA:
		if pk, ok := key.(*PrivateKey); ok {
			return pk.PubKey().Address()
		}
		fallthrough
	default:
		pub := Keccak256(key.PublicBytes())
		return pub[12:]
	}
}

// LoadKeyOrGenerate loads an existing encrypted key or generates a new one.
// For automated/dev environments, an empty passphrase uses a default.
// Returns an ECDSA key; for PQC schemes use LoadKeyOrGenerateScheme.
func LoadKeyOrGenerate(keyFile string, passphrase string) (*PrivateKey, error) {
	encKeyFile := keyFile + ".enc"
	if _, err := os.Stat(encKeyFile); err == nil {
		key, err := DecryptKey(encKeyFile, passphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt existing key: %w", err)
		}
		return key, nil
	}

	if data, err := os.ReadFile(keyFile); err == nil {
		keyBytes, err := hex.DecodeString(string(data))
		if err == nil {
			key, err := PrivateKeyFromBytes(keyBytes)
			if err == nil {
				if encErr := EncryptKey(key, passphrase, encKeyFile); encErr == nil {
					os.Remove(keyFile)
				}
				return key, nil
			}
		}
	}

	key, err := GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	if err := EncryptKey(key, passphrase, encKeyFile); err != nil {
		return nil, fmt.Errorf("failed to save encrypted key: %w", err)
	}

	return key, nil
}

// LoadKeyOrGenerateScheme loads or generates a key for the given scheme.
func LoadKeyOrGenerateScheme(keyFile string, passphrase string, scheme Scheme) (Signer, error) {
	encKeyFile := keyFile + ".enc"
	if _, err := os.Stat(encKeyFile); err == nil {
		signer, err := DecryptKeyToSigner(encKeyFile, passphrase)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt existing key: %w", err)
		}
		if signer.Scheme() != scheme {
			return nil, fmt.Errorf("key scheme mismatch: stored=%s requested=%s", signer.Scheme(), scheme)
		}
		return signer, nil
	}

	gen, ok := GetGenerator(scheme)
	if !ok {
		return nil, fmt.Errorf("unknown scheme: %s", scheme)
	}
	kp, err := gen.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate %s key: %w", scheme, err)
	}

	if err := EncryptKey(kp, passphrase, encKeyFile); err != nil {
		return nil, fmt.Errorf("failed to save encrypted key: %w", err)
	}

	return kp, nil
}
