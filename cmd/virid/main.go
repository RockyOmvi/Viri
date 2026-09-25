package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/viri-chain/viri/internal/layer1/config"
	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/logging"
	"github.com/viri-chain/viri/internal/layer1/state"
)

const Version = "0.1.0"

type nodeFlags struct {
	dataDir        string
	validator      bool
	name           string
	p2pPort        int
	rpcPort        int
	apiPort        int
	l3Port         int
	logLevel       string
	bootnodes      string
	privKey        string
	p2pKey         string
	chainID        uint64
	genesis        string
	config         string
	rpc            bool
	api            bool
	syncMode       string
	noMDNS         bool
	peerFile       string
	writePeerFile  string
	consensusDelay time.Duration
	explorerMode   bool
	faucetMode     bool
	tlsAuto        bool
	parallelExec   bool
	gnarkProver    bool
	mevMode        string
	scheme         string
	testnet        bool
}

func parseFlags() nodeFlags {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	f := nodeFlags{}

	flag.StringVar(&f.dataDir, "data-dir", "", "Data directory for node storage (default: ~/.viri)")
	flag.BoolVar(&f.validator, "validator", false, "Run as validator node")
	flag.StringVar(&f.name, "name", "viri-node", "Node name/identifier")
	flag.IntVar(&f.p2pPort, "p2p-port", 30303, "P2P listening port")
	flag.IntVar(&f.rpcPort, "rpc-port", 8545, "JSON-RPC server port")
	flag.IntVar(&f.apiPort, "api-port", 8546, "REST API server port")
	flag.IntVar(&f.l3Port, "l3-port", 8548, "L3 API server port")
	flag.StringVar(&f.logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&f.bootnodes, "bootnodes", "", "Comma-separated bootnode multiaddresses")
	flag.StringVar(&f.privKey, "private-key", "", "Validator private key (hex)")
	flag.StringVar(&f.p2pKey, "p2p-key", "", "P2P host private key (hex)")
	flag.Uint64Var(&f.chainID, "chain-id", 0, "Chain ID")
	flag.StringVar(&f.genesis, "genesis", "", "Genesis file path")
	flag.StringVar(&f.config, "config", "", "Config file path")
	flag.BoolVar(&f.rpc, "rpc", true, "Enable JSON-RPC server")
	flag.BoolVar(&f.api, "api", true, "Enable REST API server")
	flag.StringVar(&f.syncMode, "sync-mode", "fast", "Sync mode: full, fast, snap")
	flag.BoolVar(&f.noMDNS, "no-mdns", false, "Disable mDNS discovery (recommended on Windows)")
	flag.StringVar(&f.peerFile, "peer-file", "", "Read bootnode peer info from file")
	flag.StringVar(&f.writePeerFile, "write-peer-file", "", "Write this node's peer info to file")
	flag.DurationVar(&f.consensusDelay, "consensus-delay", 0, "Delay before starting consensus (e.g. 35s to allow peer discovery)")
	flag.BoolVar(&f.explorerMode, "explorer", false, "Run as block explorer")
	flag.BoolVar(&f.faucetMode, "faucet", false, "Run as testnet faucet")
	flag.BoolVar(&f.tlsAuto, "tls-auto", false, "Auto-generate self-signed TLS certificates")
	flag.BoolVar(&f.parallelExec, "parallel-exec", false, "Enable parallel transaction execution")
	flag.BoolVar(&f.gnarkProver, "gnark-prover", false, "Use gnark-based ZK prover/verifier")
	flag.StringVar(&f.mevMode, "mev-mode", "standard", "MEV resistance mode: standard, encrypted, commit-reveal")
	flag.StringVar(&f.scheme, "scheme", "ecdsa", "Crypto scheme: ecdsa, mldsa44, mldsa65, mldsa87, sphincs")
	flag.BoolVar(&f.testnet, "testnet", false, "Run in testnet mode (shorthand for --config configs/node-testnet.json)")

	flag.Parse()

	// Check env var for TLS auto mode
	if os.Getenv("VIRI_TLS_AUTO") == "true" || os.Getenv("VIRI_TLS_AUTO") == "1" {
		f.tlsAuto = true
	}

	// Check env var for testnet mode
	if os.Getenv("VIRI_TESTNET") == "true" || os.Getenv("VIRI_TESTNET") == "1" {
		f.testnet = true
	}

	// Validate crypto scheme
	if _, ok := crypto.ParseScheme(f.scheme); !ok {
		fmt.Fprintf(os.Stderr, "ERROR: unknown crypto scheme %q; valid: ecdsa, mldsa44, mldsa65, mldsa87, sphincs\n", f.scheme)
		os.Exit(2)
	}

	return f
}

func getDefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".viri"
	}
	return filepath.Join(home, ".viri")
}

