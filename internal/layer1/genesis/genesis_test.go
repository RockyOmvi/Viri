package genesis

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultGenesis(t *testing.T) {
	g := DefaultGenesis()
	if g == nil {
		t.Fatal("expected non-nil genesis")
	}
	if g.ChainID != 7777777 {
		t.Fatalf("expected chain ID 7777777, got %d", g.ChainID)
	}
	if g.Network != "viri-devnet" {
		t.Fatalf("expected viri-devnet, got %s", g.Network)
	}
	if len(g.Validators) != 1 {
		t.Fatalf("expected 1 validator, got %d", len(g.Validators))
	}
	if g.Parameters.BlockGasLimit != 30000000 {
		t.Fatalf("expected block gas limit 30000000, got %d", g.Parameters.BlockGasLimit)
	}
}

func TestDefaultGenesisValidate(t *testing.T) {
	g := DefaultGenesis()
	if err := g.Validate(); err != nil {
		t.Fatalf("DefaultGenesis should be valid: %v", err)
	}
}

func TestValidateChainID(t *testing.T) {
	g := DefaultGenesis()
	g.ChainID = 0
	if err := g.Validate(); err == nil {
		t.Fatal("expected error for zero chain ID")
	}
}

func TestValidateNoValidators(t *testing.T) {
	g := DefaultGenesis()
	g.Validators = nil
	if err := g.Validate(); err == nil {
		t.Fatal("expected error for no validators")
	}
}

