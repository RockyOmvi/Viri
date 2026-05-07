package integration

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

type GenesisValidatorEntry struct {
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
	Stake     uint64 `json:"stake"`
	Name      string `json:"name"`
}

type GenesisManifestEntry struct {
	ChainID      uint64                 `json:"chain_id"`
	NetworkName  string                 `json:"network_name"`
	GenesisTime  string                 `json:"genesis_time"`
	Validators   []GenesisValidatorEntry `json:"validators"`
	TotalStake   uint64                 `json:"total_stake"`
	Quorum       int                    `json:"quorum"`
	Complete     bool                   `json:"complete"`
}

type GenesisOutputFile struct {
	ChainID     uint64                 `json:"chain_id"`
	NetworkName string                 `json:"network_name"`
	GenesisTime string                 `json:"genesis_time"`
	Hash        string                 `json:"genesis_hash"`
	Validators  []GenesisValidatorEntry `json:"validators"`
	TotalStake  uint64                 `json:"total_stake"`
	Version     string                 `json:"version"`
}

type GenesisSignature struct {
	ValidatorAddress string `json:"validator_address"`
	PublicKey       string `json:"public_key"`
	Signature       string `json:"signature"`
	Timestamp       string `json:"timestamp"`
	ManifestHash    string `json:"manifest_hash"`
}

type GenesisCeremonyState struct {
	Phase             string                      `json:"phase"`
	RequiredSigners   []string                    `json:"required_signers"`
	CollectedSigs     map[string]GenesisSignature `json:"collected_signatures"`
	Threshold         int                         `json:"threshold,omitempty"`
	StakeWeighted     bool                        `json:"stake_weighted"`
}

type CeremonyAuditEvent struct {
	Timestamp string `json:"timestamp"`
	EventType string `json:"event_type"`
	Details   string `json:"details"`
	Signature string `json:"signature,omitempty"`
}

func TestInitCreatesCeremonyDirectory(t *testing.T) {
	dir, _ := os.MkdirTemp("", "genesis-test")
	defer os.RemoveAll(dir)

	ceremonyDir := filepath.Join(dir, ".viri", "genesis")
	if err := os.MkdirAll(ceremonyDir, 0700); err != nil {
		t.Fatal(err)
	}

	config := map[string]interface{}{
		"chain_id":       1,
		"network_name":   "viri-mainnet",
		"genesis_time":   time.Now().UTC().Format(time.RFC3339),
		"min_validators": 3,
	}

	configPath := filepath.Join(ceremonyDir, "ceremony.json")
	if err := saveTestJSON(configPath, config); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("ceremony.json was not created")
	}

	manifest := GenesisManifestEntry{
		ChainID:     1,
		NetworkName: "viri-mainnet",
		GenesisTime: time.Now().UTC().Format(time.RFC3339),
		Validators:  []GenesisValidatorEntry{},
		Complete:    false,
	}

	manifestPath := filepath.Join(ceremonyDir, "manifest.json")
	if err := saveTestJSON(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("manifest.json was not created")
	}
}

func TestAddValidatorUpdatesManifest(t *testing.T) {
	dir, _ := os.MkdirTemp("", "genesis-test")
	defer os.RemoveAll(dir)

	ceremonyDir := filepath.Join(dir, ".viri", "genesis")
	os.MkdirAll(ceremonyDir, 0700)

	manifest := GenesisManifestEntry{
		ChainID:     1,
		NetworkName: "viri-mainnet",
		Validators:  []GenesisValidatorEntry{},
		TotalStake:  0,
	}

	key, _ := crypto.GenerateKey()
	pubKeyHex := hex.EncodeToString(key.PubKey().Bytes())
	addr := "0x" + pubKeyHex[:40]

	validator := GenesisValidatorEntry{
		Address:   addr,
		PublicKey: pubKeyHex,
		Stake:     1000000,
		Name:      "validator1",
	}

	manifest.Validators = append(manifest.Validators, validator)
	manifest.TotalStake += validator.Stake

	manifestPath := filepath.Join(ceremonyDir, "manifest.json")
	if err := saveTestJSON(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}

	var loadedManifest GenesisManifestEntry
	if err := loadTestJSON(manifestPath, &loadedManifest); err != nil {
		t.Fatal(err)
	}

	if len(loadedManifest.Validators) != 1 {
		t.Fatalf("expected 1 validator, got %d", len(loadedManifest.Validators))
	}

	if loadedManifest.Validators[0].Name != "validator1" {
		t.Errorf("expected validator1, got %s", loadedManifest.Validators[0].Name)
	}

	if loadedManifest.TotalStake != 1000000 {
		t.Errorf("expected total stake 1000000, got %d", loadedManifest.TotalStake)
	}
}