func resolveScheme(s string) crypto.Scheme {
	switch strings.ToLower(s) {
	case "mldsa44":
		return crypto.SchemeMLDSA44
	case "mldsa65":
		return crypto.SchemeMLDSA65
	case "mldsa87":
		return crypto.SchemeMLDSA87
	case "sphincs":
		return crypto.SchemeSPHINCS
	default:
		return crypto.SchemeECDSA
	}
}

func loadKey(flags nodeFlags, cfg *config.Config, log *logging.Logger, scheme crypto.Scheme) *crypto.PrivateKey {
	log.WithField("scheme", scheme.String()).Info("Crypto scheme for key loading")
	crypto.SetDefaultScheme(scheme)
	if scheme != crypto.SchemeECDSA {
		log.Warn(fmt.Sprintf("Scheme %q selected – PQC keys used for signing/verification; key loading uses secp256k1 for node identity", scheme))
	}

	if flags.privKey != "" {
		keyBytes, err := hex.DecodeString(flags.privKey)
		if err != nil {
			log.Fatal(fmt.Sprintf("Invalid private key: %v", err))
		}

		key, err := crypto.PrivateKeyFromBytes(keyBytes)
		if err != nil {
			log.Fatal(fmt.Sprintf("Invalid private key bytes: %v", err))
		}
		return key
	}

	// Try key file from env var or config, in priority order
	keyPaths := []string{
		os.Getenv("VIRI_VALIDATOR_KEY"),
		cfg.Node.ValidatorKey,
	}
	for _, keyPath := range keyPaths {
		if keyPath == "" {
			continue
		}
		if _, err := os.Stat(keyPath); err == nil {
			keyBytes, err := os.ReadFile(keyPath)
			if err != nil {
				log.Fatal(fmt.Sprintf("Failed to read validator key file: %v", err))
			}
			privHex := strings.TrimSpace(string(keyBytes))
			if strings.HasPrefix(privHex, "0x") {
				privHex = privHex[2:]
			}
			rawKey, err := hex.DecodeString(privHex)
			if err != nil {
				log.Fatal(fmt.Sprintf("Invalid private key in file %s: %v", keyPath, err))
			}
			key, err := crypto.PrivateKeyFromBytes(rawKey)
			if err != nil {
				log.Fatal(fmt.Sprintf("Invalid private key bytes from %s: %v", keyPath, err))
			}
			log.WithField("path", keyPath).Info("Validator key loaded from file")
			return key
		} else {
			log.WithField("path", keyPath).WithField("error", err.Error()).Warn("Validator key file not accessible, trying next")
		}
	}

	passphrase := os.Getenv("VIRI_KEY_PASSPHRASE")
	if passphrase == "" {
		fmt.Fprintln(os.Stderr, "ERROR: VIRI_KEY_PASSPHRASE environment variable is required.")
		fmt.Fprintln(os.Stderr, "       Set it to a strong passphrase for your encrypted validator keystore.")
		fmt.Fprintln(os.Stderr, "       Example (generate one):  openssl rand -hex 32")
		fmt.Fprintln(os.Stderr, "       Then export VIRI_KEY_PASSPHRASE=<your-passphrase>")
		os.Exit(2)
	}
	if len(passphrase) < 12 {
		log.Fatal("VIRI_KEY_PASSPHRASE must be at least 12 characters long")
	}
	key, err := crypto.LoadKeyOrGenerate(filepath.Join(flags.dataDir, "node.key"), passphrase)
	if err != nil {
		log.Fatal(fmt.Sprintf("Failed to load/generate key: %v", err))
	}

	log.Info("Validator key ready (encrypted keystore)")
	return key
}

func initDB(flags nodeFlags, log *logging.Logger) state.KVStore {
	badgerDir := filepath.Join(flags.dataDir, "badger")
	store, err := state.NewBadgerStore(badgerDir)
	if err != nil {
		log.Warn(fmt.Sprintf("Failed to open BadgerDB, falling back to in-memory store: %v", err))
		return state.NewMemoryStore()
	}
	log.WithField("path", badgerDir).Info("Persistent storage initialized")
	return store
}

func main() {
	flags := parseFlags()

	// Handle special modes - explorer & faucet run as standalone services
	if flags.explorerMode {
		RunExplorer()
		return
	}
	if flags.faucetMode {
		RunFaucet()
		return
	}

	node, err := NewNodeBuilder(flags).
		LoadConfig().
		SetupLogging().
		InitializeDB().
		InitializeBlockchain().
		SetupExecutionEngine().
		SetupL3Modules().
		SetupNetwork().
		SetupConsensus().
		SetupServers().
		Build()

	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to build node: %v\n", err)
		os.Exit(1)
	}

	node.Start()
	node.Wait()
	node.Stop()
}