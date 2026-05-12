package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Duration is a custom type that supports both string ("1s", "500ms") and
// numeric (nanoseconds) JSON values for time.Duration fields.
type Duration time.Duration

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		parsed, err := time.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", s, err)
		}
		*d = Duration(parsed)
		return nil
	}

	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	*d = Duration(n)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(int64(d), 10)), nil
}

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

type Config struct {
	Chain     ChainConfig     `json:"chain"`
	Network   NetworkConfig   `json:"network"`
	Node      NodeConfig      `json:"node"`
	Consensus ConsensusConfig `json:"consensus"`
	Storage   StorageConfig   `json:"storage"`
	Logging   LoggingConfig   `json:"logging"`
	Readiness ReadinessConfig `json:"readiness"`
}

type ChainConfig struct {
	ChainID        uint64   `json:"chain_id"`
	NetworkName    string   `json:"network_name"`
	BlockTime      Duration `json:"block_time"`
	MaxBlockSize   uint64   `json:"max_block_size"`
	MaxGasPerBlock uint64   `json:"max_gas_per_block"`
	GenesisFile    string   `json:"genesis_file"`
}

type NetworkConfig struct {
	ListenAddr       string   `json:"listen_addr"`
	ExternalAddr     string   `json:"external_addr"`
	BootstrapPeers   []string `json:"bootstrap_peers"`
	MaxPeers         int      `json:"max_peers"`
	EnableDHT        bool     `json:"enable_dht"`
	EnableNAT        bool     `json:"enable_nat"`
}

type NodeConfig struct {
	Name          string `json:"name"`
	DataDir       string `json:"data_dir"`
	ValidatorMode bool   `json:"validator_mode"`
	ValidatorKey  string `json:"validator_key"`
	RPCEnabled    bool   `json:"rpc_enabled"`
	RPCPort       int    `json:"rpc_port"`
	APIEnabled    bool   `json:"api_enabled"`
	APIPort       int    `json:"api_port"`
	TLSCertPath   string `json:"tls_cert_path"`
	TLSKeyPath    string `json:"tls_key_path"`
	APIKeyHash    string `json:"api_key_hash"`
}

type ConsensusConfig struct {
	MinStake          uint64   `json:"min_stake"`
	MaxValidators     int      `json:"max_validators"`
	EpochLength       uint64   `json:"epoch_length"`
	SlashingEnabled   bool     `json:"slashing_enabled"`
	FinalityThreshold Duration `json:"finality_threshold"`
}

type StorageConfig struct {
	Backend          string `json:"backend"`
	Path             string `json:"path"`
	MaxStateSize     int64  `json:"max_state_size"`
	PruningEnabled   bool   `json:"pruning_enabled"`
	PruningKeepRecent uint64 `json:"pruning_keep_recent"`
	ArchiveMode      bool   `json:"archive_mode"`
}

type LoggingConfig struct {
	Level      string `json:"level"`
	Format     string `json:"format"`
	Output     string `json:"output"`
	MaxSize    int    `json:"max_size"`
	MaxBackups int    `json:"max_backups"`
}

type ReadinessConfig struct {
	MinPeers       int  `json:"min_peers"`
	MinBlockHeight uint64 `json:"min_block_height"`
	ForceReady     bool `json:"force_ready"`
}