func TestSignCreatesValidECDSASignature(t *testing.T) {
	key, _ := crypto.GenerateKey()
	pubKeyBytes := key.PubKey().Bytes()
	pubKeyHex := hex.EncodeToString(pubKeyBytes)

	manifest := GenesisManifestEntry{
		ChainID:     1,
		NetworkName: "viri-mainnet",
		Validators: []GenesisValidatorEntry{
			{Address: "0x" + pubKeyHex[:40], PublicKey: pubKeyHex, Stake: 1000000, Name: "test"},
		},
	}

	manifestHash := computeTestManifestHash(manifest)

	signature, err := key.Sign(manifestHash)
	if err != nil {
		t.Fatal(err)
	}

	if !key.PubKey().Verify(manifestHash, signature) {
		t.Error("signature verification failed")
	}

	genSig := GenesisSignature{
		ValidatorAddress: "0x" + pubKeyHex[:40],
		PublicKey:       pubKeyHex,
		Signature:       hex.EncodeToString(signature.Bytes()),
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		ManifestHash:    hex.EncodeToString(manifestHash),
	}

	sigData, err := json.Marshal(genSig)
	if err != nil {
		t.Fatal(err)
	}

	var loadedSig GenesisSignature
	if err := json.Unmarshal(sigData, &loadedSig); err != nil {
		t.Fatal(err)
	}

	sigBytes, err := hex.DecodeString(loadedSig.Signature)
	if err != nil {
		t.Fatal(err)
	}

	ecdsaSig, err := crypto.SignatureFromBytes(sigBytes)
	if err != nil {
		t.Fatal(err)
	}

	pubKey, err := crypto.PubKeyFromBytes(pubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	if !pubKey.Verify(manifestHash, ecdsaSig) {
		t.Error("loaded signature verification failed")
	}
}

func TestVerifyChecksSignaturesCorrectly(t *testing.T) {
	dir, _ := os.MkdirTemp("", "genesis-test")
	defer os.RemoveAll(dir)

	ceremonyDir := filepath.Join(dir, ".viri", "genesis")
	os.MkdirAll(ceremonyDir, 0700)

	key1, _ := crypto.GenerateKey()
	key2, _ := crypto.GenerateKey()

	pubKeyHex1 := hex.EncodeToString(key1.PubKey().Bytes())
	pubKeyHex2 := hex.EncodeToString(key2.PubKey().Bytes())

	manifest := GenesisManifestEntry{
		ChainID:     1,
		NetworkName: "viri-mainnet",
		Validators: []GenesisValidatorEntry{
			{Address: "0x" + pubKeyHex1[:40], PublicKey: pubKeyHex1, Stake: 1000000, Name: "validator1"},
			{Address: "0x" + pubKeyHex2[:40], PublicKey: pubKeyHex2, Stake: 2000000, Name: "validator2"},
		},
	}

	manifestPath := filepath.Join(ceremonyDir, "manifest.json")
	saveTestJSON(manifestPath, manifest)

	manifestHash := computeTestManifestHash(manifest)

	sig1, _ := key1.Sign(manifestHash)
	genSig1 := GenesisSignature{
		ValidatorAddress: "0x" + pubKeyHex1[:40],
		PublicKey:       pubKeyHex1,
		Signature:       hex.EncodeToString(sig1.Bytes()),
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		ManifestHash:    hex.EncodeToString(manifestHash),
	}

	sig1Path := filepath.Join(ceremonyDir, fmt.Sprintf("sig_0x%s.json", pubKeyHex1[:40]))
	saveTestJSON(sig1Path, genSig1)

	sigFiles, _ := filepath.Glob(filepath.Join(ceremonyDir, "sig_*.json"))
	if len(sigFiles) != 1 {
		t.Fatalf("expected 1 sig file, got %d", len(sigFiles))
	}

	validCount := 0
	for _, sigFile := range sigFiles {
		var sig GenesisSignature
		loadTestJSON(sigFile, &sig)

		sigBytes, err := hex.DecodeString(sig.Signature)
		if err != nil {
			continue
		}

		ecdsaSig, err := crypto.SignatureFromBytes(sigBytes)
		if err != nil {
			continue
		}

		pubKeyBytes, err := hex.DecodeString(sig.PublicKey)
		if err != nil {
			continue
		}

		pubKey, err := crypto.PubKeyFromBytes(pubKeyBytes)
		if err != nil {
			continue
		}

		manifestHashBytes, err := hex.DecodeString(sig.ManifestHash)
		if err != nil {
			continue
		}

		if pubKey.Verify(manifestHashBytes, ecdsaSig) {
			validCount++
		}
	}

	if validCount != 1 {
		t.Errorf("expected 1 valid signature, got %d", validCount)
	}
}

func TestFinalizeProducesGenesisJSON(t *testing.T) {
	dir, _ := os.MkdirTemp("", "genesis-test")
	defer os.RemoveAll(dir)

	ceremonyDir := filepath.Join(dir, ".viri", "genesis")
	os.MkdirAll(ceremonyDir, 0700)

	key1, _ := crypto.GenerateKey()
	key2, _ := crypto.GenerateKey()

	pubKeyHex1 := hex.EncodeToString(key1.PubKey().Bytes())
	pubKeyHex2 := hex.EncodeToString(key2.PubKey().Bytes())

	manifest := GenesisManifestEntry{
		ChainID:     1,
		NetworkName: "viri-mainnet",
		GenesisTime: time.Now().UTC().Format(time.RFC3339),
		Validators: []GenesisValidatorEntry{
			{Address: "0x" + pubKeyHex1[:40], PublicKey: pubKeyHex1, Stake: 1000000, Name: "validator1"},
			{Address: "0x" + pubKeyHex2[:40], PublicKey: pubKeyHex2, Stake: 2000000, Name: "validator2"},
		},
		TotalStake: 3000000,
		Quorum:     2,
		Complete:   true,
	}

	manifestPath := filepath.Join(ceremonyDir, "manifest.json")
	saveTestJSON(manifestPath, manifest)

	hashBytes := computeTestManifestHash(manifest)
	hash := hex.EncodeToString(hashBytes)

	genesis := GenesisOutputFile{
		ChainID:     manifest.ChainID,
		NetworkName: manifest.NetworkName,
		GenesisTime: manifest.GenesisTime,
		Hash:        hash,
		Validators:  manifest.Validators,
		TotalStake:  manifest.TotalStake,
		Version:     "0.1.0",
	}

	genesisPath := filepath.Join(ceremonyDir, "genesis.json")
	if err := saveTestJSON(genesisPath, genesis); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(genesisPath); os.IsNotExist(err) {
		t.Error("genesis.json was not created")
	}

	var loadedGenesis GenesisOutputFile
	if err := loadTestJSON(genesisPath, &loadedGenesis); err != nil {
		t.Fatal(err)
	}

	if loadedGenesis.Hash != hash {
		t.Errorf("expected hash %s, got %s", hash, loadedGenesis.Hash)
	}

	if len(loadedGenesis.Validators) != 2 {
		t.Errorf("expected 2 validators, got %d", len(loadedGenesis.Validators))
	}
}

func TestQuorumCalculation(t *testing.T) {
	tests := []struct {
		validators int
		expected  int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 3},
		{4, 3},
		{5, 4},
		{6, 5},
		{7, 5},
		{8, 6},
		{12, 9},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("validators=%d", tt.validators), func(t *testing.T) {
			manifest := GenesisManifestEntry{
				Validators: make([]GenesisValidatorEntry, tt.validators),
			}
			quorum := calculateTestQuorum(manifest)
			if quorum != tt.expected {
				t.Errorf("expected quorum %d for %d validators, got %d", tt.expected, tt.validators, quorum)
			}
		})
	}
}

