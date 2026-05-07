// Viri Testnet Generator - Creates complete testnet deployment
// Usage: go run deploy/cmd/generate-testnet/main.go [flags]
package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ripemd160"
)

type ValidatorKey struct {
	Address   string `json:"address"`
	PublicKey string `json:"public_key"`
	Stake     uint64 `json:"stake"`
	Name      string `json:"name"`
}

type GenesisConfig struct {
	ChainID     uint64           `json:"chain_id"`
	NetworkName string           `json:"network_name"`
	GenesisTime string           `json:"genesis_time"`
	Hash        string           `json:"hash"`
	Validators  []ValidatorKey   `json:"validators"`
	TotalStake  uint64           `json:"total_stake"`
	Version     string           `json:"version"`
}

type NodeConfig struct {
	Chain     ChainConfig     `json:"chain"`
	Network   NetworkConfig   `json:"network"`
	Node      NodeConfig2     `json:"node"`
	Consensus ConsensusConfig `json:"consensus"`
	Storage   StorageConfig   `json:"storage"`
	Logging   LoggingConfig   `json:"logging"`
	Readiness ReadinessConfig `json:"readiness"`
}

type ChainConfig struct {
	ChainID        uint64 `json:"chain_id"`
	NetworkName    string `json:"network_name"`
	BlockTime      string `json:"block_time"`
	MaxBlockSize   int    `json:"max_block_size"`
	MaxGasPerBlock int    `json:"max_gas_per_block"`
	GenesisFile    string `json:"genesis_file"`
}

type NetworkConfig struct {
	ListenAddr    string   `json:"listen_addr"`
	BootstrapPeers []string `json:"bootstrap_peers"`
	MaxPeers      int      `json:"max_peers"`
	EnableDHT     bool     `json:"enable_dht"`
	EnableNAT     bool     `json:"enable_nat"`
}

type NodeConfig2 struct {
	Name          string `json:"name"`
	DataDir       string `json:"data_dir"`
	ValidatorMode bool   `json:"validator_mode"`
	ValidatorKey  string `json:"validator_key"`
	RPCEnabled    bool   `json:"rpc_enabled"`
	RPCPort       int    `json:"rpc_port"`
	APIEnabled    bool   `json:"api_enabled"`
	APIPort       int    `json:"api_port"`
	MetricsEnabled bool  `json:"metrics_enabled"`
	MetricsPort   int    `json:"metrics_port"`
}

type ConsensusConfig struct {
	MinStake       uint64 `json:"min_stake"`
	MaxValidators  int    `json:"max_validators"`
	EpochLength    int    `json:"epoch_length"`
	SlashingEnabled bool  `json:"slashing_enabled"`
	FinalityThreshold string `json:"finality_threshold"`
}

type StorageConfig struct {
	Backend          string `json:"backend"`
	Path             string `json:"path"`
	MaxStateSize     int64  `json:"max_state_size"`
	PruningEnabled   bool   `json:"pruning_enabled"`
	PruningKeepRecent int   `json:"pruning_keep_recent"`
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
	MinPeers      int  `json:"min_peers"`
	MinBlockHeight int `json:"min_block_height"`
	ForceReady    bool `json:"force_ready"`
}

type ComposeService struct {
	Image         string            `json:"image,omitempty"`
	Build         map[string]string `json:"build,omitempty"`
	ContainerName string            `json:"container_name"`
	Ports         []string          `json:"ports,omitempty"`
	Volumes       []string          `json:"volumes,omitempty"`
	Environment   []string          `json:"environment,omitempty"`
	Restart       string            `json:"restart,omitempty"`
	Healthcheck   *Healthcheck      `json:"healthcheck,omitempty"`
	DependsOn     []string          `json:"depends_on,omitempty"`
	Command       []string          `json:"command,omitempty"`
}

type Healthcheck struct {
	Test     []string `json:"test"`
	Interval string   `json:"interval"`
	Timeout  string   `json:"timeout"`
	Retries  int      `json:"retries"`
}

type DockerCompose struct {
	Version  string                    `yaml:"version" json:"version"`
	Services map[string]ComposeService `json:"services"`
	Volumes  map[string]interface{}    `json:"volumes,omitempty"`
}