func TestValidateValidatorFields(t *testing.T) {
	tests := []struct {
		name  string
		modFn func(g *GenesisConfig)
	}{
		{"empty address", func(g *GenesisConfig) { g.Validators[0].Address = "" }},
		{"empty public key", func(g *GenesisConfig) { g.Validators[0].PublicKey = "" }},
		{"zero stake", func(g *GenesisConfig) { g.Validators[0].Stake = 0 }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := DefaultGenesis()
			tc.modFn(g)
			if err := g.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateAccountAddress(t *testing.T) {
	g := DefaultGenesis()
	g.Accounts[0].Address = ""
	if err := g.Validate(); err == nil {
		t.Fatal("expected error for empty account address")
	}

	g.Accounts[0].Address = "invalid!"
	if err := g.Validate(); err == nil {
		t.Fatal("expected error for invalid account address")
	}
}

func TestValidateBlockGasLimit(t *testing.T) {
	g := DefaultGenesis()
	g.Parameters.BlockGasLimit = 0
	if err := g.Validate(); err == nil {
		t.Fatal("expected error for zero block gas limit")
	}
}

func TestComputeHashDeterministic(t *testing.T) {
	g := DefaultGenesis()
	h1 := g.ComputeHash()
	h2 := g.ComputeHash()

	if len(h1) == 0 {
		t.Fatal("expected non-empty hash")
	}
	if string(h1) != string(h2) {
		t.Fatal("hash should be deterministic")
	}
}

func TestComputeHashDifferentConfigs(t *testing.T) {
	g1 := DefaultGenesis()
	g2 := DefaultGenesis()
	g2.ChainID = 9999999

	h1 := g1.ComputeHash()
	h2 := g2.ComputeHash()

	if string(h1) == string(h2) {
		t.Fatal("different configs should produce different hashes")
	}
}

func TestNewGenesisCeremony(t *testing.T) {
	gc := NewGenesisCeremony(3)
	if gc == nil {
		t.Fatal("expected non-nil ceremony")
	}
	if gc.Required != 3 {
		t.Fatalf("expected required=3, got %d", gc.Required)
	}
	if gc.Phase != PhaseRegistration {
		t.Fatalf("expected PhaseRegistration, got %d", gc.Phase)
	}
	if len(gc.Participants) != 0 {
		t.Fatalf("expected 0 participants, got %d", len(gc.Participants))
	}
}

func TestRegisterParticipant(t *testing.T) {
	gc := NewGenesisCeremony(2)
	p := GenesisParticipant{Address: "0xabc", PublicKey: "0x01", Stake: 1000}

	err := gc.RegisterParticipant(p)
	if err != nil {
		t.Fatalf("RegisterParticipant failed: %v", err)
	}
	if len(gc.Participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(gc.Participants))
	}
}

func TestRegisterDuplicateParticipant(t *testing.T) {
	gc := NewGenesisCeremony(2)
	p := GenesisParticipant{Address: "0xabc", PublicKey: "0x01", Stake: 1000}

	gc.RegisterParticipant(p)
	err := gc.RegisterParticipant(p)
	if err == nil {
		t.Fatal("expected error for duplicate participant")
	}
}

func TestRegisterAfterPhaseClosed(t *testing.T) {
	gc := NewGenesisCeremony(1)
	gc.Phase = PhaseCommitment

	err := gc.RegisterParticipant(GenesisParticipant{Address: "0xabc"})
	if err == nil {
		t.Fatal("expected error when registering after registration phase")
	}
}

func TestCommitParticipant(t *testing.T) {
	gc := NewGenesisCeremony(1)
	gc.RegisterParticipant(GenesisParticipant{Address: "0xabc"})
	gc.Phase = PhaseCommitment

	err := gc.Commit("0xabc", []byte("commitment"))
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if !gc.Participants[0].Committed {
		t.Fatal("participant should be committed")
	}
}

func TestCommitWrongPhase(t *testing.T) {
	gc := NewGenesisCeremony(1)
	gc.RegisterParticipant(GenesisParticipant{Address: "0xabc"})

	err := gc.Commit("0xabc", []byte("commitment"))
	if err == nil {
		t.Fatal("expected error when committing outside commitment phase")
	}
}

func TestCommitUnknownParticipant(t *testing.T) {
	gc := NewGenesisCeremony(1)
	gc.Phase = PhaseCommitment

	err := gc.Commit("0xunknown", []byte("commitment"))
	if err == nil {
		t.Fatal("expected error for unknown participant")
	}
}

func TestRevealPhase(t *testing.T) {
	gc := NewGenesisCeremony(1)
	gc.Phase = PhaseReveal

	err := gc.Reveal("0xabc", []byte("reveal"))
	if err != nil {
		t.Fatalf("Reveal failed: %v", err)
	}
}

func TestRevealWrongPhase(t *testing.T) {
	gc := NewGenesisCeremony(1)
	err := gc.Reveal("0xabc", []byte("reveal"))
	if err == nil {
		t.Fatal("expected error when revealing outside reveal phase")
	}
}

func TestAddValidator(t *testing.T) {
	gc := NewGenesisCeremony(1)
	gc.AddValidator("0xval", "0xpub", 500000)

	if len(gc.Config.Validators) != 1 {
		t.Fatalf("expected 1 validator, got %d", len(gc.Config.Validators))
	}
	if gc.Config.Validators[0].Address != "0xval" {
		t.Fatalf("expected address 0xval, got %s", gc.Config.Validators[0].Address)
	}
	if gc.Config.Validators[0].Stake != 500000 {
		t.Fatalf("expected stake 500000, got %d", gc.Config.Validators[0].Stake)
	}
}

func TestAddAccount(t *testing.T) {
	gc := NewGenesisCeremony(1)
	gc.AddAccount("0xacc", big.NewInt(1000))

	if len(gc.Config.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(gc.Config.Accounts))
	}
	if gc.Config.Accounts[0].Address != "0xacc" {
		t.Fatalf("expected address 0xacc, got %s", gc.Config.Accounts[0].Address)
	}
	if gc.Config.Accounts[0].Balance != "1000" {
		t.Fatalf("expected balance 1000, got %s", gc.Config.Accounts[0].Balance)
	}
	if gc.Config.Allocations["0xacc"].Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("expected allocation 1000, got %s", gc.Config.Allocations["0xacc"])
	}
}

func TestSetParameters(t *testing.T) {
	gc := NewGenesisCeremony(1)
	params := GenesisParameters{
		BlockGasLimit: 50000000,
		MaxTxSize:     256000,
		EpochLength:   2000,
		MinValidators: 2,
		MaxValidators: 50,
		BlockTime:     1,
		BaseFee:       "5000000000",
	}
	gc.SetParameters(params)

	if gc.Config.Parameters.BlockGasLimit != 50000000 {
		t.Fatalf("expected BlockGasLimit 50000000, got %d", gc.Config.Parameters.BlockGasLimit)
	}
}

func TestFinalize(t *testing.T) {
	gc := NewGenesisCeremony(1)
	gc.RegisterParticipant(GenesisParticipant{Address: "0xabc", PublicKey: "0x01", Stake: 1000})
	gc.AddValidator("0xval", "0xpub", 500000)

	config, err := gc.Finalize()
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}
	if gc.Phase != PhaseFinalization {
		t.Fatalf("expected PhaseFinalization, got %d", gc.Phase)
	}
}