func TestStakeWeightedQuorum(t *testing.T) {
	tests := []struct {
		totalStake uint64
		expected  uint64
	}{
		{0, 1},
		{100, 67},
		{1000, 667},
		{3000000, 2000001},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("stake=%d", tt.totalStake), func(t *testing.T) {
			manifest := GenesisManifestEntry{
				TotalStake: tt.totalStake,
			}
			quorum := calculateTestStakeWeightedQuorum(manifest)
			if quorum != tt.expected {
				t.Errorf("expected stake quorum %d for total stake %d, got %d", tt.expected, tt.totalStake, quorum)
			}
		})
	}
}

func TestCeremonyAuditTrail(t *testing.T) {
	dir, _ := os.MkdirTemp("", "genesis-test")
	defer os.RemoveAll(dir)

	ceremonyDir := filepath.Join(dir, ".viri", "genesis")
	os.MkdirAll(ceremonyDir, 0700)

	logFile := filepath.Join(ceremonyDir, "ceremony_log.json")

	events := []CeremonyAuditEvent{
		{Timestamp: time.Now().UTC().Format(time.RFC3339), EventType: "ceremony initialized", Details: "chain_id=1"},
		{Timestamp: time.Now().UTC().Format(time.RFC3339), EventType: "validator registered", Details: "name=val1"},
		{Timestamp: time.Now().UTC().Format(time.RFC3339), EventType: "validator signed", Details: "address=0x123"},
	}

	if err := saveTestJSON(logFile, events); err != nil {
		t.Fatal(err)
	}

	var loadedEvents []CeremonyAuditEvent
	if err := loadTestJSON(logFile, &loadedEvents); err != nil {
		t.Fatal(err)
	}

	if len(loadedEvents) != 3 {
		t.Fatalf("expected 3 audit events, got %d", len(loadedEvents))
	}

	expectedTypes := []string{"ceremony initialized", "validator registered", "validator signed"}
	for i, event := range loadedEvents {
		if event.EventType != expectedTypes[i] {
			t.Errorf("expected event type %s, got %s", expectedTypes[i], event.EventType)
		}
	}
}

