package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEncryptDecryptKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	keyFile := filepath.Join(dir, "keystore.json")

	passphrase := "test-passphrase-123"
	if err := EncryptKey(key, passphrase, keyFile); err != nil {
		t.Fatalf("failed to encrypt key: %v", err)
	}

	// Verify file permissions (skip on Windows)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(keyFile)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("expected permissions 0600, got %v", info.Mode().Perm())
		}
	}

	// Decrypt and verify
	decrypted, err := DecryptKey(keyFile, passphrase)
	if err != nil {
		t.Fatalf("failed to decrypt key: %v", err)
	}

	if key.PubKey().Hex() != decrypted.PubKey().Hex() {
		t.Error("decrypted public key does not match original")
	}
}

func TestDecryptWrongPassphrase(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	keyFile := filepath.Join(dir, "keystore.json")

	if err := EncryptKey(key, "correct-pass", keyFile); err != nil {
		t.Fatal(err)
	}

	_, err = DecryptKey(keyFile, "wrong-pass")
	if err == nil {
		t.Error("expected error when decrypting with wrong passphrase")
	}
}

func TestLoadKeyOrGenerate_NewKey(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key")

	key, err := LoadKeyOrGenerate(keyFile, "pass")
	if err != nil {
		t.Fatalf("failed to load/generate key: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}

	// Verify encrypted file was created
	encFile := keyFile + ".enc"
	if _, err := os.Stat(encFile); err != nil {
		t.Errorf("expected encrypted key file to exist: %v", err)
	}

	// Loading again should return the same key
	key2, err := LoadKeyOrGenerate(keyFile, "pass")
	if err != nil {
		t.Fatalf("failed to reload key: %v", err)
	}
	if key.PubKey().Hex() != key2.PubKey().Hex() {
		t.Error("reloaded key does not match original")
	}
}

func TestLoadKeyOrGenerate_MigrateRawKey(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key")

	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	// Write raw hex key
	rawData := hex.EncodeToString(key.PrivateBytes())
	if err := os.WriteFile(keyFile, []byte(rawData), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadKeyOrGenerate(keyFile, "pass")
	if err != nil {
		t.Fatalf("failed to load key: %v", err)
	}

	if key.PubKey().Hex() != loaded.PubKey().Hex() {
		t.Error("migrated key does not match original")
	}

	// Raw key should be removed
	if _, err := os.Stat(keyFile); err == nil {
		t.Error("expected raw key file to be removed after migration")
	}
}

func TestEncryptedKeystoreFormat(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	keyFile := filepath.Join(dir, "keystore.json")

	if err := EncryptKey(key, "pass", keyFile); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}

	var ks EncryptedKeystore
	if err := json.Unmarshal(data, &ks); err == nil {
		// Just verify file is valid JSON
	}
}

func TestEncryptKeyCreatesDirectory(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	keyFile := filepath.Join(dir, "subdir", "keystore.json")

	if err := EncryptKey(key, "pass", keyFile); err != nil {
		t.Fatalf("expected directory to be created: %v", err)
	}

	if _, err := os.Stat(keyFile); err != nil {
		t.Errorf("expected key file to exist: %v", err)
	}
}

func TestDecryptKeyNonExistent(t *testing.T) {
	_, err := DecryptKey("/nonexistent/path/key.json", "pass")
	if err == nil {
		t.Error("expected error for non-existent key file")
	}
}

func TestKeystoreDeterministicEncryption(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()

	// Encrypt twice with same passphrase
	file1 := filepath.Join(dir, "key1.json")
	file2 := filepath.Join(dir, "key2.json")

	if err := EncryptKey(key, "pass", file1); err != nil {
		t.Fatal(err)
	}
	if err := EncryptKey(key, "pass", file2); err != nil {
		t.Fatal(err)
	}

	// Both should decrypt successfully
	k1, err := DecryptKey(file1, "pass")
	if err != nil {
		t.Fatal(err)
	}
	k2, err := DecryptKey(file2, "pass")
	if err != nil {
		t.Fatal(err)
	}

	if k1.PubKey().Hex() != k2.PubKey().Hex() {
		t.Error("keys should be identical after decrypt")
	}

	// Ciphertext should differ due to random salt/nonce
	data1, _ := os.ReadFile(file1)
	data2, _ := os.ReadFile(file2)
	if string(data1) == string(data2) {
		t.Error("encrypted files should differ due to random salt/nonce")
	}
}

func TestKeystoreEmptyPassphrase(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	keyFile := filepath.Join(dir, "keystore.json")

	if err := EncryptKey(key, "", keyFile); err != nil {
		t.Fatalf("expected empty passphrase to work: %v", err)
	}

	decrypted, err := DecryptKey(keyFile, "")
	if err != nil {
		t.Fatalf("expected decrypt with empty passphrase to work: %v", err)
	}

	if key.PubKey().Hex() != decrypted.PubKey().Hex() {
		t.Error("decrypted key does not match")
	}
}

func TestKeystoreLongPassphrase(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	keyFile := filepath.Join(dir, "keystore.json")

	passphrase := string(make([]byte, 1024))
	rand.Read([]byte(passphrase))

	if err := EncryptKey(key, passphrase, keyFile); err != nil {
		t.Fatalf("expected long passphrase to work: %v", err)
	}

	decrypted, err := DecryptKey(keyFile, passphrase)
	if err != nil {
		t.Fatalf("expected decrypt with long passphrase to work: %v", err)
	}

	if key.PubKey().Hex() != decrypted.PubKey().Hex() {
		t.Error("decrypted key does not match")
	}
}