func TestFinalizeInsufficientParticipants(t *testing.T) {
	gc := NewGenesisCeremony(3)
	gc.AddValidator("0xval", "0xpub", 500000)

	_, err := gc.Finalize()
	if err == nil {
		t.Fatal("expected error for insufficient participants")
	}
}

func TestFinalizeNoValidators(t *testing.T) {
	gc := NewGenesisCeremony(1)
	gc.RegisterParticipant(GenesisParticipant{Address: "0xabc"})

	_, err := gc.Finalize()
	if err == nil {
		t.Fatal("expected error for no validators")
	}
}

func TestSaveAndLoadGenesis(t *testing.T) {
	g := DefaultGenesis()
	path := filepath.Join(t.TempDir(), "genesis.json")

	err := g.SaveToFile(path)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	loaded, err := LoadGenesisFromFile(path)
	if err != nil {
		t.Fatalf("LoadGenesisFromFile failed: %v", err)
	}

	if loaded.ChainID != g.ChainID {
		t.Fatalf("loaded ChainID %d != original %d", loaded.ChainID, g.ChainID)
	}
	if loaded.Network != g.Network {
		t.Fatalf("loaded Network %s != original %s", loaded.Network, g.Network)
	}
	if len(loaded.Validators) != len(g.Validators) {
		t.Fatalf("loaded Validators %d != original %d", len(loaded.Validators), len(g.Validators))
	}
}

func TestLoadGenesisInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	_, err := LoadGenesisFromFile(path)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadGenesisInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(path, []byte("not json"), 0644)

	_, err := LoadGenesisFromFile(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadGenesisInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	os.WriteFile(path, []byte(`{"chain_id":0}`), 0644)

	_, err := LoadGenesisFromFile(path)
	if err == nil {
		t.Fatal("expected error for invalid genesis config")
	}
}

func TestCeremonyWithFullFlow(t *testing.T) {
	gc := NewGenesisCeremony(2)

	p1 := GenesisParticipant{Address: "0xval1", PublicKey: "0xpub1", Stake: 1000000}
	p2 := GenesisParticipant{Address: "0xval2", PublicKey: "0xpub2", Stake: 2000000}

	if err := gc.RegisterParticipant(p1); err != nil {
		t.Fatalf("register p1 failed: %v", err)
	}
	if err := gc.RegisterParticipant(p2); err != nil {
		t.Fatalf("register p2 failed: %v", err)
	}

	gc.AddValidator("0xval1", "0xpub1", 1000000)
	gc.AddValidator("0xval2", "0xpub2", 2000000)
	gc.AddAccount("abcdef1234567890abcdef1234567890abcdef12", big.NewInt(1000000000))
	gc.SetParameters(GenesisParameters{
		BlockGasLimit: 30000000,
		MaxTxSize:     128000,
		EpochLength:   1000,
		MinValidators: 2,
		MaxValidators: 50,
		BlockTime:     2,
		BaseFee:       "1000000000",
	})

	gc.Phase = PhaseCommitment
	gc.Commit("0xval1", []byte("c1"))
	gc.Commit("0xval2", []byte("c2"))

	gc.Phase = PhaseReveal
	gc.Reveal("0xval1", []byte("r1"))

	config, err := gc.Finalize()
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	hash := config.ComputeHash()
	if len(hash) == 0 {
		t.Fatal("expected non-empty hash")
	}
}

func TestCeremonyPhaseTransitions(t *testing.T) {
	gc := NewGenesisCeremony(1)

	if gc.Phase != PhaseRegistration {
		t.Fatalf("initial phase should be PhaseRegistration")
	}

	gc.RegisterParticipant(GenesisParticipant{Address: "0xval"})
	gc.AddValidator("0xval", "0xpub", 1000)

	gc.Phase = PhaseCommitment
	if gc.Phase != PhaseCommitment {
		t.Fatalf("phase should be PhaseCommitment")
	}

	gc.Phase = PhaseReveal
	if gc.Phase != PhaseReveal {
		t.Fatalf("phase should be PhaseReveal")
	}

	config, err := gc.Finalize()
	if err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}
	if config == nil {
		t.Fatal("expected non-nil config")
	}

	if gc.Phase != PhaseFinalization {
		t.Fatalf("final phase should be PhaseFinalization, got %d", gc.Phase)
	}
}