func generateKey() (privHex, pubHex, address string) {
	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pubKey := privKey.PublicKey
	pubKeyBytes := elliptic.Marshal(pubKey.Curve, pubKey.X, pubKey.Y)
	hash := sha256.Sum256(pubKeyBytes)
	ripemd := ripemd160.New()
	ripemd.Write(hash[:])
	address = hex.EncodeToString(ripemd.Sum(nil))
	return hex.EncodeToString(privKey.D.Bytes()), hex.EncodeToString(pubKeyBytes), address
}

func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func main() {
	validators := 4
	chainID := uint64(1337)
	stake := uint64(1000000)
	outputDir := "./testnet"
	monitoring := false

	if len(os.Args) > 1 {
		for i := 1; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--validators":
				i++
				fmt.Sscanf(os.Args[i], "%d", &validators)
			case "--chain-id":
				i++
				fmt.Sscanf(os.Args[i], "%d", &chainID)
			case "--stake":
				i++
				fmt.Sscanf(os.Args[i], "%d", &stake)
			case "--output-dir":
				i++
				outputDir = os.Args[i]
			case "--monitoring":
				monitoring = true
			}
		}
	}

	fmt.Printf("==> Testnet Configuration\n")
	fmt.Printf("  Validators: %d\n", validators)
	fmt.Printf("  Chain ID:   %d\n", chainID)
	fmt.Printf("  Stake/Node: %d\n", stake)
	fmt.Printf("  Output:     %s\n", outputDir)
	fmt.Printf("  Monitoring: %v\n\n", monitoring)

	// Clean output dir
	os.RemoveAll(outputDir)

	// Create directories
	os.MkdirAll(filepath.Join(outputDir, "genesis"), 0755)
	os.MkdirAll(filepath.Join(outputDir, "keys"), 0755)

	var validatorKeys []ValidatorKey
	var privKeys []string

	fmt.Println("==> Generating validator keys...")
	for i := 0; i < validators; i++ {
		privHex, pubHex, address := generateKey()
		privKeys = append(privKeys, privHex)
		key := ValidatorKey{
			Address:   "0x" + address,
			PublicKey: pubHex,
			Stake:     stake,
			Name:      fmt.Sprintf("validator-%d", i),
		}
		validatorKeys = append(validatorKeys, key)

		// Save key files
		os.WriteFile(
			filepath.Join(outputDir, "keys", fmt.Sprintf("validator-%d.json", i)),
			mustJSON(key),
			0644,
		)
		os.WriteFile(
			filepath.Join(outputDir, "keys", fmt.Sprintf("validator-%d.key", i)),
			[]byte(privHex),
			0600,
		)

		fmt.Printf("  [SUCCESS] Validator %d: %s...\n", i, key.Address[:18])
	}

	// Create genesis
	fmt.Println("\n==> Creating genesis configuration...")
	genesis := GenesisConfig{
		ChainID:     chainID,
		NetworkName: "viri-testnet",
		GenesisTime: time.Now().UTC().Format(time.RFC3339),
		Hash:        "",
		Validators:  validatorKeys,
		TotalStake:  stake * uint64(validators),
		Version:     "0.1.0",
	}
	writeJSON(filepath.Join(outputDir, "genesis", "genesis.json"), genesis)
	fmt.Printf("  [SUCCESS] Genesis created (total stake: %d)\n", genesis.TotalStake)

	// Generate docker-compose.yml
	fmt.Println("\n==> Generating docker-compose.yml...")
	composeYAML := generateComposeYAML(validators, chainID, monitoring)
	os.WriteFile(filepath.Join(outputDir, "docker-compose.yml"), []byte(composeYAML), 0644)
	fmt.Println("  [SUCCESS] Docker Compose generated")

	// Generate node configs
	fmt.Println("\n==> Generating node configurations...")
	for i := 0; i < validators; i++ {
		configDir := filepath.Join(outputDir, "configs", fmt.Sprintf("validator-%d", i))
		os.MkdirAll(configDir, 0755)

		// Build bootstrap peers
		var peers []string
		for j := 0; j < validators; j++ {
			if i != j {
				peers = append(peers, fmt.Sprintf("/dns/validator-%d/tcp/30303/p2p/placeholder", j))
			}
		}

		nodeConfig := NodeConfig{
			Chain: ChainConfig{
				ChainID:        chainID,
				NetworkName:    "viri-testnet",
				BlockTime:      "1s",
				MaxBlockSize:   10485760,
				MaxGasPerBlock: 30000000,
				GenesisFile:    "/home/viri/data/genesis.json",
			},
			Network: NetworkConfig{
				ListenAddr:    "0.0.0.0:30303",
				BootstrapPeers: peers,
				MaxPeers:      50,
				EnableDHT:     true,
				EnableNAT:     false,
			},
			Node: NodeConfig2{
				Name:          fmt.Sprintf("validator-%d", i),
				DataDir:       "/home/viri/.viri",
				ValidatorMode: true,
				ValidatorKey:  "/keys/validator.key",
				RPCEnabled:    true,
				RPCPort:       8545,
				APIEnabled:    true,
				APIPort:       8546,
				MetricsEnabled: true,
				MetricsPort:   9090,
			},
			Consensus: ConsensusConfig{
				MinStake:       10000,
				MaxValidators:  validators,
				EpochLength:    100,
				SlashingEnabled: true,
				FinalityThreshold: "2s",
			},
			Storage: StorageConfig{
				Backend:          "leveldb",
				Path:             "/home/viri/.viri/chaindata",
				MaxStateSize:     10737418240,
				PruningEnabled:   true,
				PruningKeepRecent: 100000,
				ArchiveMode:      false,
			},
			Logging: LoggingConfig{
				Level:      "info",
				Format:     "json",
				Output:     "stdout",
				MaxSize:    100,
				MaxBackups: 3,
			},
			Readiness: ReadinessConfig{
				MinPeers:      1,
				MinBlockHeight: 0,
				ForceReady:    false,
			},
		}

		writeJSON(filepath.Join(configDir, "config.json"), nodeConfig)
		os.WriteFile(
			filepath.Join(configDir, "validator.key"),
			[]byte(privKeys[i]),
			0600,
		)
	}
	fmt.Println("  [SUCCESS] Node configurations created")

	// Generate summary
	fmt.Println("\n==> Deployment Summary")
	fmt.Println("========================================")
	fmt.Println("  Viri Blockchain Testnet - Summary")
	fmt.Println("========================================")
	fmt.Printf("\nChain ID:     %d\n", chainID)
	fmt.Printf("Network:      viri-testnet\n")
	fmt.Printf("Validators:   %d\n", validators)
	fmt.Printf("Total Stake:  %d\n", genesis.TotalStake)
	fmt.Println("\n----------------------------------------")
	fmt.Println("  Quick Start")
	fmt.Println("----------------------------------------")
	fmt.Println("\n  cd", outputDir)
	fmt.Println("  docker compose up -d    # Start all nodes")
	fmt.Println("  docker compose logs -f  # View logs")
	fmt.Println("  docker compose down     # Stop all nodes")
	fmt.Println("\n========================================")
	fmt.Printf("\n[SUCCESS] Testnet deployment ready at: %s\n", outputDir)
}