func TestStateTransitions(t *testing.T) {
	dir, _ := os.MkdirTemp("", "genesis-test")
	defer os.RemoveAll(dir)

	ceremonyDir := filepath.Join(dir, ".viri", "genesis")
	os.MkdirAll(ceremonyDir, 0700)

	state := GenesisCeremonyState{
		Phase:           "init",
		RequiredSigners:  []string{},
		CollectedSigs:    make(map[string]GenesisSignature),
		StakeWeighted:    true,
	}

	statePath := filepath.Join(ceremonyDir, "state.json")
	saveTestJSON(statePath, state)

	var loadedState GenesisCeremonyState
	loadTestJSON(statePath, &loadedState)

	if loadedState.Phase != "init" {
		t.Errorf("expected phase init, got %s", loadedState.Phase)
	}

	loadedState.Phase = "registration"
	saveTestJSON(statePath, loadedState)

	loadTestJSON(statePath, &loadedState)
	if loadedState.Phase != "registration" {
		t.Errorf("expected phase registration, got %s", loadedState.Phase)
	}

	loadedState.Phase = "signing"
	saveTestJSON(statePath, loadedState)

	loadTestJSON(statePath, &loadedState)
	if loadedState.Phase != "signing" {
		t.Errorf("expected phase signing, got %s", loadedState.Phase)
	}

	loadedState.Phase = "complete"
	saveTestJSON(statePath, loadedState)

	loadTestJSON(statePath, &loadedState)
	if loadedState.Phase != "complete" {
		t.Errorf("expected phase complete, got %s", loadedState.Phase)
	}
}

func computeTestManifestHash(manifest GenesisManifestEntry) []byte {
	data, _ := json.Marshal(manifest.Validators)
	h := crypto.DoubleSHA256(data)
	return h
}

func calculateTestQuorum(manifest GenesisManifestEntry) int {
	return int(math.Floor(float64(len(manifest.Validators))*2.0/3.0)) + 1
}

func calculateTestStakeWeightedQuorum(manifest GenesisManifestEntry) uint64 {
	return manifest.TotalStake*2/3 + 1
}

func saveTestJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadTestJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
