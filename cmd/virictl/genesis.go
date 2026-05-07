package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

type SigningPayload struct {
	ManifestHash string                 `json:"manifest_hash"`
	Validators   []GenesisValidatorEntry `json:"validators"`
	GenesisTime  string                 `json:"genesis_time"`
	ChainID      uint64                 `json:"chain_id"`
	NetworkName  string                 `json:"network_name"`
}

type GenesisCeremonyState struct {
	Phase             string                      `json:"phase"`
	RequiredSigners   []string                    `json:"required_signers"`
	CollectedSigs     map[string]GenesisSignature `json:"collected_signatures"`
	Threshold         int                         `json:"threshold,omitempty"`
	StakeWeighted     bool                        `json:"stake_weighted"`
}

type CeremonyAuditEvent struct {
	Timestamp    string `json:"timestamp"`
	EventType    string `json:"event_type"`
	Details      string `json:"details"`
	Signature    string `json:"signature,omitempty"`
}

func genesisInit() {
	dir := getCeremonyDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create ceremony directory: %v\n", err)
		os.Exit(1)
	}

	config := map[string]interface{}{
		"chain_id":       1,
		"network_name":   "viri-mainnet",
		"genesis_time":   time.Now().UTC().Format(time.RFC3339),
		"min_validators": 3,
		"initial_supply": 1000000000,
		"block_time":     "1s",
	}

	if err := saveJSON(filepath.Join(dir, "ceremony.json"), config); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		os.Exit(1)
	}

	manifest := GenesisManifestEntry{
		ChainID:     1,
		NetworkName: "viri-mainnet",
		GenesisTime: config["genesis_time"].(string),
		Complete:    false,
	}

	if err := saveJSON(filepath.Join(dir, "manifest.json"), manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save manifest: %v\n", err)
		os.Exit(1)
	}

	state := GenesisCeremonyState{
		Phase:         "init",
		RequiredSigners: []string{},
		CollectedSigs: make(map[string]GenesisSignature),
		StakeWeighted: true,
	}

	if err := saveJSON(filepath.Join(dir, "state.json"), state); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
		os.Exit(1)
	}

	logAuditEvent("ceremony initialized", fmt.Sprintf("chain_id=%d network=%s", manifest.ChainID, manifest.NetworkName), "")

	fmt.Println("Genesis ceremony initialized!")
	fmt.Printf("Directory: %s\n", dir)
	fmt.Printf("Chain ID: %d\n", manifest.ChainID)
	fmt.Printf("Network: %s\n", manifest.NetworkName)
	fmt.Printf("Minimum validators: %d\n", config["min_validators"])
}

func genesisAddValidator() {
	dir := getCeremonyDir()
	var manifest GenesisManifestEntry
	if err := loadJSON(filepath.Join(dir, "manifest.json"), &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load manifest: %v\n", err)
		os.Exit(1)
	}

	var name, pubKey string
	var stake uint64

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--name":
			if i+1 < len(os.Args) {
				name = os.Args[i+1]
				i++
			}
		case "--pubkey":
			if i+1 < len(os.Args) {
				pubKey = os.Args[i+1]
				i++
			}
		case "--stake":
			if i+1 < len(os.Args) {
				fmt.Sscanf(os.Args[i+1], "%d", &stake)
				i++
			}
		}
	}

	if name == "" || pubKey == "" {
		fmt.Println("Usage: virictl genesis add-validator --name <name> --pubkey <hex> --stake <amount>")
		os.Exit(1)
	}

	addr := "0x" + pubKey
	if len(pubKey) > 40 {
		addr = "0x" + pubKey[:40]
	}

	validator := GenesisValidatorEntry{
		Address:   addr,
		PublicKey: pubKey,
		Stake:     stake,
		Name:      name,
	}

	manifest.Validators = append(manifest.Validators, validator)
	manifest.TotalStake += stake

	if err := saveJSON(filepath.Join(dir, "manifest.json"), manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save manifest: %v\n", err)
		os.Exit(1)
	}

	var state GenesisCeremonyState
	if err := loadJSON(filepath.Join(dir, "state.json"), &state); err == nil {
		state.RequiredSigners = append(state.RequiredSigners, addr)
		if state.Phase == "init" {
			state.Phase = "registration"
		}
		saveJSON(filepath.Join(dir, "state.json"), state)
	}

	logAuditEvent("validator registered", fmt.Sprintf("name=%s address=%s stake=%d", name, addr, stake), "")

	fmt.Printf("Validator added: %s (%s)\n", name, addr)
	fmt.Printf("Total validators: %d\n", len(manifest.Validators))
	fmt.Printf("Total stake: %d\n", manifest.TotalStake)
}

