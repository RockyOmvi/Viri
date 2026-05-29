package main

import (
	"context"
	crand "crypto/rand"
	csha256 "crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/viri-chain/viri/internal/layer1/config"
	"github.com/viri-chain/viri/internal/layer1/consensus"
	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/events"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
	"github.com/viri-chain/viri/internal/layer1/p2p"
	"github.com/viri-chain/viri/internal/layer1/sequencer"
	"github.com/viri-chain/viri/internal/layer1/state"
	nodesync "github.com/viri-chain/viri/internal/layer1/sync"
	"github.com/viri-chain/viri/internal/layer2/accounts"
	"github.com/viri-chain/viri/internal/layer2/agents"
	"github.com/viri-chain/viri/internal/layer2/contracts"
	"github.com/viri-chain/viri/internal/layer2/execution"
	"github.com/viri-chain/viri/internal/layer2/gas"
	"github.com/viri-chain/viri/internal/layer2/mev"
	"github.com/viri-chain/viri/internal/layer2/privacy"
	"github.com/viri-chain/viri/internal/layer2/rollups"
	"github.com/viri-chain/viri/internal/layer2/zk"
	"github.com/viri-chain/viri/internal/layer3/api"
	"github.com/viri-chain/viri/internal/layer3/appchain"
	"github.com/viri-chain/viri/internal/layer3/bridge"
	"github.com/viri-chain/viri/internal/layer3/governance"
	"github.com/viri-chain/viri/internal/layer3/intent"
	"github.com/viri-chain/viri/internal/layer3/interop"
	"github.com/viri-chain/viri/internal/pkg/audit"
	"github.com/viri-chain/viri/internal/pkg/metrics"
	"github.com/viri-chain/viri/internal/pkg/observability"
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

	flag.StringVar(&f.dataDir, "data-dir", getDefaultDataDir(), "Data directory for node storage")
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

	fmt.Printf("Viri Daemon v%s\n", Version)
	fmt.Printf("Node: %s | Data: %s | Validator: %v\n", flags.name, flags.dataDir, flags.validator)
	fmt.Println("Initializing...")

	// If --testnet flag is set, default to testnet config
	if flags.testnet {
		if flags.config == "" {
			flags.config = "configs/node-testnet.json"
		}
		if flags.chainID == 0 {
			flags.chainID = 2
		}
	}

	cfg, err := config.LoadConfigOrDefault(flags.config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if flags.genesis != "" {
		cfg.Chain.GenesisFile = flags.genesis
	}
	if flags.chainID != 0 {
		cfg.Chain.ChainID = flags.chainID
	}

	cfg.Node.RPCPort = flags.rpcPort
	cfg.Node.APIPort = flags.apiPort
	cfg.Node.ValidatorMode = flags.validator
	cfg.Node.RPCEnabled = flags.rpc
	cfg.Node.APIEnabled = flags.api
	cfg.Logging.Level = flags.logLevel
	cfg.Node.Name = flags.name
	if flags.dataDir != "" {
		cfg.Node.DataDir = flags.dataDir
	}

	cfg.ApplyEnvOverrides()

	observability.ConfigureReadiness(cfg.Readiness.MinPeers, cfg.Readiness.MinBlockHeight)
	observability.ForceReady(cfg.Readiness.ForceReady)

	log := logging.NewLogger("virid", logging.ParseLogLevel(cfg.Logging.Level), cfg.Logging.Level)

	// Set up file-based log rotation if output path is configured
	if cfg.Logging.Output != "" && cfg.Logging.Output != "stdout" {
		maxSize := cfg.Logging.MaxSize
		if maxSize <= 0 {
			maxSize = 100
		}
		maxBackups := cfg.Logging.MaxBackups
		if maxBackups <= 0 {
			maxBackups = 7
		}
		rotatingWriter, err := logging.NewRotatingFileWriter(cfg.Logging.Output, "virid", maxSize, maxBackups)
		if err != nil {
			log.WithField("error", err.Error()).Warn("Failed to set up log rotation, using stdout")
		} else {
			log.SetOutput(rotatingWriter)
			log.WithField("path", cfg.Logging.Output).
				WithField("max_size_mb", maxSize).
				WithField("max_backups", maxBackups).
				Info("File-based log rotation enabled")
			defer rotatingWriter.Close()
		}
	}

	log.WithField("chain_id", cfg.Chain.ChainID).
		WithField("network", cfg.Chain.NetworkName).
		WithField("validator", flags.validator).
		WithField("name", cfg.Node.Name).
		Info("Starting Viri node")

	if err := cfg.Validate(); err != nil {
		log.Fatal(fmt.Sprintf("Invalid configuration: %v", err))
	}

	if err := os.MkdirAll(cfg.Node.DataDir, 0700); err != nil {
		log.Fatal(fmt.Sprintf("Failed to create data directory: %v", err))
	}

	flags.dataDir = cfg.Node.DataDir

	version, err := state.CheckSchemaVersion(flags.dataDir)
	if err != nil {
		log.WithField("error", err.Error()).Warn("Could not check schema version")
	}
	if version != state.CurrentSchemaVersion {
		log.WithField("current", version).
			WithField("expected", state.CurrentSchemaVersion).
			Info("Database schema migration required")
	}

	db := initDB(flags, log)

	if err := state.RunMigrations(db, flags.dataDir); err != nil {
		log.Fatal(fmt.Sprintf("Failed to run migrations: %v", err))
	}
	stateMgr, err := state.NewStateManager(db)
	if err != nil {
		log.Fatal(fmt.Sprintf("Failed to initialize state manager: %v", err))
	}

	var genesis *ledger.GenesisConfig
	if cfg.Chain.GenesisFile != "" {
		genesis, err = ledger.LoadGenesis(cfg.Chain.GenesisFile)
		if err != nil {
			log.Fatal(fmt.Sprintf("Failed to load genesis file: %v", err))
		}
	} else {
		genesis = ledger.DefaultGenesis()
		genesis.ChainID = cfg.Chain.ChainID
	}

	if err := genesis.ValidateAndSanitize(); err != nil {
		log.Fatal(fmt.Sprintf("Invalid genesis configuration: %v", err))
	}

	log.WithField("chain_id", genesis.ChainID).
		WithField("supply", genesis.InitialSupply).
		Info("Genesis configuration validated")

	blockchain, err := ledger.NewPersistentBlockchain(genesis, db)
	if err != nil {
		log.Fatal(fmt.Sprintf("Failed to initialize blockchain: %v", err))
	}

	mempoolPersist := ledger.NewMempoolPersister(cfg.Node.DataDir)
	pendingTxs, err := mempoolPersist.Load()
	if err != nil {
		log.WithField("error", err.Error()).Warn("Failed to load mempool from disk")
	}
	for _, tx := range pendingTxs {
		if tx.Verify() {
			blockchain.TxPool().Add(tx)
		}
	}
	log.WithField("restored", len(pendingTxs)).Info("Mempool restored from disk")

	log.WithField("height", blockchain.Height()).
		WithField("tip", fmt.Sprintf("%x...", blockchain.TipHash()[:8])).
		Info("Blockchain initialized")

	if stateMgr.IsInitialized() {
		log.WithField("height", stateMgr.BlockHeight()).
			WithField("supply", stateMgr.TotalSupply()).
			Info("State already initialized, loading from DB")
	} else {
		log.Info("Fresh state, initializing from genesis")
		if err := stateMgr.Initialize(new(big.Int).SetUint64(genesis.InitialSupply)); err != nil {
			log.Fatal(fmt.Sprintf("Failed to initialize state: %v", err))
		}

		// Create state accounts for genesis validators only on fresh init
		for _, gv := range genesis.InitialValidators {
			if _, err := stateMgr.CreateAccount(gv.Address, state.AccountTypeValidator, new(big.Int).SetUint64(gv.Stake)); err != nil {
				log.WithField("address", fmt.Sprintf("%x", gv.Address)).
					WithField("error", err.Error()).
					Warn("Genesis validator account creation skipped")
			} else {
				log.WithField("address", fmt.Sprintf("%x", gv.Address)).
					WithField("balance", gv.Stake).
					Info("Genesis validator account created")
			}
		}
	}

	// Initialize L2 Execution Engine
	execEngine := execution.NewExecutionEngine()
	if flags.parallelExec {
		execEngine.SetParallel(true)
	}
	log.WithField("vm", "evm").
		WithField("parallel", flags.parallelExec).
		Info("L2 Execution Engine initialized (transfers, deploys, calls)")

	// Initialize L2 modules
	accountMgr := accounts.NewAccountManager()
	agentMgr := agents.NewAgentManager()
	contractMgr := contracts.NewContractManager()
	gasOracle := gas.NewGasOracle(gas.DefaultGasConfig())
	shieldedPool := privacy.NewShieldedPool()
	mevModeVal := mev.StandardMode
	switch flags.mevMode {
	case "encrypted":
		mevModeVal = mev.EncryptedMode
	case "commit-reveal":
		mevModeVal = mev.CommitReveal
	}
	mevState := mev.NewMEVState(mevModeVal)
	log.WithField("mode", flags.mevMode).Info("MEV resistance module initialized")
	rollupChain := rollups.NewRollupChain("main", rollups.RollupTypeOptimistic, 100)
	zkCircuit := zk.NewShieldedTransferCircuit()
	gv := zk.NewGnarkVerifier()
	execEngine.SetGnarkVerifier(gv, zkCircuit)
	log.Info("Gnark-based ZK verifier enabled")

	// Wire modules into the execution engine
	execEngine.SetShieldedPool(shieldedPool)
	execEngine.SetContractManager(contractMgr)

	// Initialize FeeConversionOracle for gas-in-any-token
	feeOracle := gas.NewFeeConversionOracle(5 * time.Minute)
	for tokenKey, rate := range gas.DefaultConversionRates() {
		feeOracle.SetRate([]byte(tokenKey), rate)
	}
	feeOracle.SetRate(contracts.AddrERC20, 1.0) // 1 VIRI token = 1 native VIRI
	execEngine.SetFeeOracle(feeOracle)
	log.WithField("tokens", len(feeOracle.KnownTokens())).Info("Fee Conversion Oracle initialized")

	log.WithField("modules", "accounts,agents,contracts,gas,feeOracle,mev,privacy,rollups,zk").
		Info("L2 modules initialized")

	// Initialize L3 modules
	govDAO := governance.NewGovernanceDAO(24*time.Hour, 1_000_000, 0.1)
	chainBridge := bridge.NewChainBridge(2)
	interopProtocol := interop.NewInteropProtocol()
	intentSolver := intent.NewIntentSolver()
	appChainMgr := appchain.NewAppChainManager()

	// Initialize PrivacyBridge with ZK circuit keys
	privPk := zk.GenerateProvingKey(zkCircuit)
	privVk := zk.GenerateVerifyingKey(privPk, zkCircuit)
	privacyBridge := bridge.NewPrivacyBridge(2, zkCircuit, privVk, privPk)
	privacyBridge.RegisterChain("viri-main", "Viri Main Chain", "http://localhost:8545")
	log.WithField("modules", "governance,bridge,interop,intent,appchain,privacy_bridge").
		Info("L3 modules initialized")

	// Wire gas oracle into block event bus for automatic gas tracking
	// Wire rollups and agents into block producer
	var l3APIServer *api.L3APIServer
	var wsServer *WSServer
	eventBus := events.NewEventBus()
	eventBus.Subscribe(events.EventBlockAdded, func(event events.Event) {
		block := event.Data.(*ledger.Block)
		log.WithField("height", block.Header.Height).Info("New block added")
		if wsServer != nil {
			wsServer.BroadcastBlock(block)
		}
	})

	key := loadKey(flags, cfg, log, resolveScheme(flags.scheme))

	_, err = stateMgr.CreateAccount(key.PubKey().Address(), state.AccountTypeValidator, big.NewInt(10_000_000))
	if err != nil {
		log.WithField("error", err.Error()).Warn("Validator account creation skipped")
	}

	log.WithField("address", fmt.Sprintf("%x", key.PubKey().Address())).
		WithField("pubkey", key.PubKey().Hex()[:16]+"...").
		Info("Validator key ready")

	// Deploy standard ERC-20 with 1M supply to the validator address
	validatorAddr := key.PubKey().Address()
	viriToken := contracts.NewERC20Token("VIRI Token", "VIRI", 18, new(big.Int).Mul(big.NewInt(1_000_000), big.NewInt(1e18)), validatorAddr)
	contractMgr.RegisterStandardContract(contracts.AddrERC20, viriToken)
	log.WithField("address", fmt.Sprintf("%x", contracts.AddrERC20)).
		WithField("supply", "1000000000000000000000000").
		Info("Standard ERC-20 token deployed")

	// Deploy standard ERC-721 NFT collection
	nftToken := contracts.NewERC721Token("VIRI NFT", "VNFT", "https://viri-chain.io/nft/")
	contractMgr.RegisterStandardContract(contracts.AddrERC721, nftToken)
	nftToken.Mint(validatorAddr, 1, "genesis-viri-1")
	nftToken.Mint(validatorAddr, 2, "genesis-viri-2")
	nftToken.Mint(validatorAddr, 3, "genesis-viri-3")
	log.WithField("address", fmt.Sprintf("%x", contracts.AddrERC721)).
		WithField("minted", 3).
		Info("Standard ERC-721 NFT collection deployed")

	// Wire the sequencer for decentralized transaction ordering
	seqConfig := sequencer.DefaultSequencerConfig()
	seqConfig.ProposerKey = key
	seq := sequencer.NewSequencer(seqConfig, blockchain)
	if err := seq.Start(); err != nil {
		log.Warn(fmt.Sprintf("Sequencer start skipped: %v", err))
	} else {
		log.WithField("batch_size", seqConfig.BatchSize).WithField("timeout", seqConfig.BatchTimeout).Info("Sequencer started")
	}

	// Register validator as an agent
	if err := agentMgr.Register("validator-0", agents.AgentTypeValidator, key.PubKey().Address(), 10_000_000); err != nil {
		log.WithField("error", err.Error()).Warn("Agent registration skipped")
	}

	// Wire interop handlers (default: echo handler for all ports)
	interopProtocol.RegisterHandler("default", func(packet *interop.IBCPacket) ([]byte, error) {
		log.WithField("channel", packet.SourceChain+"->"+packet.DestChain).
			WithField("sequence", packet.Sequence).
			Info("Interop packet received")
		return packet.Data, nil
	})

	txPool := blockchain.TxPool()
	log.WithField("pending", len(txPool.GetPending())).
		Info("Transaction pool ready")

	economics := blockchain.Economics()
	log.WithField("circulating", economics.CirculatingSupply().String()).
		Info("Economics module initialized")

	snapshot := stateMgr.Snapshot()
	log.WithField("block_height", snapshot.BlockHeight).
		WithField("accounts", snapshot.NumAccounts).
		Info("State snapshot")

	netConfig := p2p.DefaultNetworkConfig()
	netConfig.ChainID = cfg.Chain.ChainID
	netConfig.ListenAddr = fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", flags.p2pPort)
	if cfg.Network.ExternalAddr != "" {
		netConfig.ExternalAddr = cfg.Network.ExternalAddr
	}
	if flags.noMDNS || runtime.GOOS == "windows" {
		netConfig.EnableMDNS = false
	}

	if flags.bootnodes != "" {
		for _, addr := range strings.Split(flags.bootnodes, ",") {
			addr = strings.TrimSpace(addr)
			if addr != "" {
				netConfig.Bootstraps = append(netConfig.Bootstraps, addr)
			}
		}
	} else if len(cfg.Network.BootstrapPeers) > 0 {
		netConfig.Bootstraps = cfg.Network.BootstrapPeers
	}

	var p2pPrivKey *crypto.PrivateKey
	if flags.p2pKey != "" {
		keyBytes, err := hex.DecodeString(flags.p2pKey)
		if err == nil {
			p2pPrivKey, _ = crypto.PrivateKeyFromBytes(keyBytes)
		}
	} else if os.Getenv("VIRI_P2P_KEY") != "" {
		keyBytes, err := hex.DecodeString(os.Getenv("VIRI_P2P_KEY"))
		if err == nil {
			p2pPrivKey, _ = crypto.PrivateKeyFromBytes(keyBytes)
		}
	} else if os.Getenv("VIRI_P2P_KEY_FILE") != "" {
		keyBytes, err := os.ReadFile(os.Getenv("VIRI_P2P_KEY_FILE"))
		if err == nil {
			privHex := strings.TrimSpace(string(keyBytes))
			if strings.HasPrefix(privHex, "0x") {
				privHex = privHex[2:]
			}
			rawKey, err := hex.DecodeString(privHex)
			if err == nil {
				p2pPrivKey, _ = crypto.PrivateKeyFromBytes(rawKey)
			}
		}
	}

	viriNet, err := p2p.NewViriNetwork(netConfig, blockchain, log, p2pPrivKey)
	if err != nil {
		log.Fatal(fmt.Sprintf("Failed to create network: %v", err))
	}

	viriNet.SetValidatorAddress(key.PubKey().Address())
	viriNet.SetValidatorPubKey(key.PubKey().Bytes())

	if err := viriNet.Start(); err != nil {
		log.Fatal(fmt.Sprintf("Failed to start network: %v", err))
	}

	if flags.writePeerFile != "" {
		if err := viriNet.WritePeerInfo(flags.writePeerFile); err != nil {
			log.Warn(fmt.Sprintf("Failed to write peer info: %v", err))
		} else {
			log.Info(fmt.Sprintf("Peer info written to %s", flags.writePeerFile))
		}
	}

	if flags.peerFile != "" {
		go func() {
			for {
				if err := viriNet.ReadAndConnectPeerInfo(flags.peerFile); err != nil {
					time.Sleep(2 * time.Second)
					continue
				}
				break
			}
		}()
	}

	syncConfig := nodesync.DefaultSyncConfig()
	switch flags.syncMode {
	case "full":
		syncConfig.Mode = nodesync.FullSync
	case "fast":
		syncConfig.Mode = nodesync.FastSync
	case "snap":
		syncConfig.Mode = nodesync.SnapSync
	}

	nodeSyncer := nodesync.NewSyncer(syncConfig, log)

	if blockchain.Height() == 0 && flags.bootnodes != "" {
		log.WithField("mode", flags.syncMode).Info("Starting node sync from genesis")
	}

	var engineRef *consensus.HotStuffEngine

	viriNet.SetMessageHandler(&p2p.SimpleMessageHandler{
		OnBlockHandler: func(msg *p2p.Message, from peer.ID) error {
			log.WithField("peer", from.String()).
				WithField("size", len(msg.Payload)).
				Info("Received block from peer")
			if engineRef != nil && engineRef.GetStateSyncer() != nil && engineRef.GetStateSyncer().IsSyncing() {
				if err := engineRef.GetStateSyncer().ReceiveBlock(msg.Payload); err != nil {
					log.WithField("error", err.Error()).Warn("Failed to receive sync block")
				}
			}
			return nil
		},
		OnTransactionHandler: func(msg *p2p.Message, from peer.ID) error {
			log.WithField("peer", from.String()).
				WithField("size", len(msg.Payload)).
				Info("Received transaction from peer")
			tx, err := ledger.DeserializeTransaction(msg.Payload)
			if err != nil {
				log.WithField("error", err.Error()).Warn("Failed to deserialize received transaction")
				return nil
			}
			if !tx.Verify() {
				log.Warn("Received invalid transaction from peer")
				return nil
			}
			txPool := blockchain.TxPool()
			if err := txPool.Add(tx); err != nil {
				log.WithField("error", err.Error()).Debug("Received transaction not added to pool")
			}
			return nil
		},
		OnGetBlocksHandler: func(msg *p2p.Message, from peer.ID) error {
			log.WithField("peer", from.String()).Info("Peer requested blocks")
			return nil
		},
		OnGetHeadersHandler: func(msg *p2p.Message, from peer.ID) error {
			log.WithField("peer", from.String()).Info("Peer requested headers")
			return nil
		},
		OnAnnounceHandler: func(msg *p2p.Message, from peer.ID) error {
			log.WithField("peer", from.String()).Info("Peer announced new data")
			return nil
		},
	})

	if err := viriNet.SubscribeToBlocks(func(msg *p2p.Message, from peer.ID) {
		log.Debug(fmt.Sprintf("Block subscription: peer=%s size=%d", from.String(), len(msg.Payload)))
		if engineRef != nil && engineRef.GetStateSyncer() != nil && engineRef.GetStateSyncer().IsSyncing() {
			if err := engineRef.GetStateSyncer().ReceiveBlock(msg.Payload); err != nil {
				log.Debug(fmt.Sprintf("Sync block receive failed: %v", err))
			}
		}
	}); err != nil {
		log.Error(fmt.Sprintf("Failed to subscribe to blocks: %v", err))
	}

	if err := viriNet.SubscribeToTransactions(func(msg *p2p.Message, from peer.ID) {
		log.Debug(fmt.Sprintf("Transaction subscription: peer=%s size=%d", from.String(), len(msg.Payload)))
	}); err != nil {
		log.Error(fmt.Sprintf("Failed to subscribe to transactions: %v", err))
	}

	if err := viriNet.SubscribeToHeaders(func(msg *p2p.Message, from peer.ID) {
		log.Debug(fmt.Sprintf("Header subscription: peer=%s size=%d", from.String(), len(msg.Payload)))
	}); err != nil {
		log.Error(fmt.Sprintf("Failed to subscribe to headers: %v", err))
	}

	log.WithField("peer_id", viriNet.ShortPeerID()).
		WithField("addresses", viriNet.Addresses()).
		Info("P2P network initialized")

	blockProducer := newChainBlockProducer(blockchain, key, execEngine, stateMgr, gasOracle, mevState, shieldedPool, rollupChain, agentMgr)

	validators := make([]*consensus.Validator, 0, len(genesis.InitialValidators))
	for _, gv := range genesis.InitialValidators {
		validators = append(validators, &consensus.Validator{
			Address:  gv.Address,
			PublicKey: gv.PublicKey,
			Stake:    gv.Stake,
			IsActive: true,
		})
	}

	if len(validators) == 0 {
		validators = append(validators, &consensus.Validator{
			Address:  key.PubKey().Address(),
			PublicKey: key.PubKey().Bytes(),
			Stake:    1000000,
			IsActive: true,
		})
	}

	// Load extra validators from config
	for _, ev := range cfg.Consensus.ExtraValidators {
		addr, err := hex.DecodeString(strings.TrimPrefix(ev.Address, "0x"))
		if err != nil {
			log.WithField("address", ev.Address).Warn("Invalid extra validator address, skipping")
			continue
		}
		pubBytes, err := hex.DecodeString(strings.TrimPrefix(ev.PublicKey, "0x"))
		if err != nil {
			log.WithField("address", ev.Address).Warn("Invalid extra validator public key, skipping")
			continue
		}
		stake := ev.Stake
		if stake == 0 {
			stake = 1000000
		}
		validators = append(validators, &consensus.Validator{
			Address:   addr,
			PublicKey: pubBytes,
			Stake:     stake,
			IsActive:  true,
		})
		log.WithField("address", fmt.Sprintf("%x", addr)).
			WithField("stake", stake).
			Info("Extra validator added from config")
	}

	log.WithField("validators", len(validators)).Info("Loaded validators from genesis")
	for _, v := range validators {
		log.WithField("address", fmt.Sprintf("%x", v.Address[:8])).
			WithField("stake", v.Stake).
			WithField("active", v.IsActive).
			Debug("Validator loaded")
	}

	validatorSet := consensus.NewValidatorSet(validators, 1)
	staking := consensus.NewStakingModule(21*24*time.Hour, 0.01)
	for _, v := range validators {
		if err := staking.Stake(v.Address, v.PublicKey, v.Stake); err != nil {
			log.WithField("address", fmt.Sprintf("%x", v.Address)).
				WithField("error", err.Error()).
				Warn("Failed to stake genesis validator")
		}
	}

	consensusConfig := consensus.DefaultConsensusConfig()
	consensusConfig.BlockTime = cfg.Chain.BlockTime.Duration()
	consensusConfig.ViewTimeout = 5 * time.Second
	consensusConfig.MaxViewTimeout = 15 * time.Second
	consensusConfig.TimeoutIncrease = 2 * time.Second
	consensusConfig.EpochLength = 1000
	if len(validators) > 1 {
		consensusConfig.MinValidators = len(validators)
	} else if flags.validator {
		consensusConfig.MinValidators = 1
	}

	auditConfig := audit.DefaultAuditConfig()
	auditConfig.OutputPath = filepath.Join(flags.dataDir, "audit")
	auditLogger, err := audit.NewAuditLogger(auditConfig)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to create audit logger: %v", err))
	}
	engine := consensus.NewHotStuffEngine(consensusConfig, validatorSet, blockProducer, staking, log, auditLogger)

	// Wire block rewards to actual on-chain balances
	var econConfig = ledger.DefaultEconomicsConfig()
	rewardEcon := ledger.NewEconomics(econConfig)
	engine.BlockRewardFn = func(height uint64, proposer []byte, _ *big.Int) {
		reward := rewardEcon.CalculateBlockReward(height)
		if reward.Sign() > 0 {
			if err := stateMgr.MintTokens(proposer, reward); err != nil {
				log.WithField("height", height).
					WithField("proposer", fmt.Sprintf("%x", proposer)).
					WithField("reward", reward.String()).
					Error(fmt.Sprintf("Failed to mint block reward: %v", err))
			} else {
				log.WithField("height", height).
					WithField("proposer", fmt.Sprintf("%x", proposer)).
					WithField("reward", reward.String()).
					Info("Block reward minted")
			}
		}
	}

	metricsCollector := metrics.NewMetricsCollector()
	engine.SetMetrics(metricsCollector)
	viriNet.SetMetrics(metricsCollector)

	metricsCollector.StartMetricsServer(9090)

	engine.SetBroadcast(func(msg *consensus.ConsensusMessage) {
		data, err := json.Marshal(msg)
		if err != nil {
			log.WithField("error", err.Error()).Warn("Failed to marshal consensus message")
			return
		}
		if err := viriNet.PublishConsensus(data); err != nil {
			log.WithField("error", err.Error()).Warn("Failed to publish consensus message")
		}
	})

	if err := viriNet.SubscribeToConsensus(func(msg *p2p.Message, from peer.ID) {
		var consensusMsg consensus.ConsensusMessage
		if err := json.Unmarshal(msg.Payload, &consensusMsg); err != nil {
			log.WithField("error", err.Error()).Warn("Failed to unmarshal consensus message")
			return
		}
		engine.HandleMessage(&consensusMsg)
	}); err != nil {
		log.Error(fmt.Sprintf("Failed to subscribe to consensus: %v", err))
	}

	obsAuditLog, err := observability.NewAuditLogger(filepath.Join(flags.dataDir, "logs"), 100, 3)
	if err != nil {
		log.WithField("error", err.Error()).Warn("Observability audit logging disabled")
	}

	defer func() {
		if auditLogger != nil {
			auditLogger.Close()
		}
		if obsAuditLog != nil {
			obsAuditLog.Close()
		}
	}()

	// Emergency shutdown handler for DoS protection
	var shutdownOnce sync.Once
	doEmergencyShutdown := func() {
		shutdownOnce.Do(func() {
			log.Warn("!!!! EMERGENCY SHUTDOWN TRIGGERED !!!!")
			if flags.validator {
				engine.Stop()
			}
			viriNet.Close()
			db.Close()
			stateMgr.Close()
			os.Exit(1)
		})
	}

	// Register DoS protector with emergency shutdown
	if viriNet.GetDoSProtector() != nil {
		viriNet.GetDoSProtector().SetEmergencyHandler(doEmergencyShutdown)
	}

	if flags.validator {
		if flags.consensusDelay > 0 {
			log.Info(fmt.Sprintf("Waiting %s for peer discovery before starting consensus", flags.consensusDelay))
			time.Sleep(flags.consensusDelay)
			log.WithField("validators", validatorSet.Size()).Info("Starting consensus after peer discovery delay")
			for _, v := range validatorSet.GetValidators() {
				log.Debug(fmt.Sprintf("Validator address=%x stake=%d", v.Address, v.Stake))
			}
		}
		if err := engine.Start(blockchain.Height() + 1); err != nil {
			log.Error(fmt.Sprintf("Failed to start consensus engine: %v", err))
		}

		engine.OnSyncComplete(func() {
			log.WithField("height", blockchain.Height()).Info("State sync completed, resuming consensus")
		})
	}

	log.WithField("height", blockchain.Height()).
		WithField("validators", validatorSet.Size()).
		WithField("epoch", validatorSet.Epoch()).
		WithField("consensus", flags.validator).
		Info("Consensus engine initialized")

	var rpcServer *RPCServer
	var apiServer *APIServer

	// Auto-generate API key if not configured
	if cfg.Node.APIKeyHash == "" && (flags.rpc || flags.api) {
		apiKeyPath := filepath.Join(flags.dataDir, "api_key.txt")
		var rawKey string
		if data, err := os.ReadFile(apiKeyPath); err == nil && len(data) > 0 {
			rawKey = string(data)
		} else {
			keyBytes := make([]byte, 32)
			crand.Read(keyBytes)
			rawKey = hex.EncodeToString(keyBytes)
			os.WriteFile(apiKeyPath, []byte(rawKey), 0600)
		}
		h := csha256.Sum256([]byte(rawKey))
		cfg.Node.APIKeyHash = hex.EncodeToString(h[:])
		log.WithField("hint", fmt.Sprintf("%s...%s", rawKey[:8], rawKey[len(rawKey)-4:])).Warn("Generated API key hash — save this value as it will not be shown again")
	}

	// Auto-generate TLS certs if requested
	tlsCert := cfg.Node.TLSCertPath
	tlsKey := cfg.Node.TLSKeyPath
	if flags.tlsAuto && (tlsCert == "" || tlsKey == "") {
		log.Info("Auto-generating self-signed TLS certificates...")
		genCert, genKey, err := EnsureTLSCerts(flags.dataDir)
		if err != nil {
			log.Error(fmt.Sprintf("TLS auto-generation failed: %v", err))
		} else {
			tlsCert = genCert
			tlsKey = genKey
			log.WithField("cert", genCert).WithField("key", genKey).Info("TLS certificates ready")
		}
	}

	if flags.rpc {
		entryPoint := accounts.NewEntryPoint(accountMgr, cfg.Chain.ChainID, nil)
		rpcServer = NewRPCServer(flags.rpcPort, blockchain, stateMgr, viriNet, engine, log, cfg.Chain.ChainID, flags.validator, key.PubKey().Address(), tlsCert, tlsKey, cfg.Node.APIKeyHash, obsAuditLog, nodeSyncer, entryPoint, contractMgr, shieldedPool, mevState, rollupChain)
		if err := rpcServer.Start(); err != nil {
			log.Error(fmt.Sprintf("Failed to start RPC server: %v", err))
		}
	}

	wsPort := flags.rpcPort + 2
	wsServer = NewWSServer(wsPort, blockchain, viriNet, log, tlsCert, tlsKey, cfg.Node.APIKeyHash)
	if err := wsServer.Start(); err != nil {
		log.Error(fmt.Sprintf("Failed to start WebSocket server: %v", err))
	}

	if flags.api {
		apiServer = NewAPIServer(flags.apiPort, blockchain, stateMgr, viriNet, log, tlsCert, tlsKey, cfg.Node.APIKeyHash)
		if err := apiServer.Start(); err != nil {
			log.Error(fmt.Sprintf("Failed to start API server: %v", err))
		}
	}

	// Start L3 API server
	l3APIServer = api.NewL3APIServer(flags.l3Port, govDAO, chainBridge, interopProtocol, intentSolver, appChainMgr, agentMgr)
	if err := l3APIServer.Start(); err != nil {
		log.Error(fmt.Sprintf("Failed to start L3 API server: %v", err))
	}
	log.WithField("port", flags.l3Port).Info("L3 API server started")

	// Start Admin API server
	adminPort := flags.rpcPort + 4
	adminServer := NewAdminServer(adminPort, blockchain, stateMgr, viriNet, engine, log, cfg.Node.APIKeyHash)
	if err := adminServer.Start(); err != nil {
		log.Error(fmt.Sprintf("Failed to start Admin API server: %v", err))
	}
	log.WithField("port", adminPort).Info("Admin API server started")

	stopCh := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		time.Sleep(5 * time.Second)
		viriNet.BroadcastGetPeers()
		log.Info("Initial peer discovery broadcast sent")
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stats := viriNet.Stats().Snapshot()
				connStats := viriNet.ConnManager().Stats()
				log.WithField("peers", stats.CurrentPeers).
					WithField("blocks_in", stats.TotalBlocksIn).
					WithField("blocks_out", stats.TotalBlocksOut).
					WithField("txs_in", stats.TotalTxsIn).
					WithField("txs_out", stats.TotalTxsOut).
					WithField("bytes_in", stats.TotalBytesIn).
					WithField("bytes_out", stats.TotalBytesOut).
					WithField("rejected", stats.RejectedMessages).
					WithField("conn_active", connStats.ActivePeers).
					WithField("uptime", stats.Uptime.String()).
					Info("Network stats")
			case <-stopCh:
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				metricsCollector.UpdateUptime()
				syncing := nodeSyncer != nil && nodeSyncer.IsSyncing()
				mempoolSize := blockchain.TxPool().Size()
				metricsCollector.SetNodeIsSyncing(syncing)
				metricsCollector.SetMempoolPendingTxs(mempoolSize)
				metricsCollector.SetHealthData(blockchain.Height(), viriNet.PeerCount(), syncing)

				if mempoolSize > 0 && txPool != nil {
					pressure := txPool.PressureLevel()
					if pressure > 0.9 {
						log.WithField("pressure", pressure).
							WithField("pending", mempoolSize).
							Warn("Mempool under high pressure")
					}
				}
			case <-stopCh:
				return
			}
		}
	}()

	log.WithField("rpc_port", flags.rpcPort).
		WithField("api_port", flags.apiPort).
		WithField("p2p_port", flags.p2pPort).
		WithField("rpc_enabled", flags.rpc).
		WithField("api_enabled", flags.api).
		Info("Viri node is running")

	fmt.Println("Press Ctrl+C to stop.")
	select {
	case <-sigCh:
	case <-stopCh:
	}

	log.Info("Shutting down gracefully...")

	if nodeSyncer != nil {
		nodeSyncer.Stop()
	}

	if mempoolPersist != nil {
		if err := mempoolPersist.Save(blockchain.TxPool()); err != nil {
			log.WithField("error", err.Error()).Warn("Failed to save mempool to disk")
		} else {
			log.WithField("pending", blockchain.TxPool().Size()).Info("Mempool saved to disk")
		}
	}

	log.Info("Draining connections...")

	if wsServer != nil {
		if err := wsServer.Stop(); err != nil {
			log.Error(fmt.Sprintf("Error stopping WebSocket server: %v", err))
		}
	}

	if rpcServer != nil {
		if err := rpcServer.Stop(); err != nil {
			log.Error(fmt.Sprintf("Error stopping RPC server: %v", err))
		}
	}

	if apiServer != nil {
		if err := apiServer.Stop(); err != nil {
			log.Error(fmt.Sprintf("Error stopping API server: %v", err))
		}
	}

	if l3APIServer != nil {
		if err := l3APIServer.Stop(); err != nil {
			log.Error(fmt.Sprintf("Error stopping L3 API server: %v", err))
		}
	}

	if adminServer != nil {
		if err := adminServer.Stop(); err != nil {
			log.Error(fmt.Sprintf("Error stopping Admin API server: %v", err))
		}
	}

	if flags.validator {
		engine.Stop()
	}

	drainCtx, drainCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := viriNet.Drain(drainCtx); err != nil {
		log.Warn(fmt.Sprintf("P2P drain warning: %v", err))
	}
	drainCancel()

	if err := viriNet.Close(); err != nil {
		log.Error(fmt.Sprintf("Error closing network: %v", err))
	}

	if err := db.Close(); err != nil {
		log.Error(fmt.Sprintf("Error closing database: %v", err))
	}

	if err := stateMgr.Close(); err != nil {
		log.Error(fmt.Sprintf("Error closing state manager: %v", err))
	}

	log.Info("Viri node stopped")
}