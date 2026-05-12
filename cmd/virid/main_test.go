package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/viri-chain/viri/internal/layer1/config"
	"github.com/viri-chain/viri/internal/layer1/logging"
)

func TestParseFlagsDefaults(t *testing.T) {
	old := os.Args
	os.Args = []string{"virid"}
	defer func() { os.Args = old }()

	flags := parseFlags()
	if flags.p2pPort == 0 || flags.rpcPort == 0 || flags.apiPort == 0 {
		t.Fatalf("expected default ports")
	}
	if flags.name == "" {
		t.Fatalf("expected default name")
	}
}

func TestConfigOverrides(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.json")
	data := []byte(`{"chain":{"chain_id":2024,"network_name":"viri-prod","block_time":1000000000,"max_block_size":1048576,"max_gas_per_block":30000000},"network":{"listen_addr":"0.0.0.0:30303","max_peers":50,"enable_dht":true,"enable_nat":true},"node":{"name":"node-a","data_dir":"` + root + `","validator_mode":false,"rpc_enabled":true,"rpc_port":8545,"api_enabled":true,"api_port":8546},"consensus":{"min_stake":1000,"max_validators":10,"epoch_length":100,"slashing_enabled":true,"finality_threshold":1000000000},"storage":{"backend":"leveldb","path":"` + root + `\\chaindata","max_state_size":1000,"pruning_enabled":true,"pruning_keep_recent":1000,"archive_mode":false},"logging":{"level":"info","format":"text","output":"stdout","max_size":10,"max_backups":1}}`)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	old := os.Args
	os.Args = []string{"virid", "--config", configPath, "--chain-id", "3030", "--data-dir", root}
	defer func() { os.Args = old }()

	flags := parseFlags()
	if flags.config != configPath {
		t.Fatalf("expected config path")
	}
	if flags.chainID != 3030 {
		t.Fatalf("expected chain-id override")
	}
}

func TestGetDefaultDataDir(t *testing.T) {
	if dir := getDefaultDataDir(); dir == "" {
		t.Fatalf("expected data dir")
	}
}

func TestLoadKeyGeneratesAndLoads(t *testing.T) {
	os.Setenv("VIRI_KEY_PASSPHRASE", "test-key-passphrase-67890")
	root := t.TempDir()
	flags := nodeFlags{dataDir: root}
	log := logging.NewLogger("test", logging.INFO, "text")
	cfg := config.DevConfig()

	key1 := loadKey(flags, cfg, log)
	if key1 == nil {
		t.Fatalf("expected key")
	}

	key2 := loadKey(flags, cfg, log)
	if key2 == nil {
		t.Fatalf("expected key")
	}
}

func TestInitDBFallback(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing", "badger")
	flags := nodeFlags{dataDir: root}
	log := logging.NewLogger("test", logging.INFO, "text")

	store := initDB(flags, log)
	if store == nil {
		t.Fatalf("expected store")
	}
	_ = store.Close()
}