func genesisSign() {
	dir := getCeremonyDir()
	var manifest GenesisManifestEntry
	if err := loadJSON(filepath.Join(dir, "manifest.json"), &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load manifest: %v\n", err)
		os.Exit(1)
	}

	var keyPath, passphrase, address string

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--key":
			if i+1 < len(os.Args) {
				keyPath = os.Args[i+1]
				i++
			}
		case "--passphrase":
			if i+1 < len(os.Args) {
				passphrase = os.Args[i+1]
				i++
			}
		case "--address":
			if i+1 < len(os.Args) {
				address = os.Args[i+1]
				i++
			}
		}
	}

	if keyPath == "" {
		fmt.Println("Usage: virictl genesis sign --key <keystore_path> [--passphrase <pass>] [--address <addr>]")
		os.Exit(1)
	}

	if passphrase == "" {
		fmt.Print("Enter passphrase: ")
		var err error
		passphrase, err = readPassword()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read passphrase: %v\n", err)
			os.Exit(1)
		}
	}

	privKey, err := crypto.DecryptKey(keyPath, passphrase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to decrypt key: %v\n", err)
		os.Exit(1)
	}

	manifestHash := computeManifestHash(manifest)

	signature, err := privKey.Sign(manifestHash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to sign: %v\n", err)
		os.Exit(1)
	}

	pubKey := privKey.PubKey()
	pubKeyHex := pubKey.Hex()
	addr := "0x" + pubKeyHex
	if len(pubKeyHex) > 40 {
		addr = "0x" + pubKeyHex[:40]
	}

	if address != "" && addr != address {
		fmt.Fprintf(os.Stderr, "Key address %s does not match expected address %s\n", addr, address)
		os.Exit(1)
	}

	genSig := GenesisSignature{
		ValidatorAddress: addr,
		PublicKey:       pubKeyHex,
		Signature:       hex.EncodeToString(signature.Bytes()),
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		ManifestHash:    hex.EncodeToString(manifestHash),
	}

	sigFile := filepath.Join(dir, fmt.Sprintf("sig_%s.json", addr))
	if err := saveJSON(sigFile, genSig); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save signature: %v\n", err)
		os.Exit(1)
	}

	var state GenesisCeremonyState
	if err := loadJSON(filepath.Join(dir, "state.json"), &state); err == nil {
		if state.Phase == "registration" {
			state.Phase = "signing"
		}
		state.CollectedSigs[addr] = genSig
		saveJSON(filepath.Join(dir, "state.json"), state)
	}

	logAuditEvent("validator signed", fmt.Sprintf("address=%s", addr), genSig.Signature)

	quorum := calculateQuorum(manifest)
	sigFiles, _ := filepath.Glob(filepath.Join(dir, "sig_*.json"))
	if len(sigFiles) >= quorum {
		logAuditEvent("quorum reached", fmt.Sprintf("signatures=%d required=%d", len(sigFiles), quorum), "")
	}

	fmt.Printf("Genesis signed successfully!\n")
	fmt.Printf("Validator: %s\n", addr)
	fmt.Printf("Signature: %s...\n", genSig.Signature[:32])
	fmt.Printf("Saved to: %s\n", sigFile)
}

