package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

type ValidatorEntry struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
	Stake     uint64 `json:"stake"`
}

type GenesisConfig struct {
	ChainID     uint64            `json:"chain_id"`
	NetworkName string            `json:"network_name"`
	GenesisTime string            `json:"genesis_time"`
	Validators  []ValidatorEntry  `json:"validators"`
	TotalStake  uint64            `json:"total_stake"`
	Version     string            `json:"version"`
}

func main() {
	chainID := 1
	networkName := "viri-mainnet"
	validatorCount := 4
	stake := uint64(1000000)
	passphrase := ""
	outputDir := "./.viri/genesis-ceremony"
	outputFile := "configs/genesis/mainnet.json"

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--chain-id":
			if i+1 < len(args) { fmt.Sscanf(args[i+1], "%d", &chainID); i++ }
		case "--network":
			if i+1 < len(args) { networkName = args[i+1]; i++ }
		case "--validators":
			if i+1 < len(args) { fmt.Sscanf(args[i+1], "%d", &validatorCount); i++ }
		case "--stake":
			if i+1 < len(args) { fmt.Sscanf(args[i+1], "%d", &stake); i++ }
		case "--passphrase":
			if i+1 < len(args) { passphrase = args[i+1]; i++ }
		case "--dir":
			if i+1 < len(args) { outputDir = args[i+1]; i++ }
		case "--output":
			if i+1 < len(args) { outputFile = args[i+1]; i++ }
		}
	}

	if passphrase == "" {
		fmt.Fprintf(os.Stderr, "Error: --passphrase is required\n")
		os.Exit(1)
	}

	if err := os.MkdirAll(outputDir+"/keys", 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create dir: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== Genesis Ceremony ===\n")
	fmt.Printf("Chain ID: %d\n", chainID)
	fmt.Printf("Network: %s\n", networkName)
	fmt.Printf("Validators: %d\n", validatorCount)
	fmt.Printf("Stake per validator: %d\n\n", stake)

	var validators []ValidatorEntry
	var totalStake uint64

	for i := 0; i < validatorCount; i++ {
		name := fmt.Sprintf("validator-%d", i)
		keyFile := filepath.Join(outputDir, "keys", name+".json")

		privKey, err := crypto.GenerateKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate key %d: %v\n", i, err)
			os.Exit(1)
		}

		if err := crypto.EncryptKey(privKey, passphrase, keyFile); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to encrypt key %d: %v\n", i, err)
			os.Exit(1)
		}

		pubKey := privKey.PubKey()
		pubHex := pubKey.Hex()
		addr := "0x" + pubHex
		if len(pubHex) > 40 {
			addr = "0x" + pubHex[:40]
		}

		validators = append(validators, ValidatorEntry{
			Name:      name,
			Address:   addr,
			PublicKey: pubHex,
			Stake:     stake,
		})
		totalStake += stake

		fmt.Printf("  Generated: %s\n", name)
		fmt.Printf("    Address:  %s\n", addr)
		fmt.Printf("    Key file: %s\n", keyFile)
	}

	fmt.Printf("\n=== Computing genesis hash ===\n")

	genesis := GenesisConfig{
		ChainID:     uint64(chainID),
		NetworkName: networkName,
		GenesisTime: "2026-05-17T00:00:00Z",
		Validators:  validators,
		TotalStake:  totalStake,
		Version:     "0.1.0",
	}

	sigData, _ := json.Marshal(genesis)
	genesisHash := sha256.Sum256(sigData)

	fmt.Printf("Genesis hash: %s\n\n", hex.EncodeToString(genesisHash[:]))

	fmt.Printf("=== Signing with validator keys ===\n")
	for _, v := range validators {
		keyFile := filepath.Join(outputDir, "keys", v.Name+".json")
		privKey, err := crypto.DecryptKey(keyFile, passphrase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to decrypt %s: %v\n", v.Name, err)
			os.Exit(1)
		}
		sig, err := privKey.Sign(genesisHash[:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to sign %s: %v\n", v.Name, err)
			os.Exit(1)
		}
		sigHex := hex.EncodeToString(sig.Bytes())
		fmt.Printf("  Signed: %s (%s...)\n", v.Name, sigHex[:16])
	}
	fmt.Println()

	fmt.Printf("=== Finalizing ===\n")
	genesisData, _ := json.MarshalIndent(genesis, "", "  ")

	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output dir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outputFile, genesisData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write genesis: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "genesis.json"), genesisData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write genesis to ceremony dir: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Genesis written to: %s\n\n", outputFile)
	fmt.Printf("=== Ceremony Complete ===\n")
	fmt.Printf("Chain ID:       %d\n", chainID)
	fmt.Printf("Network:        %s\n", networkName)
	fmt.Printf("Validators:     %d\n", len(validators))
	fmt.Printf("Total stake:    %d\n", totalStake)
	fmt.Printf("Genesis hash:   %s\n", hex.EncodeToString(genesisHash[:]))
	fmt.Printf("Key directory:  %s/keys/\n", outputDir)
	fmt.Println()
	fmt.Println("IMPORTANT: Keep all key files in keys/ directory secure!")
	fmt.Println("Distribute genesis.json to all mainnet participants.")
}
