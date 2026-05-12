package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Chain.ChainID != 1 {
		t.Errorf("Expected chain_id 1, got %d", cfg.Chain.ChainID)
	}

	if cfg.Chain.BlockTime != Duration(3*time.Second) {
		t.Errorf("Expected block_time 3s, got %v", cfg.Chain.BlockTime)
	}

	if cfg.Network.MaxPeers != 50 {
		t.Errorf("Expected max_peers 50, got %d", cfg.Network.MaxPeers)
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}

func TestDevConfig(t *testing.T) {
	cfg := DevConfig()

	if cfg.Chain.ChainID != 1337 {
		t.Errorf("Expected dev chain_id 1337, got %d", cfg.Chain.ChainID)
	}

	if cfg.Chain.BlockTime != Duration(500*time.Millisecond) {
		t.Errorf("Expected dev block_time 500ms, got %v", cfg.Chain.BlockTime)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected dev log level debug, got %s", cfg.Logging.Level)
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Dev config should be valid: %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(*Config)
		expectErr bool
	}{
		{
			name: "valid config",
			modify: func(c *Config) {},
			expectErr: false,
		},
		{
			name: "zero chain_id",
			modify: func(c *Config) { c.Chain.ChainID = 0 },
			expectErr: true,
		},
		{
			name: "zero block_time",
			modify: func(c *Config) { c.Chain.BlockTime = 0 },
			expectErr: true,
		},
		{
			name: "zero max_block_size",
			modify: func(c *Config) { c.Chain.MaxBlockSize = 0 },
			expectErr: true,
		},
		{
			name: "zero max_peers",
			modify: func(c *Config) { c.Network.MaxPeers = 0 },
			expectErr: true,
		},
		{
			name: "validator without key",
			modify: func(c *Config) {
				c.Node.ValidatorMode = true
				c.Node.ValidatorKey = ""
			},
			expectErr: false,
		},
		{
			name: "invalid log level",
			modify: func(c *Config) { c.Logging.Level = "invalid" },
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)

			err := cfg.Validate()
			if tt.expectErr && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestConfigSaveAndLoad(t *testing.T) {
	cfg := DevConfig()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.Chain.ChainID != cfg.Chain.ChainID {
		t.Errorf("Loaded chain_id %d != expected %d", loaded.Chain.ChainID, cfg.Chain.ChainID)
	}

	if loaded.Chain.BlockTime != cfg.Chain.BlockTime {
		t.Errorf("Loaded block_time %v != expected %v", loaded.Chain.BlockTime, cfg.Chain.BlockTime)
	}
}

func TestConfigDefaultsApplied(t *testing.T) {
	cfg := &Config{}

	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for empty config")
	}

	cfg = &Config{
		Chain: ChainConfig{
			ChainID: 1,
		},
		Consensus: ConsensusConfig{
			MinStake:      1_000_000,
			MaxValidators: 100,
		},
	}

	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		t.Errorf("Expected validation to pass after defaults applied, got: %v", err)
	}
}