func genesisVerify() {
	dir := getCeremonyDir()
	var manifest GenesisManifestEntry
	if err := loadJSON(filepath.Join(dir, "manifest.json"), &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load manifest: %v\n", err)
		os.Exit(1)
	}

	sigFiles, err := filepath.Glob(filepath.Join(dir, "sig_*.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list signatures: %v\n", err)
		os.Exit(1)
	}

	manifestHash := computeManifestHash(manifest)
	quorum := calculateQuorum(manifest)
	stakeQuorum := calculateStakeWeightedQuorum(manifest)

	fmt.Printf("Validators: %d\n", len(manifest.Validators))
	fmt.Printf("Total stake: %d\n", manifest.TotalStake)
	fmt.Printf("Quorum required (validators): %d\n", quorum)
	fmt.Printf("Quorum required (stake): %d\n", stakeQuorum)
	fmt.Printf("Signatures collected: %d\n\n", len(sigFiles))

	signedAddrs := make(map[string]bool)
	validSigs := 0
	var signedStake uint64

	for _, sigFile := range sigFiles {
		var sig GenesisSignature
		if err := loadJSON(sigFile, &sig); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load %s: %v\n", sigFile, err)
			continue
		}

		sigAddr := sig.ValidatorAddress

		isKnownValidator := false
		for _, v := range manifest.Validators {
			if v.Address == sigAddr {
				isKnownValidator = true
				break
			}
		}

		if !isKnownValidator {
			fmt.Printf("  [UNKNOWN] %s - not a registered validator\n", sigAddr)
			continue
		}

		if sig.ManifestHash != hex.EncodeToString(manifestHash) {
			fmt.Printf("  [INVALID] %s - manifest hash mismatch\n", sigAddr)
			continue
		}

		pubKeyBytes, err := hex.DecodeString(sig.PublicKey)
		if err != nil {
			fmt.Printf("  [INVALID] %s - invalid public key\n", sigAddr)
			continue
		}

		pubKey, err := crypto.PubKeyFromBytes(pubKeyBytes)
		if err != nil {
			fmt.Printf("  [INVALID] %s - invalid public key\n", sigAddr)
			continue
		}

		sigBytes, err := hex.DecodeString(sig.Signature)
		if err != nil {
			fmt.Printf("  [INVALID] %s - invalid signature\n", sigAddr)
			continue
		}

		ecdsaSig, err := crypto.SignatureFromBytes(sigBytes)
		if err != nil {
			fmt.Printf("  [INVALID] %s - invalid signature\n", sigAddr)
			continue
		}

		if !pubKey.Verify(manifestHash, ecdsaSig) {
			fmt.Printf("  [INVALID] %s - signature verification failed\n", sigAddr)
			continue
		}

		fmt.Printf("  [VALID]   %s - signed at %s\n", sigAddr, sig.Timestamp)
		signedAddrs[sigAddr] = true
		validSigs++
		for _, v := range manifest.Validators {
			if v.Address == sigAddr {
				signedStake += v.Stake
				break
			}
		}
	}

	fmt.Printf("\nValidation summary:\n")
	fmt.Printf("  Valid signatures: %d\n", validSigs)
	fmt.Printf("  Signed stake: %d / %d\n", signedStake, manifest.TotalStake)
	fmt.Printf("  Quorum required (validators): %d\n", quorum)
	fmt.Printf("  Quorum required (stake): %d\n", stakeQuorum)

	if validSigs >= quorum {
		fmt.Printf("  Status: %s\n", "QUORUM REACHED")
	} else {
		fmt.Printf("  Status: %s (need %d more validators)\n", "QUORUM NOT MET", quorum-validSigs)
	}

	if signedStake >= stakeQuorum {
		fmt.Printf("  Stake Status: %s\n", "QUORUM REACHED")
	} else {
		fmt.Printf("  Stake Status: %s (need %d more stake)\n", "QUORUM NOT MET", stakeQuorum-signedStake)
	}

	fmt.Printf("\nUnsigned validators:\n")
	for _, v := range manifest.Validators {
		if !signedAddrs[v.Address] {
			fmt.Printf("  - %s (%s) - stake: %d\n", v.Name, v.Address, v.Stake)
		}
	}
}