func DefaultConfig() *Config {
	return &Config{
		Chain: ChainConfig{
			ChainID:        1,
			NetworkName:    "viri-mainnet",
			BlockTime:      Duration(3 * time.Second),
			MaxBlockSize:   10 * 1024 * 1024,
			MaxGasPerBlock: 30_000_000,
		},
		Network: NetworkConfig{
			ListenAddr:     "0.0.0.0:30303",
			MaxPeers:       50,
			EnableDHT:      true,
			EnableNAT:      true,
			BootstrapPeers: []string{},
		},
		Node: NodeConfig{
			Name:          "viri-node",
			DataDir:       defaultDataDir(),
			ValidatorMode: false,
			ValidatorKey:  "",
			RPCEnabled:    true,
			RPCPort:       8545,
			APIEnabled:    true,
			APIPort:       8546,
		},
		Consensus: ConsensusConfig{
			MinStake:          10_000_000,
			MaxValidators:     100,
			EpochLength:       1000,
			SlashingEnabled:   true,
			FinalityThreshold: Duration(2 * time.Second),
		},
		Storage: StorageConfig{
			Backend:           "leveldb",
			Path:              filepath.Join(defaultDataDir(), "chaindata"),
			MaxStateSize:      10 * 1024 * 1024 * 1024,
			PruningEnabled:    true,
			PruningKeepRecent: 100_000,
			ArchiveMode:       false,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    100,
			MaxBackups: 3,
		},
		Readiness: ReadinessConfig{
			MinPeers:       3,
			MinBlockHeight: 1,
			ForceReady:     false,
		},
	}
}

func TestnetConfig() *Config {
	cfg := DefaultConfig()
	cfg.Chain.ChainID = 999
	cfg.Chain.NetworkName = "viri-testnet"
	cfg.Chain.BlockTime = Duration(time.Second)
	cfg.Network.ListenAddr = "0.0.0.0:30303"
	cfg.Network.MaxPeers = 100
	cfg.Network.BootstrapPeers = []string{
		"/ip4/52.14.23.45/tcp/30303/p2p/16Uiu2HAmTestnetBootstrap1",
		"/ip4/3.14.56.78/tcp/30303/p2p/16Uiu2HAmTestnetBootstrap2",
	}
	cfg.Node.Name = "testnet-validator"
	cfg.Node.DataDir = ".viri-testnet"
	cfg.Storage.Path = filepath.Join(".viri-testnet", "chaindata")
	cfg.Storage.PruningKeepRecent = 500_000
	cfg.Logging.Level = "info"
	cfg.Logging.Format = "json"
	cfg.Consensus.MinStake = 100_000
	cfg.Consensus.MaxValidators = 50
	cfg.Consensus.EpochLength = 5000
	cfg.Readiness.MinPeers = 3
	cfg.Readiness.MinBlockHeight = 1
	return cfg
}

func DevConfig() *Config {
	cfg := DefaultConfig()
	cfg.Chain.ChainID = 1337
	cfg.Chain.NetworkName = "viri-dev"
	cfg.Chain.BlockTime = Duration(500 * time.Millisecond)
	cfg.Node.Name = "dev-node"
	cfg.Node.DataDir = ".viri-dev"
	cfg.Storage.Path = filepath.Join(".viri-dev", "chaindata")
	cfg.Logging.Level = "debug"
	cfg.Logging.Format = "text"
	cfg.Consensus.MinStake = 1_000
	cfg.Consensus.MaxValidators = 10
	cfg.Consensus.EpochLength = 100
	cfg.Readiness.ForceReady = true
	return cfg
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.applyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func LoadConfigOrDefault(path string) (*Config, error) {
	if path == "" {
		cfg := DevConfig()
		cfg.applyDefaults()
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("invalid configuration: %w", err)
		}
		return cfg, nil
	}

	if _, err := os.Stat(path); err != nil {
		cfg := DevConfig()
		cfg.applyDefaults()
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("invalid configuration: %w", err)
		}
		return cfg, nil
	}

	return LoadConfig(path)
}

