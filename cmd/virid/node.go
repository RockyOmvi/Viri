package main

import (
	"context"
	crand "crypto/rand"
	csha256 "crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

type Node struct {
	flags            nodeFlags
	cfg              *config.Config
	log              *logging.Logger
	db               state.KVStore
	stateMgr         *state.StateManager
	blockchain       *ledger.PersistentBlockchain
	mempoolPersist   *ledger.MempoolPersister
	execEngine       *execution.ExecutionEngine
	viriNet          *p2p.ViriNetwork
	nodeSyncer       *nodesync.Syncer
	consensusEngine  *consensus.HotStuffEngine
	validatorKey     *crypto.PrivateKey
	validatorSet     *consensus.ValidatorSet
	rpcServer        *RPCServer
	wsServer         *WSServer
	apiServer        *APIServer
	l3APIServer      *api.L3APIServer
	adminServer      *AdminServer
	metricsCollector *metrics.MetricsCollector
	obsAuditLog      *observability.AuditLogger
	auditLogger      *audit.AuditLogger
	stopCh           chan struct{}
	shutdownOnce     sync.Once
}

type NodeBuilder struct {
	flags            nodeFlags
	cfg              *config.Config
	log              *logging.Logger
	db               state.KVStore
	stateMgr         *state.StateManager
	blockchain       *ledger.PersistentBlockchain
	mempoolPersist   *ledger.MempoolPersister
	execEngine       *execution.ExecutionEngine
	viriNet          *p2p.ViriNetwork
	nodeSyncer       *nodesync.Syncer
	consensusEngine  *consensus.HotStuffEngine
	validatorKey     *crypto.PrivateKey
	validatorSet     *consensus.ValidatorSet
	rpcServer        *RPCServer
	wsServer         *WSServer
	apiServer        *APIServer
	l3APIServer      *api.L3APIServer
	adminServer      *AdminServer
	metricsCollector *metrics.MetricsCollector
	obsAuditLog      *observability.AuditLogger
	auditLogger      *audit.AuditLogger

	err error

	// L2/L3 wiring references
	accountMgr    *accounts.AccountManager
	agentMgr      *agents.AgentManager
	contractMgr   *contracts.ContractManager
	gasOracle     *gas.GasOracle
	govDAO        *governance.GovernanceDAO
	chainBridge   *bridge.ChainBridge
	interopProto  *interop.InteropProtocol
	intentSolver  *intent.IntentSolver
	appChainMgr   *appchain.AppChainManager
	shieldedPool  *privacy.ShieldedPool
	mevState      *mev.MEVState
	rollupChain   *rollups.RollupChain
}

func NewNodeBuilder(flags nodeFlags) *NodeBuilder {
	return &NodeBuilder{flags: flags}
}

func (nb *NodeBuilder) LoadConfig() *NodeBuilder {
	if nb.err != nil {
		return nb
	}

	fmt.Printf("Viri Daemon v%s\n", Version)
	fmt.Printf("Node: %s | Data: %s | Validator: %v\n", nb.flags.name, nb.flags.dataDir, nb.flags.validator)
	fmt.Println("Initializing...")

	if nb.flags.testnet {
		if nb.flags.config == "" {
			nb.flags.config = "configs/node-testnet.json"
		}
		if nb.flags.chainID == 0 {
			nb.flags.chainID = 99997
		}
	}

	cfg, err := config.LoadConfigOrDefault(nb.flags.config)
	if err != nil {
		nb.err = fmt.Errorf("failed to load config: %w", err)
		return nb
	}

	if nb.flags.genesis != "" {
		cfg.Chain.GenesisFile = nb.flags.genesis
	}
	if nb.flags.chainID != 0 {
		cfg.Chain.ChainID = nb.flags.chainID
	}

	cfg.Node.RPCPort = nb.flags.rpcPort
	cfg.Node.APIPort = nb.flags.apiPort
	cfg.Node.ValidatorMode = nb.flags.validator
	cfg.Node.RPCEnabled = nb.flags.rpc
	cfg.Node.APIEnabled = nb.flags.api
	cfg.Logging.Level = nb.flags.logLevel
	cfg.Node.Name = nb.flags.name
	if nb.flags.dataDir != "" {
		cfg.Node.DataDir = nb.flags.dataDir
	} else if cfg.Node.DataDir == "" {
		cfg.Node.DataDir = getDefaultDataDir()
	}

	cfg.ApplyEnvOverrides()

	observability.ConfigureReadiness(cfg.Readiness.MinPeers, cfg.Readiness.MinBlockHeight)
	observability.ForceReady(cfg.Readiness.ForceReady)

	nb.cfg = cfg
	return nb
}

func (nb *NodeBuilder) SetupLogging() *NodeBuilder {
	if nb.err != nil {
		return nb
	}

	log := logging.NewLogger("virid", logging.ParseLogLevel(nb.cfg.Logging.Level), nb.cfg.Logging.Level)

	if nb.cfg.Logging.Output != "" && nb.cfg.Logging.Output != "stdout" {
		maxSize := nb.cfg.Logging.MaxSize
		if maxSize <= 0 {
			maxSize = 100
		}
		maxBackups := nb.cfg.Logging.MaxBackups
		if maxBackups <= 0 {
			maxBackups = 7
		}
		rotatingWriter, err := logging.NewRotatingFileWriter(nb.cfg.Logging.Output, "virid", maxSize, maxBackups)
		if err != nil {
			log.WithField("error", err.Error()).Warn("Failed to set up log rotation, using stdout")
		} else {
			log.SetOutput(rotatingWriter)
			log.WithField("path", nb.cfg.Logging.Output).
				WithField("max_size_mb", maxSize).
				WithField("max_backups", maxBackups).
				Info("File-based log rotation enabled")
		}
	}

	log.WithField("chain_id", nb.cfg.Chain.ChainID).
		WithField("network", nb.cfg.Chain.NetworkName).
		WithField("validator", nb.flags.validator).
		WithField("name", nb.cfg.Node.Name).
		Info("Starting Viri node")

	if err := nb.cfg.Validate(); err != nil {
		nb.err = fmt.Errorf("invalid configuration: %w", err)
		return nb
	}

	if err := os.MkdirAll(nb.cfg.Node.DataDir, 0700); err != nil {
		nb.err = fmt.Errorf("failed to create data directory: %w", err)
		return nb
	}

	nb.flags.dataDir = nb.cfg.Node.DataDir
	nb.log = log
	return nb
}

func (nb *NodeBuilder) InitializeDB() *NodeBuilder {
	if nb.err != nil {
		return nb
	}

	version, err := state.CheckSchemaVersion(nb.flags.dataDir)
	if err != nil {
		nb.log.WithField("error", err.Error()).Warn("Could not check schema version")
	}
	if version != state.CurrentSchemaVersion {
		nb.log.WithField("current", version).
			WithField("expected", state.CurrentSchemaVersion).
			Info("Database schema migration required")
	}

	db := initDB(nb.flags, nb.log)

	if err := state.RunMigrations(db, nb.flags.dataDir); err != nil {
		nb.err = fmt.Errorf("failed to run migrations: %w", err)
		return nb
	}

	stateMgr, err := state.NewStateManager(db)
	if err != nil {
		nb.err = fmt.Errorf("failed to initialize state manager: %w", err)
		return nb
	}

	nb.db = db
	nb.stateMgr = stateMgr
	return nb
}

func (nb *NodeBuilder) InitializeBlockchain() *NodeBuilder {
	if nb.err != nil {
		return nb
	}

	var err error
	var genesis *ledger.GenesisConfig
	if nb.cfg.Chain.GenesisFile != "" {
		genesis, err = ledger.LoadGenesis(nb.cfg.Chain.GenesisFile)
		if err != nil {
			nb.err = fmt.Errorf("failed to load genesis file: %w", err)
			return nb
		}
	} else {
		genesis = ledger.DefaultGenesis()
		genesis.ChainID = nb.cfg.Chain.ChainID
	}

	if err := genesis.ValidateAndSanitize(); err != nil {
		nb.err = fmt.Errorf("invalid genesis configuration: %w", err)
		return nb
	}

	nb.log.WithField("chain_id", genesis.ChainID).
		WithField("supply", genesis.InitialSupply).
		Info("Genesis configuration validated")

	blockchain, err := ledger.NewPersistentBlockchain(genesis, nb.db)
	if err != nil {
		nb.err = fmt.Errorf("failed to initialize blockchain: %w", err)
		return nb
	}

	mempoolPersist := ledger.NewMempoolPersister(nb.cfg.Node.DataDir)
	pendingTxs, err := mempoolPersist.Load()
	if err != nil {
		nb.log.WithField("error", err.Error()).Warn("Failed to load mempool from disk")
	}
	for _, tx := range pendingTxs {
		if tx.Verify() {
			blockchain.TxPool().Add(tx)
		}
	}
	nb.log.WithField("restored", len(pendingTxs)).Info("Mempool restored from disk")

	nb.log.WithField("height", blockchain.Height()).
		WithField("tip", fmt.Sprintf("%x...", blockchain.TipHash()[:8])).
		Info("Blockchain initialized")

	if nb.stateMgr.IsInitialized() {
		nb.log.WithField("height", nb.stateMgr.BlockHeight()).
			WithField("supply", nb.stateMgr.TotalSupply()).
			Info("State already initialized, loading from DB")
	} else {
		nb.log.Info("Fresh state, initializing from genesis")
		if err := nb.stateMgr.Initialize(new(big.Int).SetUint64(genesis.InitialSupply)); err != nil {
			nb.err = fmt.Errorf("failed to initialize state: %w", err)
			return nb
		}

		for _, gv := range genesis.InitialValidators {
			if _, err := nb.stateMgr.CreateAccount(gv.Address, state.AccountTypeValidator, new(big.Int).SetUint64(gv.Stake)); err != nil {
				nb.log.WithField("address", fmt.Sprintf("%x", gv.Address)).
					WithField("error", err.Error()).
					Warn("Genesis validator account creation skipped")
			} else {
				nb.log.WithField("address", fmt.Sprintf("%x", gv.Address)).
					WithField("balance", gv.Stake).
					Info("Genesis validator account created")
			}
		}

		for addrHex, balStr := range genesis.Allocations {
			addr, err := hex.DecodeString(strings.TrimPrefix(addrHex, "0x"))
			if err != nil {
				nb.log.WithField("address", addrHex).WithField("error", err.Error()).
					Warn("Genesis allocation skipped: invalid address")
				continue
			}
			bal, ok := new(big.Int).SetString(balStr, 10)
			if !ok || bal.Sign() < 0 {
				nb.log.WithField("address", addrHex).WithField("balance", balStr).
					Warn("Genesis allocation skipped: invalid balance")
				continue
			}
			if _, err := nb.stateMgr.CreateAccount(addr, state.AccountTypeNormal, bal); err != nil {
				nb.log.WithField("address", addrHex).WithField("balance", balStr).
					WithField("error", err.Error()).
					Warn("Genesis allocation account creation skipped")
			} else {
				nb.log.WithField("address", addrHex).WithField("balance", balStr).
					Info("Genesis allocation account created")
			}
		}
	}

	nb.blockchain = blockchain
	nb.mempoolPersist = mempoolPersist
	return nb
}

func (nb *NodeBuilder) SetupExecutionEngine() *NodeBuilder {
	if nb.err != nil {
		return nb
	}

	execEngine := execution.NewExecutionEngine()
	if nb.flags.parallelExec {
		execEngine.SetParallel(true)
	}
	nb.log.WithField("vm", "evm").
		WithField("parallel", nb.flags.parallelExec).
		Info("L2 Execution Engine initialized (transfers, deploys, calls)")

	nb.accountMgr = accounts.NewAccountManager()
	nb.agentMgr = agents.NewAgentManager()
	nb.contractMgr = contracts.NewContractManager()
	nb.gasOracle = gas.NewGasOracle(gas.DefaultGasConfig())
	nb.shieldedPool = privacy.NewShieldedPool()

	mevModeVal := mev.StandardMode
	switch nb.flags.mevMode {
	case "encrypted":
		mevModeVal = mev.EncryptedMode
	case "commit-reveal":
		mevModeVal = mev.CommitReveal
	}
	nb.mevState = mev.NewMEVState(mevModeVal)
	nb.log.WithField("mode", nb.flags.mevMode).Info("MEV resistance module initialized")

	nb.rollupChain = rollups.NewRollupChain("main", rollups.RollupTypeOptimistic, 100)
	zkCircuit := zk.NewShieldedTransferCircuit()
	gv := zk.NewGnarkVerifier()
	execEngine.SetGnarkVerifier(gv, zkCircuit)
	nb.log.Info("Gnark-based ZK verifier enabled")

	execEngine.SetShieldedPool(nb.shieldedPool)
	execEngine.SetContractManager(nb.contractMgr)

	feeOracle := gas.NewFeeConversionOracle(5 * time.Minute)
	for tokenKey, rate := range gas.DefaultConversionRates() {
		feeOracle.SetRate([]byte(tokenKey), rate)
	}
	feeOracle.SetRate(contracts.AddrERC20, 1.0)
	execEngine.SetFeeOracle(feeOracle)
	nb.log.WithField("tokens", len(feeOracle.KnownTokens())).Info("Fee Conversion Oracle initialized")

	nb.log.WithField("modules", "accounts,agents,contracts,gas,feeOracle,mev,privacy,rollups,zk").
		Info("L2 modules initialized")

	nb.execEngine = execEngine
	return nb
}

func (nb *NodeBuilder) SetupL3Modules() *NodeBuilder {
	if nb.err != nil {
		return nb
	}

	nb.govDAO = governance.NewGovernanceDAO(24*time.Hour, 1_000_000, 0.1)
	nb.chainBridge = bridge.NewChainBridge(2)
	nb.interopProto = interop.NewInteropProtocol()
	nb.intentSolver = intent.NewIntentSolver()
	nb.appChainMgr = appchain.NewAppChainManager()

	zkCircuit := zk.NewShieldedTransferCircuit()
	privPk := zk.GenerateProvingKey(zkCircuit)
	privVk := zk.GenerateVerifyingKey(privPk, zkCircuit)
	privacyBridge := bridge.NewPrivacyBridge(2, zkCircuit, privVk, privPk)
	privacyBridge.RegisterChain("viri-main", "Viri Main Chain", "http://localhost:8545")

	nb.log.WithField("modules", "governance,bridge,interop,intent,appchain,privacy_bridge").
		Info("L3 modules initialized")

	return nb
}

func (nb *NodeBuilder) SetupNetwork() *NodeBuilder {
	if nb.err != nil {
		return nb
	}

	key := loadKey(nb.flags, nb.cfg, nb.log, resolveScheme(nb.flags.scheme))

	initialBalance := new(big.Int).Mul(big.NewInt(1_000_000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	_, err := nb.stateMgr.CreateAccount(key.PubKey().Address(), state.AccountTypeValidator, initialBalance)
	if err != nil {
		nb.log.WithField("error", err.Error()).Warn("Validator account creation skipped")
	}

	nb.log.WithField("address", fmt.Sprintf("%x", key.PubKey().Address())).
		WithField("pubkey", key.PubKey().Hex()[:16]+"...").
		Info("Validator key ready")

	validatorAddr := key.PubKey().Address()
	viriToken := contracts.NewERC20Token("VIRI Token", "VIRI", 18, new(big.Int).Mul(big.NewInt(1_000_000), big.NewInt(1e18)), validatorAddr)
	nb.contractMgr.RegisterStandardContract(contracts.AddrERC20, viriToken)
	nb.log.WithField("address", fmt.Sprintf("%x", contracts.AddrERC20)).
		WithField("supply", "1000000000000000000000000").
		Info("Standard ERC-20 token deployed")

	nftToken := contracts.NewERC721Token("VIRI NFT", "VNFT", "https://viri-chain.io/nft/")
	nb.contractMgr.RegisterStandardContract(contracts.AddrERC721, nftToken)
	nftToken.Mint(validatorAddr, 1, "genesis-viri-1")
	nftToken.Mint(validatorAddr, 2, "genesis-viri-2")
	nftToken.Mint(validatorAddr, 3, "genesis-viri-3")
	nb.log.WithField("address", fmt.Sprintf("%x", contracts.AddrERC721)).
		WithField("minted", 3).
		Info("Standard ERC-721 NFT collection deployed")

	seqConfig := sequencer.DefaultSequencerConfig()
	seqConfig.ProposerKey = key
	seq := sequencer.NewSequencer(seqConfig, nb.blockchain)
	if err := seq.Start(); err != nil {
		nb.log.Warn(fmt.Sprintf("Sequencer start skipped: %v", err))
	} else {
		nb.log.WithField("batch_size", seqConfig.BatchSize).WithField("timeout", seqConfig.BatchTimeout).Info("Sequencer started")
	}

	if err := nb.agentMgr.Register("validator-0", agents.AgentTypeValidator, key.PubKey().Address(), 10_000_000); err != nil {
		nb.log.WithField("error", err.Error()).Warn("Agent registration skipped")
	}

	nb.interopProto.RegisterHandler("default", func(packet *interop.IBCPacket) ([]byte, error) {
		nb.log.WithField("channel", packet.SourceChain+"->"+packet.DestChain).
			WithField("sequence", packet.Sequence).
			Info("Interop packet received")
		return packet.Data, nil
	})

	txPool := nb.blockchain.TxPool()
	nb.log.WithField("pending", len(txPool.GetPending())).
		Info("Transaction pool ready")

	economics := nb.blockchain.Economics()
	nb.log.WithField("circulating", economics.CirculatingSupply().String()).
		Info("Economics module initialized")

	snapshot := nb.stateMgr.Snapshot()
	nb.log.WithField("block_height", snapshot.BlockHeight).
		WithField("accounts", snapshot.NumAccounts).
		Info("State snapshot")

	netConfig := p2p.DefaultNetworkConfig()
	netConfig.ChainID = nb.cfg.Chain.ChainID
	netConfig.ListenAddr = fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", nb.flags.p2pPort)
	if nb.cfg.Network.ExternalAddr != "" {
		netConfig.ExternalAddr = nb.cfg.Network.ExternalAddr
	}
	if nb.flags.noMDNS || runtime.GOOS == "windows" {
		netConfig.EnableMDNS = false
	}

	if nb.flags.bootnodes != "" {
		for _, addr := range strings.Split(nb.flags.bootnodes, ",") {
			addr = strings.TrimSpace(addr)
			if addr != "" {
				netConfig.Bootstraps = append(netConfig.Bootstraps, addr)
			}
		}
	} else if len(nb.cfg.Network.BootstrapPeers) > 0 {
		netConfig.Bootstraps = nb.cfg.Network.BootstrapPeers
	}

	var p2pPrivKey *crypto.PrivateKey
	if nb.flags.p2pKey != "" {
		keyBytes, err := hex.DecodeString(nb.flags.p2pKey)
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

	viriNet, err := p2p.NewViriNetwork(netConfig, nb.blockchain, nb.log, p2pPrivKey)
	if err != nil {
		nb.err = fmt.Errorf("failed to create network: %w", err)
		return nb
	}

	viriNet.SetValidatorAddress(key.PubKey().Address())
	viriNet.SetValidatorPubKey(key.PubKey().Bytes())

	nb.viriNet = viriNet
	nb.validatorKey = key
	return nb
}

func (nb *NodeBuilder) SetupConsensus() *NodeBuilder {
	if nb.err != nil {
		return nb
	}

	syncConfig := nodesync.DefaultSyncConfig()
	switch nb.flags.syncMode {
	case "full":
		syncConfig.Mode = nodesync.FullSync
	case "fast":
		syncConfig.Mode = nodesync.FastSync
	case "snap":
		syncConfig.Mode = nodesync.SnapSync
	}

	nodeSyncer := nodesync.NewSyncer(syncConfig, nb.log)

	if nb.blockchain.Height() == 0 && nb.flags.bootnodes != "" {
		nb.log.WithField("mode", nb.flags.syncMode).Info("Starting node sync from genesis")
	}

	var genesis *ledger.GenesisConfig
	var err error
	if nb.cfg.Chain.GenesisFile != "" {
		genesis, err = ledger.LoadGenesis(nb.cfg.Chain.GenesisFile)
		if err != nil {
			nb.err = fmt.Errorf("failed to load genesis file for consensus setup: %w", err)
			return nb
		}
	} else {
		genesis = ledger.DefaultGenesis()
		genesis.ChainID = nb.cfg.Chain.ChainID
	}

	blockProducer := newChainBlockProducer(nb.blockchain, nb.validatorKey, nb.execEngine, nb.stateMgr, nb.gasOracle, nb.mevState, nb.shieldedPool, nb.rollupChain, nb.agentMgr)

	validators := make([]*consensus.Validator, 0, len(genesis.InitialValidators))
	for _, gv := range genesis.InitialValidators {
		validators = append(validators, &consensus.Validator{
			Address:   gv.Address,
			PublicKey: gv.PublicKey,
			Stake:     gv.Stake,
			IsActive:  true,
		})
	}

	if len(validators) == 0 {
		validators = append(validators, &consensus.Validator{
			Address:   nb.validatorKey.PubKey().Address(),
			PublicKey: nb.validatorKey.PubKey().Bytes(),
			Stake:     1000000,
			IsActive:  true,
		})
	}

	for _, ev := range nb.cfg.Consensus.ExtraValidators {
		addr, err := hex.DecodeString(strings.TrimPrefix(ev.Address, "0x"))
		if err != nil {
			nb.log.WithField("address", ev.Address).Warn("Invalid extra validator address, skipping")
			continue
		}
		pubBytes, err := hex.DecodeString(strings.TrimPrefix(ev.PublicKey, "0x"))
		if err != nil {
			nb.log.WithField("address", ev.Address).Warn("Invalid extra validator public key, skipping")
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
		nb.log.WithField("address", fmt.Sprintf("%x", addr)).
			WithField("stake", stake).
			Info("Extra validator added from config")
	}

	nb.log.WithField("validators", len(validators)).Info("Loaded validators from genesis")

	validatorSet := consensus.NewValidatorSet(validators, 1)
	staking := consensus.NewStakingModule(21*24*time.Hour, 0.01)
	for _, v := range validators {
		if err := staking.Stake(v.Address, v.PublicKey, v.Stake); err != nil {
			nb.log.WithField("address", fmt.Sprintf("%x", v.Address)).
				WithField("error", err.Error()).
				Warn("Failed to stake genesis validator")
		}
	}

	// Update network total stake in Governance DAO for quorum calculations
	nb.govDAO.SetTotalNetworkStake(staking.TotalStaked())
	nb.log.WithField("total_network_stake", staking.TotalStaked()).Info("Governance total network stake updated")

	consensusConfig := consensus.DefaultConsensusConfig()
	consensusConfig.BlockTime = nb.cfg.Chain.BlockTime.Duration()
	consensusConfig.ViewTimeout = 5 * time.Second
	consensusConfig.MaxViewTimeout = 15 * time.Second
	consensusConfig.TimeoutIncrease = 2 * time.Second
	consensusConfig.EpochLength = 1000
	if len(validators) > 1 {
		consensusConfig.MinValidators = len(validators)
	} else if nb.flags.validator {
		consensusConfig.MinValidators = 1
	}

	auditConfig := audit.DefaultAuditConfig()
	auditConfig.OutputPath = filepath.Join(nb.flags.dataDir, "audit")
	auditLogger, err := audit.NewAuditLogger(auditConfig)
	if err != nil {
		nb.log.Error(fmt.Sprintf("Failed to create audit logger: %v", err))
	}

	engine := consensus.NewHotStuffEngine(consensusConfig, validatorSet, blockProducer, staking, nb.log, auditLogger)

	econConfig := ledger.DefaultEconomicsConfig()
	rewardEcon := ledger.NewEconomics(econConfig)
	engine.BlockRewardFn = func(height uint64, proposer []byte, _ *big.Int) {
		reward := rewardEcon.CalculateBlockReward(height)
		if reward.Sign() > 0 {
			if err := nb.stateMgr.MintTokens(proposer, reward); err != nil {
				nb.log.WithField("height", height).
					WithField("proposer", fmt.Sprintf("%x", proposer)).
					WithField("reward", reward.String()).
					Error(fmt.Sprintf("Failed to mint block reward: %v", err))
			} else {
				nb.log.WithField("height", height).
					WithField("proposer", fmt.Sprintf("%x", proposer)).
					WithField("reward", reward.String()).
					Info("Block reward minted")
			}
			if err := nb.stateMgr.Commit(height); err != nil {
				nb.log.WithField("height", height).
					WithField("error", err.Error()).
					Warn("Failed to commit state after block reward")
			}
		}
	}

	metricsCollector := metrics.NewMetricsCollector()
	engine.SetMetrics(metricsCollector)
	nb.viriNet.SetMetrics(metricsCollector)
	metricsCollector.StartMetricsServer(9090)

	nb.nodeSyncer = nodeSyncer
	nb.consensusEngine = engine
	nb.validatorSet = validatorSet
	nb.metricsCollector = metricsCollector
	nb.auditLogger = auditLogger
	return nb
}

func (nb *NodeBuilder) SetupServers() *NodeBuilder {
	if nb.err != nil {
		return nb
	}

	if nb.cfg.Node.APIKeyHash == "" && (nb.flags.rpc || nb.flags.api) {
		apiKeyPath := filepath.Join(nb.flags.dataDir, "api_key.txt")
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
		nb.cfg.Node.APIKeyHash = hex.EncodeToString(h[:])
		nb.log.WithField("hint", fmt.Sprintf("%s...%s", rawKey[:8], rawKey[len(rawKey)-4:])).Warn("Generated API key hash")
	}

	tlsCert := nb.cfg.Node.TLSCertPath
	tlsKey := nb.cfg.Node.TLSKeyPath
	if nb.flags.tlsAuto && (tlsCert == "" || tlsKey == "") {
		nb.log.Info("Auto-generating self-signed TLS certificates...")
		genCert, genKey, err := EnsureTLSCerts(nb.flags.dataDir)
		if err != nil {
			nb.log.Error(fmt.Sprintf("TLS auto-generation failed: %v", err))
		} else {
			tlsCert = genCert
			tlsKey = genKey
			nb.log.WithField("cert", genCert).WithField("key", genKey).Info("TLS certificates ready")
		}
	}

	obsAuditLog, err := observability.NewAuditLogger(filepath.Join(nb.flags.dataDir, "logs"), 100, 3)
	if err != nil {
		nb.log.WithField("error", err.Error()).Warn("Observability audit logging disabled")
	}

	var rpcServer *RPCServer
	if nb.flags.rpc {
		entryPoint := accounts.NewEntryPoint(nb.accountMgr, nb.cfg.Chain.ChainID, nil)
		rpcServer = NewRPCServer(nb.flags.rpcPort, nb.blockchain, nb.stateMgr, nb.viriNet, nb.consensusEngine, nb.log, nb.cfg.Chain.ChainID, nb.flags.validator, nb.validatorKey.PubKey().Address(), tlsCert, tlsKey, nb.cfg.Node.APIKeyHash, obsAuditLog, nb.nodeSyncer, entryPoint, nb.contractMgr, nb.shieldedPool, nb.mevState, nb.rollupChain)
	}

	wsPort := nb.flags.rpcPort + 2
	wsServer := NewWSServer(wsPort, nb.blockchain, nb.viriNet, nb.log, tlsCert, tlsKey, nb.cfg.Node.APIKeyHash)

	var apiServer *APIServer
	if nb.flags.api {
		apiServer = NewAPIServer(nb.flags.apiPort, nb.blockchain, nb.stateMgr, nb.viriNet, nb.log, tlsCert, tlsKey, nb.cfg.Node.APIKeyHash)
	}

	l3APIServer := api.NewL3APIServer(nb.flags.l3Port, nb.govDAO, nb.chainBridge, nb.interopProto, nb.intentSolver, nb.appChainMgr, nb.agentMgr)

	adminPort := nb.flags.rpcPort + 4
	adminServer := NewAdminServer(adminPort, nb.blockchain, nb.stateMgr, nb.viriNet, nb.consensusEngine, nb.log, nb.cfg.Node.APIKeyHash)

	nb.rpcServer = rpcServer
	nb.wsServer = wsServer
	nb.apiServer = apiServer
	nb.l3APIServer = l3APIServer
	nb.adminServer = adminServer
	nb.obsAuditLog = obsAuditLog

	return nb
}

func (nb *NodeBuilder) Build() (*Node, error) {
	if nb.err != nil {
		return nil, nb.err
	}

	return &Node{
		flags:            nb.flags,
		cfg:              nb.cfg,
		log:              nb.log,
		db:               nb.db,
		stateMgr:         nb.stateMgr,
		blockchain:       nb.blockchain,
		mempoolPersist:   nb.mempoolPersist,
		execEngine:       nb.execEngine,
		viriNet:          nb.viriNet,
		nodeSyncer:       nb.nodeSyncer,
		consensusEngine:  nb.consensusEngine,
		validatorKey:     nb.validatorKey,
		validatorSet:     nb.validatorSet,
		rpcServer:        nb.rpcServer,
		wsServer:         nb.wsServer,
		apiServer:        nb.apiServer,
		l3APIServer:      nb.l3APIServer,
		adminServer:      nb.adminServer,
		metricsCollector: nb.metricsCollector,
		obsAuditLog:      nb.obsAuditLog,
		auditLogger:      nb.auditLogger,
		stopCh:           make(chan struct{}),
	}, nil
}

func (n *Node) Start() {
	if err := n.viriNet.Start(); err != nil {
		n.log.Fatal(fmt.Sprintf("Failed to start network: %v", err))
	}

	if n.flags.writePeerFile != "" {
		if err := n.viriNet.WritePeerInfo(n.flags.writePeerFile); err != nil {
			n.log.Warn(fmt.Sprintf("Failed to write peer info: %v", err))
		} else {
			n.log.Info(fmt.Sprintf("Peer info written to %s", n.flags.writePeerFile))
		}
	}

	if n.flags.peerFile != "" {
		go func() {
			for {
				if err := n.viriNet.ReadAndConnectPeerInfo(n.flags.peerFile); err != nil {
					time.Sleep(2 * time.Second)
					continue
				}
				break
			}
		}()
	}

	n.consensusEngine.SetBroadcast(func(msg *consensus.ConsensusMessage) {
		data, err := json.Marshal(msg)
		if err != nil {
			n.log.WithField("error", err.Error()).Warn("Failed to marshal consensus message")
			return
		}
		if err := n.viriNet.PublishConsensus(data); err != nil {
			n.log.WithField("error", err.Error()).Warn("Failed to publish consensus message")
		}
	})

	if err := n.viriNet.SubscribeToConsensus(func(msg *p2p.Message, from peer.ID) {
		var consensusMsg consensus.ConsensusMessage
		if err := json.Unmarshal(msg.Payload, &consensusMsg); err != nil {
			n.log.WithField("error", err.Error()).Warn("Failed to unmarshal consensus message")
			return
		}
		n.consensusEngine.HandleMessage(&consensusMsg)
	}); err != nil {
		n.log.Error(fmt.Sprintf("Failed to subscribe to consensus: %v", err))
	}

	n.viriNet.SetMessageHandler(&p2p.SimpleMessageHandler{
		OnBlockHandler: func(msg *p2p.Message, from peer.ID) error {
			n.log.WithField("peer", from.String()).
				WithField("size", len(msg.Payload)).
				Info("Received block from peer")
			if n.consensusEngine != nil && n.consensusEngine.GetStateSyncer() != nil && n.consensusEngine.GetStateSyncer().IsSyncing() {
				if err := n.consensusEngine.GetStateSyncer().ReceiveBlock(msg.Payload); err != nil {
					n.log.WithField("error", err.Error()).Warn("Failed to receive sync block")
				}
			}
			return nil
		},
		OnTransactionHandler: func(msg *p2p.Message, from peer.ID) error {
			n.log.WithField("peer", from.String()).
				WithField("size", len(msg.Payload)).
				Info("Received transaction from peer")
			tx, err := ledger.DeserializeTransaction(msg.Payload)
			if err != nil {
				n.log.WithField("error", err.Error()).Warn("Failed to deserialize received transaction")
				return nil
			}
			if !tx.Verify() {
				n.log.Warn("Received invalid transaction from peer")
				return nil
			}
			txPool := n.blockchain.TxPool()
			if err := txPool.Add(tx); err != nil {
				n.log.WithField("error", err.Error()).Debug("Received transaction not added to pool")
			}
			return nil
		},
		OnGetBlocksHandler: func(msg *p2p.Message, from peer.ID) error {
			n.log.WithField("peer", from.String()).Info("Peer requested blocks")
			return nil
		},
		OnGetHeadersHandler: func(msg *p2p.Message, from peer.ID) error {
			n.log.WithField("peer", from.String()).Info("Peer requested headers")
			return nil
		},
		OnAnnounceHandler: func(msg *p2p.Message, from peer.ID) error {
			n.log.WithField("peer", from.String()).Info("Peer announced new data")
			return nil
		},
	})

	if err := n.viriNet.SubscribeToBlocks(func(msg *p2p.Message, from peer.ID) {
		n.log.Debug(fmt.Sprintf("Block subscription: peer=%s size=%d", from.String(), len(msg.Payload)))
		if n.consensusEngine != nil && n.consensusEngine.GetStateSyncer() != nil && n.consensusEngine.GetStateSyncer().IsSyncing() {
			if err := n.consensusEngine.GetStateSyncer().ReceiveBlock(msg.Payload); err != nil {
				n.log.Debug(fmt.Sprintf("Sync block receive failed: %v", err))
			}
		}
	}); err != nil {
		n.log.Error(fmt.Sprintf("Failed to subscribe to blocks: %v", err))
	}

	if err := n.viriNet.SubscribeToTransactions(func(msg *p2p.Message, from peer.ID) {
		n.log.Debug(fmt.Sprintf("Transaction subscription: peer=%s size=%d", from.String(), len(msg.Payload)))
	}); err != nil {
		n.log.Error(fmt.Sprintf("Failed to subscribe to transactions: %v", err))
	}

	if err := n.viriNet.SubscribeToHeaders(func(msg *p2p.Message, from peer.ID) {
		n.log.Debug(fmt.Sprintf("Header subscription: peer=%s size=%d", from.String(), len(msg.Payload)))
	}); err != nil {
		n.log.Error(fmt.Sprintf("Failed to subscribe to headers: %v", err))
	}

	eventBus := events.NewEventBus()
	eventBus.Subscribe(events.EventBlockAdded, func(event events.Event) {
		block := event.Data.(*ledger.Block)
		n.log.WithField("height", block.Header.Height).Info("New block added")
		if n.wsServer != nil {
			n.wsServer.BroadcastBlock(block)
		}
	})

	// Emergency shutdown handler for DoS protection
	doEmergencyShutdown := func() {
		n.shutdownOnce.Do(func() {
			n.log.Warn("!!!! EMERGENCY SHUTDOWN TRIGGERED !!!!")
			if n.flags.validator {
				n.consensusEngine.Stop()
			}
			n.viriNet.Close()
			n.db.Close()
			n.stateMgr.Close()
			os.Exit(1)
		})
	}

	if n.viriNet.GetDoSProtector() != nil {
		n.viriNet.GetDoSProtector().SetEmergencyHandler(doEmergencyShutdown)
	}

	if n.flags.validator {
		if n.flags.consensusDelay > 0 {
			n.log.Info(fmt.Sprintf("Waiting %s for peer discovery before starting consensus", n.flags.consensusDelay))
			time.Sleep(n.flags.consensusDelay)
			n.log.WithField("validators", n.validatorSet.Size()).Info("Starting consensus after peer discovery delay")
		}
		if err := n.consensusEngine.Start(n.blockchain.Height() + 1); err != nil {
			n.log.Error(fmt.Sprintf("Failed to start consensus engine: %v", err))
		}

		n.consensusEngine.OnSyncComplete(func() {
			n.log.WithField("height", n.blockchain.Height()).Info("State sync completed, resuming consensus")
		})
	}

	n.log.WithField("height", n.blockchain.Height()).
		WithField("validators", n.validatorSet.Size()).
		WithField("epoch", n.validatorSet.Epoch()).
		WithField("consensus", n.flags.validator).
		Info("Consensus engine initialized")

	if n.rpcServer != nil {
		if err := n.rpcServer.Start(); err != nil {
			n.log.Error(fmt.Sprintf("Failed to start RPC server: %v", err))
		}
	}

	if n.wsServer != nil {
		if err := n.wsServer.Start(); err != nil {
			n.log.Error(fmt.Sprintf("Failed to start WebSocket server: %v", err))
		}
	}

	if n.apiServer != nil {
		if err := n.apiServer.Start(); err != nil {
			n.log.Error(fmt.Sprintf("Failed to start API server: %v", err))
		}
	}

	if n.l3APIServer != nil {
		if err := n.l3APIServer.Start(); err != nil {
			n.log.Error(fmt.Sprintf("Failed to start L3 API server: %v", err))
		}
	}

	if n.adminServer != nil {
		if err := n.adminServer.Start(); err != nil {
			n.log.Error(fmt.Sprintf("Failed to start Admin API server: %v", err))
		}
	}

	go func() {
		time.Sleep(5 * time.Second)
		n.viriNet.BroadcastGetPeers()
		n.log.Info("Initial peer discovery broadcast sent")
	}()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stats := n.viriNet.Stats().Snapshot()
				livePeers := n.viriNet.LivePeerCount()
				n.log.WithField("peers", livePeers).
					WithField("blocks_in", stats.TotalBlocksIn).
					WithField("blocks_out", stats.TotalBlocksOut).
					WithField("txs_in", stats.TotalTxsIn).
					WithField("txs_out", stats.TotalTxsOut).
					WithField("bytes_in", stats.TotalBytesIn).
					WithField("bytes_out", stats.TotalBytesOut).
					WithField("rejected", stats.RejectedMessages).
					WithField("conn_active", livePeers).
					WithField("uptime", stats.Uptime.String()).
					Info("Network stats")
			case <-n.stopCh:
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
				n.metricsCollector.UpdateUptime()
				syncing := n.nodeSyncer != nil && n.nodeSyncer.IsSyncing()
				mempoolSize := n.blockchain.TxPool().Size()
				n.metricsCollector.SetNodeIsSyncing(syncing)
				n.metricsCollector.SetMempoolPendingTxs(mempoolSize)
				n.metricsCollector.SetHealthData(n.blockchain.Height(), n.viriNet.PeerCount(), syncing)

				if mempoolSize > 0 {
					pressure := n.blockchain.TxPool().PressureLevel()
					if pressure > 0.9 {
						n.log.WithField("pressure", pressure).
							WithField("pending", mempoolSize).
							Warn("Mempool under high pressure")
					}
				}
			case <-n.stopCh:
				return
			}
		}
	}()

	n.log.WithField("rpc_port", n.flags.rpcPort).
		WithField("api_port", n.flags.apiPort).
		WithField("p2p_port", n.flags.p2pPort).
		WithField("rpc_enabled", n.flags.rpc).
		WithField("api_enabled", n.flags.api).
		Info("Viri node is running")
}

func (n *Node) Wait() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("Press Ctrl+C to stop.")
	select {
	case <-sigCh:
	case <-n.stopCh:
	}
}

func (n *Node) Stop() {
	n.log.Info("Shutting down gracefully...")

	close(n.stopCh)

	if n.nodeSyncer != nil {
		n.nodeSyncer.Stop()
	}

	if n.mempoolPersist != nil {
		if err := n.mempoolPersist.Save(n.blockchain.TxPool()); err != nil {
			n.log.WithField("error", err.Error()).Warn("Failed to save mempool to disk")
		} else {
			n.log.WithField("pending", n.blockchain.TxPool().Size()).Info("Mempool saved to disk")
		}
	}

	n.log.Info("Draining connections...")

	if n.wsServer != nil {
		if err := n.wsServer.Stop(); err != nil {
			n.log.Error(fmt.Sprintf("Error stopping WebSocket server: %v", err))
		}
	}

	if n.rpcServer != nil {
		if err := n.rpcServer.Stop(); err != nil {
			n.log.Error(fmt.Sprintf("Error stopping RPC server: %v", err))
		}
	}

	if n.apiServer != nil {
		if err := n.apiServer.Stop(); err != nil {
			n.log.Error(fmt.Sprintf("Error stopping API server: %v", err))
		}
	}

	if n.l3APIServer != nil {
		if err := n.l3APIServer.Stop(); err != nil {
			n.log.Error(fmt.Sprintf("Error stopping L3 API server: %v", err))
		}
	}

	if n.adminServer != nil {
		if err := n.adminServer.Stop(); err != nil {
			n.log.Error(fmt.Sprintf("Error stopping Admin API server: %v", err))
		}
	}

	if n.flags.validator {
		n.consensusEngine.Stop()
	}

	drainCtx, drainCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := n.viriNet.Drain(drainCtx); err != nil {
		n.log.Warn(fmt.Sprintf("P2P drain warning: %v", err))
	}
	drainCancel()

	if err := n.viriNet.Close(); err != nil {
		n.log.Error(fmt.Sprintf("Error closing network: %v", err))
	}

	if err := n.db.Close(); err != nil {
		n.log.Error(fmt.Sprintf("Error closing database: %v", err))
	}

	if err := n.stateMgr.Close(); err != nil {
		n.log.Error(fmt.Sprintf("Error closing state manager: %v", err))
	}

	n.log.Info("Viri node stopped")
}