func genesisFinalize() {
	dir := getCeremonyDir()
	var manifest GenesisManifestEntry
	if err := loadJSON(filepath.Join(dir, "manifest.json"), &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load manifest: %v\n", err)
		os.Exit(1)
	}

	quorum := calculateQuorum(manifest)
	sigFiles, _ := filepath.Glob(filepath.Join(dir, "sig_*.json"))

	if len(sigFiles) < quorum {
		fmt.Println("Cannot finalize: quorum not reached")
		fmt.Printf("Need %d signatures, have %d\n", quorum, len(sigFiles))
		os.Exit(1)
	}

	manifest.GenesisTime = time.Now().UTC().Format(time.RFC3339)
	manifest.Complete = true
	manifest.Quorum = quorum

	hashBytes := computeManifestHash(manifest)
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

	if err := saveJSON(filepath.Join(dir, "genesis.json"), genesis); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save genesis: %v\n", err)
		os.Exit(1)
	}

	if err := saveJSON(filepath.Join(dir, "manifest.json"), manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save manifest: %v\n", err)
		os.Exit(1)
	}

	var state GenesisCeremonyState
	if err := loadJSON(filepath.Join(dir, "state.json"), &state); err == nil {
		state.Phase = "complete"
		saveJSON(filepath.Join(dir, "state.json"), state)
	}

	logAuditEvent("ceremony finalized", fmt.Sprintf("hash=%s validators=%d", hash, len(genesis.Validators)), "")

	fmt.Println("Genesis ceremony completed!")
	fmt.Printf("Genesis file: %s\n", filepath.Join(dir, "genesis.json"))
	fmt.Printf("Genesis hash: %s\n", hash)
	fmt.Printf("Validators: %d\n", len(genesis.Validators))
	fmt.Printf("Total stake: %d\n", genesis.TotalStake)
}

func genesisStatus() {
	dir := getCeremonyDir()

	var manifest GenesisManifestEntry
	if err := loadJSON(filepath.Join(dir, "manifest.json"), &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load manifest: %v\n", err)
		os.Exit(1)
	}

	var state GenesisCeremonyState
	if err := loadJSON(filepath.Join(dir, "state.json"), &state); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load state: %v\n", err)
		os.Exit(1)
	}

	sigFiles, _ := filepath.Glob(filepath.Join(dir, "sig_*.json"))
	quorum := calculateQuorum(manifest)

	fmt.Printf("Genesis Ceremony Status\n")
	fmt.Printf("=======================\n\n")
	fmt.Printf("Phase: %s\n", state.Phase)
	fmt.Printf("Chain ID: %d\n", manifest.ChainID)
	fmt.Printf("Network: %s\n", manifest.NetworkName)
	fmt.Printf("Genesis Time: %s\n\n", manifest.GenesisTime)

	fmt.Printf("Validators: %d\n", len(manifest.Validators))
	fmt.Printf("Total Stake: %d\n", manifest.TotalStake)
	fmt.Printf("Quorum Required: %d\n", quorum)
	fmt.Printf("Signatures Collected: %d\n\n", len(sigFiles))

	fmt.Printf("Validator Status:\n")
	for _, v := range manifest.Validators {
		signed := false
		for _, sf := range sigFiles {
			if strings.Contains(sf, v.Address) {
				signed = true
				break
			}
		}
		status := "PENDING"
		if signed {
			status = "SIGNED"
		}
		fmt.Printf("  [%s] %s (%s) - stake: %d\n", status, v.Name, v.Address, v.Stake)
	}

	progress := float64(len(sigFiles)) / float64(quorum) * 100
	if progress > 100 {
		progress = 100
	}
	fmt.Printf("\nProgress: %.1f%% (%d/%d)\n", progress, len(sigFiles), quorum)

	if len(sigFiles) >= quorum {
		fmt.Printf("Status: QUORUM REACHED\n")
	} else {
		fmt.Printf("Status: QUORUM NOT MET (need %d more)\n", quorum-len(sigFiles))
	}
}

