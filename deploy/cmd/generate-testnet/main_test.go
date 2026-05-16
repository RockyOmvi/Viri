package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	privHex, pubHex, address := generateKey()
	if len(privHex) != 64 {
		t.Fatalf("expected 64 hex chars for private key, got %d", len(privHex))
	}
	if len(pubHex) != 130 {
		t.Fatalf("expected 130 hex chars for public key (65 bytes), got %d", len(pubHex))
	}
	if len(address) != 40 {
		t.Fatalf("expected 40 hex chars for address (20 bytes), got %d", len(address))
	}
}

func TestWriteJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	data := map[string]string{"hello": "world"}
	if err := writeJSON(path, data); err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if !strings.Contains(string(content), "world") {
		t.Fatal("written JSON missing expected content")
	}
}

func TestGenerateComposeYAML(t *testing.T) {
	yaml := generateComposeYAML(4, 1337, false)

	if !strings.Contains(yaml, "validator-0") {
		t.Fatal("missing validator-0 service")
	}
	if !strings.Contains(yaml, "validator-3") {
		t.Fatal("missing validator-3 service")
	}
	if strings.Contains(yaml, "validator-4") {
		t.Fatal("unexpected validator-4 service")
	}
	if !strings.Contains(yaml, "8545") {
		t.Fatal("missing RPC port")
	}
	if !strings.Contains(yaml, "30303") {
		t.Fatal("missing P2P port")
	}
	if !strings.Contains(yaml, "VIRI_CHAIN_ID=1337") {
		t.Fatal("missing chain ID environment variable")
	}
	if strings.Contains(yaml, "prometheus") {
		t.Fatal("unexpected monitoring service when monitoring=false")
	}
}

func TestGenerateComposeYAMLWithMonitoring(t *testing.T) {
	yaml := generateComposeYAML(2, 42, true)

	if !strings.Contains(yaml, "prometheus") {
		t.Fatal("missing prometheus when monitoring=true")
	}
	if !strings.Contains(yaml, "grafana") {
		t.Fatal("missing grafana when monitoring=true")
	}
	if !strings.Contains(yaml, "VIRI_CHAIN_ID=42") {
		t.Fatal("missing correct chain ID")
	}
}

func TestGenerateComposeYAMLVolumes(t *testing.T) {
	yaml := generateComposeYAML(4, 1337, false)

	for i := 0; i < 4; i++ {
		if !strings.Contains(yaml, fmt.Sprintf("validator-%d-data", i)) {
			t.Fatalf("missing volume for validator-%d", i)
		}
	}

	yamlWithMon := generateComposeYAML(4, 1337, true)
	if !strings.Contains(yamlWithMon, "prometheus-data") {
		t.Fatal("missing prometheus volume when monitoring=true")
	}
	if !strings.Contains(yamlWithMon, "grafana-data") {
		t.Fatal("missing grafana volume when monitoring=true")
	}
}

func TestMainWithTempDir(t *testing.T) {
	dir := t.TempDir()
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{
		"generate-testnet",
		"--validators", "2",
		"--chain-id", "99",
		"--stake", "500000",
		"--output-dir", dir,
	}

	main()

	// Verify output structure
	checkFile := func(path string) {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Fatalf("missing expected file: %s: %v", path, err)
		}
	}

	checkFile("genesis/genesis.json")
	checkFile("docker-compose.yml")
	checkFile("keys/validator-0.key")
	checkFile("keys/validator-1.key")
	checkFile("configs/validator-0/config.json")
	checkFile("configs/validator-0/validator.key")
	checkFile("configs/validator-1/config.json")
	checkFile("configs/validator-1/validator.key")

	keyFile := filepath.Join(dir, "keys", "validator-0.key")
	content, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatalf("failed to read key file: %v", err)
	}
	if len(strings.TrimSpace(string(content))) != 64 {
		t.Fatal("expected 64-char hex key")
	}
}

func TestMustJSON(t *testing.T) {
	data := mustJSON(map[string]interface{}{"key": "value"})
	if !strings.Contains(string(data), "value") {
		t.Fatal("mustJSON output missing expected value")
	}
}

func TestYamlStr(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with: colon", "\"with: colon\""},
		{"with#hash", "\"with#hash\""},
		{"", "\"\""},
		{"already-safe", "already-safe"},
	}
	for _, tc := range tests {
		result := yamlStr(tc.input)
		if result != tc.expected {
			t.Errorf("yamlStr(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestYamlList(t *testing.T) {
	result := yamlList("  ", []string{"a", "b"})
	expected := "  - \"a\"\n  - \"b\"\n"
	if result != expected {
		t.Fatalf("yamlList = %q, want %q", result, expected)
	}
}

func TestYamlMap(t *testing.T) {
	m := map[string]string{"b": "2", "a": "1"}
	result := yamlMap("  ", m)
	expected := "  a: 1\n  b: 2\n"
	if result != expected {
		t.Fatalf("yamlMap = %q, want %q", result, expected)
	}
}