func (c *Config) ApplyEnvOverrides() {
	if v := os.Getenv("VIRI_CHAIN_ID"); v != "" {
		var chainID uint64
		if _, err := fmt.Sscanf(v, "%d", &chainID); err == nil && chainID > 0 {
			c.Chain.ChainID = chainID
		}
	}
	if v := os.Getenv("VIRI_NETWORK_NAME"); v != "" {
		c.Chain.NetworkName = v
	}
	if v := os.Getenv("VIRI_NODE_NAME"); v != "" {
		c.Node.Name = v
	}
	if v := os.Getenv("VIRI_DATA_DIR"); v != "" {
		c.Node.DataDir = v
	}
	if v := os.Getenv("VIRI_RPC_PORT"); v != "" {
		var p int
		if _, err := fmt.Sscanf(v, "%d", &p); err == nil && p > 0 {
			c.Node.RPCPort = p
		}
	}
	if v := os.Getenv("VIRI_API_PORT"); v != "" {
		var p int
		if _, err := fmt.Sscanf(v, "%d", &p); err == nil && p > 0 {
			c.Node.APIPort = p
		}
	}
	if v := os.Getenv("VIRI_LOG_LEVEL"); v != "" {
		c.Logging.Level = v
	}
	if v := os.Getenv("VIRI_READINESS_MIN_PEERS"); v != "" {
		var p int
		if _, err := fmt.Sscanf(v, "%d", &p); err == nil && p >= 0 {
			c.Readiness.MinPeers = p
		}
	}
	if v := os.Getenv("VIRI_READINESS_MIN_HEIGHT"); v != "" {
		var h uint64
		if _, err := fmt.Sscanf(v, "%d", &h); err == nil {
			c.Readiness.MinBlockHeight = h
		}
	}
	if v := os.Getenv("VIRI_READINESS_FORCE"); v != "" {
		c.Readiness.ForceReady = v == "1" || v == "true"
	}
	if v := os.Getenv("VIRI_TLS_CERT"); v != "" {
		c.Node.TLSCertPath = v
	}
	if v := os.Getenv("VIRI_TLS_KEY"); v != "" {
		c.Node.TLSKeyPath = v
	}
	if v := os.Getenv("VIRI_API_KEY_HASH"); v != "" {
		c.Node.APIKeyHash = v
	}
}

func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func (c *Config) Validate() error {
	if c.Chain.ChainID == 0 {
		return fmt.Errorf("chain_id must be greater than 0")
	}

	if c.Chain.BlockTime <= 0 {
		return fmt.Errorf("block_time must be positive")
	}

	if c.Chain.MaxBlockSize == 0 {
		return fmt.Errorf("max_block_size must be greater than 0")
	}

	if c.Chain.MaxGasPerBlock == 0 {
		return fmt.Errorf("max_gas_per_block must be greater than 0")
	}

	if c.Network.MaxPeers <= 0 {
		return fmt.Errorf("max_peers must be greater than 0")
	}

	// Validator key may be provided via keystore/env; don't hard-require here.

	if c.Consensus.MinStake == 0 {
		return fmt.Errorf("min_stake must be greater than 0")
	}

	if c.Consensus.MaxValidators <= 0 {
		return fmt.Errorf("max_validators must be greater than 0")
	}

	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	return nil
}

func (c *Config) applyDefaults() {
	if c.Chain.BlockTime <= 0 {
		c.Chain.BlockTime = Duration(time.Second)
	}
	if c.Chain.MaxBlockSize == 0 {
		c.Chain.MaxBlockSize = 10 * 1024 * 1024
	}
	if c.Chain.MaxGasPerBlock == 0 {
		c.Chain.MaxGasPerBlock = 30_000_000
	}
	if c.Network.MaxPeers == 0 {
		c.Network.MaxPeers = 50
	}
	if c.Network.ListenAddr == "" {
		c.Network.ListenAddr = "0.0.0.0:30303"
	}
	if c.Node.DataDir == "" {
		c.Node.DataDir = defaultDataDir()
	}
	if c.Storage.Path == "" {
		c.Storage.Path = filepath.Join(c.Node.DataDir, "chaindata")
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Consensus.EpochLength == 0 {
		c.Consensus.EpochLength = 1000
	}
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".viri"
	}
	return filepath.Join(home, ".viri")
}