func genesisExportPayload() {
	dir := getCeremonyDir()
	var manifest GenesisManifestEntry
	if err := loadJSON(filepath.Join(dir, "manifest.json"), &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load manifest: %v\n", err)
		os.Exit(1)
	}

	manifestHash := computeManifestHash(manifest)

	payload := SigningPayload{
		ManifestHash: hex.EncodeToString(manifestHash),
		Validators:   manifest.Validators,
		GenesisTime:  manifest.GenesisTime,
		ChainID:      manifest.ChainID,
		NetworkName:  manifest.NetworkName,
	}

	payloadFile := filepath.Join(dir, "signing_payload.json")
	if err := saveJSON(payloadFile, payload); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save payload: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Signing payload exported to: %s\n", payloadFile)
	fmt.Printf("Manifest hash: %s\n", hex.EncodeToString(manifestHash))
	fmt.Println("\nTransfer this file to your offline signing device.")
	fmt.Println("Then use: virictl genesis import-signature --file <sig_file>")
}

func genesisImportSignature() {
	var sigFile string

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--file":
			if i+1 < len(os.Args) {
				sigFile = os.Args[i+1]
				i++
			}
		}
	}

	if sigFile == "" {
		fmt.Println("Usage: virictl genesis import-signature --file <signature_file>")
		os.Exit(1)
	}

	var sig GenesisSignature
	if err := loadJSON(sigFile, &sig); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load signature file: %v\n", err)
		os.Exit(1)
	}

	dir := getCeremonyDir()
	destFile := filepath.Join(dir, fmt.Sprintf("sig_%s.json", sig.ValidatorAddress))

	if err := saveJSON(destFile, sig); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save signature: %v\n", err)
		os.Exit(1)
	}

	var state GenesisCeremonyState
	if err := loadJSON(filepath.Join(dir, "state.json"), &state); err == nil {
		state.CollectedSigs[sig.ValidatorAddress] = sig
		saveJSON(filepath.Join(dir, "state.json"), state)
	}

	logAuditEvent("signature imported", fmt.Sprintf("address=%s", sig.ValidatorAddress), sig.Signature)

	fmt.Printf("Signature imported successfully!\n")
	fmt.Printf("Validator: %s\n", sig.ValidatorAddress)
	fmt.Printf("Saved to: %s\n", destFile)
}

func genesisExport() {
	dir := getCeremonyDir()
	var manifest GenesisManifestEntry
	if err := loadJSON(filepath.Join(dir, "manifest.json"), &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load manifest: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(manifest, "", "  ")
	fmt.Println(string(data))
}

func computeManifestHash(manifest GenesisManifestEntry) []byte {
	data, _ := json.Marshal(manifest.Validators)
	h := sha256.Sum256(data)
	return h[:]
}

func calculateQuorum(manifest GenesisManifestEntry) int {
	return len(manifest.Validators)*2/3 + 1
}

func calculateStakeWeightedQuorum(manifest GenesisManifestEntry) uint64 {
	return manifest.TotalStake*2/3 + 1
}

func getCeremonyDir() string {
	for i, arg := range os.Args {
		if arg == "--dir" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return filepath.Join(".viri", "genesis")
}

func saveJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func logAuditEvent(eventType, details, signature string) {
	dir := getCeremonyDir()
	logFile := filepath.Join(dir, "ceremony_log.json")

	var events []CeremonyAuditEvent
	data, err := os.ReadFile(logFile)
	if err == nil {
		json.Unmarshal(data, &events)
	}

	event := CeremonyAuditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		EventType: eventType,
		Details:   details,
		Signature: signature,
	}

	events = append(events, event)

	saveJSON(logFile, events)
}

func readPassword() (string, error) {
	var password string
	_, err := fmt.Scanln(&password)
	return password, err
}
