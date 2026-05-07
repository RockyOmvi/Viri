package main

import (
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
	"github.com/viri-chain/viri/internal/layer1/state"
	"github.com/viri-chain/viri/internal/layer1/sync"
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
	flag.Uint64Var(&f.chainID, "chain-id", 1, "Chain ID")
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

	flag.Parse()

	// Check env var for TLS auto mode
	if os.Getenv("VIRI_TLS_AUTO") == "true" || os.Getenv("VIRI_TLS_AUTO") == "1" {
		f.tlsAuto = true
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

func loadKey(flags nodeFlags, cfg *config.Config, log *logging.Logger) *crypto.PrivateKey {
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
		if cfg.Readiness.ForceReady {
			passphrase = "viri-dev-key"
			log.Warn("Using default dev passphrase for keystore")
		} else {
			log.Fatal("VIRI_KEY_PASSPHRASE must be set in production mode")
		}
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
		log.Warn(fmt.Sprintf("Failed to open BadgerDB, falling back to memory store: %v", err))
		return state.NewMemoryStore()
	}
	log.WithField("path", badgerDir).Info("Persistent storage initialized")
	return store
}

func main() {
	flags := parseFlags()

	// Handle special modes — explorer & faucet run as standalone services
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

	if err := stateMgr.Initialize(new(big.Int).SetUint64(genesis.InitialSupply)); err != nil {
		log.Fatal(fmt.Sprintf("Failed to initialize state: %v", err))
	}

	// Initialize L2 Execution Engine
	execEngine := execution.NewExecutionEngine()
	log.Info("L2 Execution Engine initialized (transfers, deploys, calls)")

	// Initialize L2 modules
	accountMgr := accounts.NewAccountManager()
	agentMgr := agents.NewAgentManager()
	contractMgr := contracts.NewContractManager()
	gasOracle := gas.NewGasOracle(gas.DefaultGasConfig())
	shieldedPool := privacy.NewShieldedPool()
	mevState := mev.NewMEVState(mev.StandardMode)
	rollupChain := rollups.NewRollupChain("main", rollups.RollupTypeOptimistic, 100)
	zkCircuit := zk.NewShieldedTransferCircuit()
	zkProvingKey := zk.GenerateProvingKey(zkCircuit)
	zkVerifyingKey := zk.GenerateVerifyingKey(zkProvingKey, zkCircuit)
	zkProver := zk.NewProver(zkProvingKey, zkCircuit)
	zkVerifier := zk.NewVerifier(zkVerifyingKey, zkCircuit)
	_ = zkProver
	// Wire modules into the execution engine
	execEngine.SetShieldedPool(shieldedPool)
	execEngine.SetZKVerifier(zkVerifier)

	log.WithField("modules", "accounts,agents,contracts,gas,mev,privacy,rollups,zk").
		Info("L2 modules initialized")

	// Initialize L3 modules
	govDAO := governance.NewGovernanceDAO(24*time.Hour, 1_000_000, 0.1)
	chainBridge := bridge.NewChainBridge(2)
	interopProtocol := interop.NewInteropProtocol()
	intentSolver := intent.NewIntentSolver()
	log.WithField("modules", "governance,bridge,interop,intent").
		Info("L3 modules initialized")

	// Wire gas oracle into block event bus for automatic gas tracking
	// Wire rollups and agents into block producer
	_ = accountMgr
	_ = contractMgr
	_ = govDAO
	_ = chainBridge
	_ = interopProtocol
	_ = intentSolver

	var l3APIServer *api.L3APIServer
	var wsServer *WSServer
	eventBus := events.NewEventBus(1000)
	eventBus.Subscribe(events.EventBlockAdded, func(event events.Event) {
		block := event.Data.(*ledger.Block)
		log.WithField("height", block.Header.Height).Info("New block added")
		if wsServer != nil {
			wsServer.BroadcastBlock(block)
		}
	})

	key := loadKey(flags, cfg, log)

	_, err = stateMgr.CreateAccount(key.PubKey().Address(), state.AccountTypeValidator, big.NewInt(10_000_000))
	if err != nil {
		log.WithField("error", err.Error()).Warn("Validator account creation skipped")
	}

	log.WithField("address", fmt.Sprintf("%x", key.PubKey().Address())).
		WithField("pubkey", key.PubKey().Hex()[:16]+"...").
		Info("Validator key ready")

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
	if flags.noMDNS || runtime.GOOS == "windows" {
		netConfig.EnableMDNS = false
	}

	if flags.bootnodes != "" {
		netConfig.Bootstraps = []string{flags.bootnodes}
	}

	viriNet, err := p2p.NewViriNetwork(netConfig, blockchain, log)
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

	syncConfig := sync.DefaultSyncConfig()
	switch flags.syncMode {
	case "full":
		syncConfig.Mode = sync.FullSync
	case "fast":
		syncConfig.Mode = sync.FastSync
	case "snap":
		syncConfig.Mode = sync.SnapSync
	}

	nodeSyncer := sync.NewSyncer(syncConfig, log)

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

	blockProducer := newChainBlockProducer(blockchain, key, execEngine, stateMgr, gasOracle, mevState, shieldedPool, zkVerifier, rollupChain, agentMgr)

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

	log.WithField("validators", len(validators)).Info("Loaded validators from genesis")
	for _, v := range validators {
		log.WithField("address", fmt.Sprintf("%x", v.Address[:8])).
			WithField("stake", v.Stake).
			WithField("active", v.IsActive).
			Debug("Validator loaded")
	}

	validatorSet := consensus.NewValidatorSet(validators, 1)
	staking := consensus.NewStakingModule(21*24*time.Hour, 0.01)
	staking.Stake(key.PubKey().Address(), key.PubKey().Bytes(), 1000000)

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
		rpcServer = NewRPCServer(flags.rpcPort, blockchain, stateMgr, viriNet, engine, log, cfg.Chain.ChainID, flags.validator, key.PubKey().Address(), tlsCert, tlsKey, cfg.Node.APIKeyHash, obsAuditLog, nodeSyncer)
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
	l3APIServer = api.NewL3APIServer(flags.l3Port, govDAO, chainBridge, interopProtocol, intentSolver)
	if err := l3APIServer.Start(); err != nil {
		log.Error(fmt.Sprintf("Failed to start L3 API server: %v", err))
	}
	log.WithField("port", flags.l3Port).Info("L3 API server started")

	go func() {
		time.Sleep(5 * time.Second)
		viriNet.BroadcastGetPeers()
		log.Info("Initial peer discovery broadcast sent")
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
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
		}
	}()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			metricsCollector.UpdateUptime()
			syncing := nodeSyncer != nil && nodeSyncer.IsSyncing()
			metricsCollector.SetNodeIsSyncing(syncing)
			metricsCollector.SetMempoolPendingTxs(blockchain.TxPool().Size())
			metricsCollector.SetHealthData(blockchain.Height(), viriNet.PeerCount(), syncing)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	log.WithField("rpc_port", flags.rpcPort).
		WithField("api_port", flags.apiPort).
		WithField("p2p_port", flags.p2pPort).
		WithField("rpc_enabled", flags.rpc).
		WithField("api_enabled", flags.api).
		Info("Viri node is running")

	fmt.Println("Press Ctrl+C to stop.")
	<-stop

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

	if flags.validator {
		engine.Stop()
	}

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