func mustJSON(v interface{}) []byte {
	data, _ := json.MarshalIndent(v, "", "  ")
	return data
}

func generateComposeYAML(validators int, chainID uint64, monitoring bool) string {
	var buf bytes.Buffer

	buf.WriteString("version: \"3.8\"\n\n")
	buf.WriteString("services:\n")

	for i := 0; i < validators; i++ {
		p2pPort := 30303 + i*10
		rpcPort := 8545 + i
		apiPort := 8546 + i

		fmt.Fprintf(&buf, "  validator-%d:\n", i)
		buf.WriteString("    build:\n")
		buf.WriteString("      context: ../\n")
		buf.WriteString("      dockerfile: Dockerfile\n")
		fmt.Fprintf(&buf, "    container_name: viri-validator-%d\n", i)
		buf.WriteString("    ports:\n")
		fmt.Fprintf(&buf, "      - \"%d:30303\"\n", p2pPort)
		fmt.Fprintf(&buf, "      - \"%d:8545\"\n", rpcPort)
		fmt.Fprintf(&buf, "      - \"%d:8546\"\n", apiPort)
		buf.WriteString("    volumes:\n")
		fmt.Fprintf(&buf, "      - ./configs/validator-%d:/home/viri/config:ro\n", i)
		buf.WriteString("      - ./genesis/genesis.json:/home/viri/data/genesis.json:ro\n")
		fmt.Fprintf(&buf, "      - validator-%d-data:/home/viri/.viri\n", i)
		buf.WriteString("    environment:\n")
		fmt.Fprintf(&buf, "      - VIRI_NODE_NAME=validator-%d\n", i)
		buf.WriteString("      - VIRI_DATA_DIR=/home/viri/.viri\n")
		buf.WriteString("      - VIRI_RPC_PORT=8545\n")
		buf.WriteString("      - VIRI_API_PORT=8546\n")
		buf.WriteString("      - VIRI_LOG_LEVEL=info\n")
		buf.WriteString("      - VIRI_VALIDATOR_MODE=true\n")
		buf.WriteString("      - VIRI_VALIDATOR_KEY=/keys/validator.key\n")
		fmt.Fprintf(&buf, "      - VIRI_CHAIN_ID=%d\n", chainID)
		buf.WriteString("    restart: unless-stopped\n")
		buf.WriteString("    healthcheck:\n")
		buf.WriteString("      test: [\"CMD\", \"curl\", \"-f\", \"http://localhost:9090/health\"]\n")
		buf.WriteString("      interval: 10s\n")
		buf.WriteString("      timeout: 5s\n")
		buf.WriteString("      retries: 3\n")
		buf.WriteString("\n")
	}

	if monitoring {
		buf.WriteString("  prometheus:\n")
		buf.WriteString("    image: prom/prometheus:latest\n")
		buf.WriteString("    container_name: viri-prometheus\n")
		buf.WriteString("    ports:\n")
		buf.WriteString("      - \"9091:9090\"\n")
		buf.WriteString("    volumes:\n")
		buf.WriteString("      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro\n")
		buf.WriteString("      - prometheus-data:/prometheus\n")
		buf.WriteString("    restart: unless-stopped\n")
		buf.WriteString("    command:\n")
		buf.WriteString("      - \"--config.file=/etc/prometheus/prometheus.yml\"\n")
		buf.WriteString("      - \"--storage.tsdb.path=/prometheus\"\n")
		buf.WriteString("      - \"--web.enable-lifecycle\"\n")
		buf.WriteString("\n")

		buf.WriteString("  grafana:\n")
		buf.WriteString("    image: grafana/grafana:latest\n")
		buf.WriteString("    container_name: viri-grafana\n")
		buf.WriteString("    ports:\n")
		buf.WriteString("      - \"3000:3000\"\n")
		buf.WriteString("    environment:\n")
		buf.WriteString("      - GF_SECURITY_ADMIN_PASSWORD=admin\n")
		buf.WriteString("      - GF_USERS_ALLOW_SIGN_UP=false\n")
		buf.WriteString("    volumes:\n")
		buf.WriteString("      - grafana-data:/var/lib/grafana\n")
		buf.WriteString("      - ./grafana:/etc/grafana/provisioning:ro\n")
		buf.WriteString("    restart: unless-stopped\n")
		buf.WriteString("    depends_on:\n")
		buf.WriteString("      - prometheus\n")
		buf.WriteString("\n")
	}

	buf.WriteString("volumes:\n")
	for i := 0; i < validators; i++ {
		fmt.Fprintf(&buf, "  validator-%d-data:\n", i)
	}
	if monitoring {
		buf.WriteString("  prometheus-data:\n")
		buf.WriteString("  grafana-data:\n")
	}

	return buf.String()
}

type composeSvc struct {
	name string
	svc  ComposeService
}

func yamlList(prefix string, items []string) string {
	var buf bytes.Buffer
	for _, item := range items {
		fmt.Fprintf(&buf, "%s- \"%s\"\n", prefix, item)
	}
	return buf.String()
}

func yamlMap(prefix string, m map[string]string) string {
	var buf bytes.Buffer
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&buf, "%s%s: %s\n", prefix, k, m[k])
	}
	return buf.String()
}

func yamlStr(s string) string {
	if strings.ContainsAny(s, ":#{}[]|>&*!?%@`") || s == "" {
		return fmt.Sprintf("\"%s\"", strings.ReplaceAll(s, "\"", "\\\""))
	}
	return s
}
