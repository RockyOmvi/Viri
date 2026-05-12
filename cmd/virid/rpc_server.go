package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer1/consensus"
	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
	"github.com/viri-chain/viri/internal/layer1/p2p"
	"github.com/viri-chain/viri/internal/layer1/state"
	nodesync "github.com/viri-chain/viri/internal/layer1/sync"
	"github.com/viri-chain/viri/internal/layer2/accounts"
	"github.com/viri-chain/viri/internal/layer2/execution"
	"github.com/viri-chain/viri/internal/layer2/vm"
	"github.com/viri-chain/viri/internal/pkg/observability"
	"github.com/viri-chain/viri/internal/pkg/security"
)

type RPCServer struct {
	mu         sync.Mutex
	port       int
	chainID    uint64
	validator  bool
	coinbase   []byte
	blockchain *ledger.PersistentBlockchain
	stateMgr   *state.StateManager
	network    *p2p.ViriNetwork
	engine     *consensus.HotStuffEngine
	logger     *logging.Logger
	server     *http.Server
	methods    map[string]RPCHandler
	tlsCert    string
	tlsKey     string
	apiKeyHash string
	auditLog   *observability.AuditLogger
	syncer     *nodesync.Syncer
	filters    sync.Map
	drainer    *security.ConnectionDrainer
	entryPoint *accounts.EntryPoint
}

type RPCHandler func(ctx context.Context, params json.RawMessage) (interface{}, error)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type LogFilter struct {
	ID       string
	FromBlock uint64
	ToBlock   uint64
	Address   string
	Topics    []string
	Logs      []FilterLog
	LastPoll  time.Time
}

type FilterLog struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockNumber string   `json:"blockNumber"`
	BlockHash   string   `json:"blockHash"`
	TxHash      string   `json:"transactionHash"`
	TxIndex     string   `json:"transactionIndex"`
}

func NewRPCServer(port int, bc *ledger.PersistentBlockchain, sm *state.StateManager, net *p2p.ViriNetwork, engine *consensus.HotStuffEngine, log *logging.Logger, chainID uint64, validator bool, coinbase []byte, tlsCert, tlsKey, apiKeyHash string, auditLog *observability.AuditLogger, syncer *nodesync.Syncer, ep *accounts.EntryPoint) *RPCServer {
	s := &RPCServer{
		port:       port,
		chainID:    chainID,
		validator:  validator,
		coinbase:   coinbase,
		blockchain: bc,
		stateMgr:   sm,
		network:    net,
		engine:     engine,
		logger:     log,
		methods:    make(map[string]RPCHandler),
		tlsCert:    tlsCert,
		tlsKey:     tlsKey,
		apiKeyHash: apiKeyHash,
		auditLog:   auditLog,
		syncer:     syncer,
		drainer:    security.NewConnectionDrainer(30 * time.Second),
		entryPoint: ep,
	}

	s.registerMethods()
	return s
}

func (s *RPCServer) registerMethods() {
	s.methods["eth_blockNumber"] = s.getBlockNumber
	s.methods["eth_getBlockByNumber"] = s.getBlockByNumber
	s.methods["eth_getBlockByHash"] = s.getBlockByHash
	s.methods["eth_getTransactionCount"] = s.getTransactionCount
	s.methods["eth_getBalance"] = s.getBalance
	s.methods["eth_sendRawTransaction"] = s.sendRawTransaction
	s.methods["eth_chainId"] = s.getChainID
	s.methods["net_version"] = s.getNetVersion
	s.methods["net_peerCount"] = s.getPeerCount
	s.methods["viri_nodeInfo"] = s.getNodeInfo
	s.methods["viri_getPeers"] = s.getPeers
	s.methods["viri_getConsensusState"] = s.getConsensusState
	s.methods["viri_addPeer"] = s.addPeer
	s.methods["web3_clientVersion"] = s.getClientVersion
	s.methods["eth_coinbase"] = s.getCoinbase
	s.methods["eth_gasPrice"] = s.getGasPrice
	s.methods["eth_getTransactionByHash"] = s.getTransactionByHash
	s.methods["eth_syncing"] = s.getSyncing
	s.methods["eth_getLogs"] = s.getLogs
	s.methods["eth_newFilter"] = s.newFilter
	s.methods["eth_getFilterChanges"] = s.getFilterChanges
	s.methods["eth_uninstallFilter"] = s.uninstallFilter
	s.methods["eth_getTransactionReceipt"] = s.getTransactionReceipt
	s.methods["eth_getBlockReceipts"] = s.getBlockReceipts
	s.methods["eth_call"] = s.call
	s.methods["eth_estimateGas"] = s.estimateGas
	s.methods["eth_sendUserOperation"] = s.sendUserOperation
	s.methods["eth_estimateUserOperationGas"] = s.estimateUserOperationGas
	s.methods["eth_getCode"] = s.getCode
	s.methods["eth_getStorageAt"] = s.getStorageAt
	s.methods["debug_traceTransaction"] = s.traceTransaction
	s.methods["eth_protocolVersion"] = s.getProtocolVersion
	s.methods["eth_hashrate"] = s.getHashrate
	s.methods["eth_mining"] = s.getMining
	s.methods["eth_getBlockTransactionCountByHash"] = s.getBlockTxCountByHash
	s.methods["eth_getBlockTransactionCountByNumber"] = s.getBlockTxCountByNumber
	s.methods["eth_getTransactionByBlockHashAndIndex"] = s.getTxByBlockHashAndIndex
	s.methods["eth_getTransactionByBlockNumberAndIndex"] = s.getTxByBlockNumberAndIndex
	s.methods["eth_getUncleCountByBlockHash"] = s.getUncleCountByHash
	s.methods["eth_getUncleCountByBlockNumber"] = s.getUncleCountByNumber
}

func (s *RPCServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)
	mux.Handle("/metrics", observability.LocalOnly(observability.MetricsHandler()))

	if s.apiKeyHash == "" {
		s.logger.Warn("RPC server API key auth is disabled")
	}

	getClientID := func(r *http.Request) string {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			return strings.TrimSpace(parts[0])
		}
		return r.RemoteAddr
	}

	rateLimiter := security.NewRateLimiter(10.0, 20)
	ddosDetector := security.NewDDoSDetector(10*time.Second, 100, 5*time.Minute)
	connLimiter := security.NewConnectionLimiter(100, 25)

	methodLimits := map[string]security.MethodLimit{
		"eth_getBlockByNumber":  {RPS: 5.0, Burst: 10},
		"eth_getBlockByHash":    {RPS: 5.0, Burst: 10},
		"eth_getLogs":           {RPS: 2.0, Burst: 5},
		"viri_getConsensusState": {RPS: 3.0, Burst: 6},
	}
	methodRateLimiter := security.NewMethodRateLimiter(20.0, 40, methodLimits)

	slowQueryDetector := security.NewSlowQueryDetector(1*time.Minute, 30, 5*time.Minute)

	baseHandler := observability.InstrumentHandler("rpc", mux, func() {
		observability.SetChainStats("rpc", s.blockchain.Height(), s.network.PeerCount())
		observability.UpdateReadiness(s.network.PeerCount(), s.blockchain.Height())
	})

	tlsEnabled := s.tlsCert != "" && s.tlsKey != ""

	handler := observability.RequestIDMiddleware(
		security.HTTPSRedirectMiddleware(tlsEnabled,
			security.ConnectionLimitMiddleware(connLimiter, getClientID)(
				security.DDoSProtectionMiddleware(ddosDetector, getClientID)(
					security.RateLimitMiddleware(rateLimiter, getClientID)(
						methodRateLimiter.Middleware(getClientID)(
							slowQueryDetector.Middleware(getClientID)(
								observability.ErrorLoggingMiddleware(baseHandler, s.logger),
							),
						),
					),
				),
			),
		),
	)

	handler = security.DrainMiddleware(handler, s.drainer)

	s.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error(fmt.Sprintf("RPC server panic: %v", r))
				fmt.Fprintf(os.Stderr, "RPC SERVER PANIC: %v\n", r)
			}
		}()
		s.logger.WithField("port", s.port).Info("JSON-RPC server starting")
		var err error
		if s.tlsCert != "" && s.tlsKey != "" {
			s.logger.Info("TLS enabled for RPC server")
			err = s.server.ListenAndServeTLS(s.tlsCert, s.tlsKey)
		} else {
			err = s.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			s.logger.Error(fmt.Sprintf("RPC server error: %v", err))
			fmt.Fprintf(os.Stderr, "RPC SERVER CRASHED: %v\n", err)
		} else if err == nil {
			s.logger.Info("JSON-RPC server stopped normally")
		}
	}()

	time.Sleep(100 * time.Millisecond)
	s.logger.WithField("port", s.port).Info("JSON-RPC server started")

	return nil
}

func (s *RPCServer) Stop() error {
	if s.server != nil {
		s.drainer.StartDrain()

		if err := s.drainer.Wait(); err != nil {
			s.logger.WithField("error", err.Error()).Warn("Timeout waiting for RPC connections to drain")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *RPCServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	observability.SetReady("rpc", true)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"height":  s.blockchain.Height(),
		"peers":   s.network.PeerCount(),
		"version": Version,
	})
}

func (s *RPCServer) handleReady(w http.ResponseWriter, r *http.Request) {
	ready := observability.IsReady()
	status := "ready"
	code := http.StatusOK
	if !ready {
		status = "not_ready"
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": status,
		"height": s.blockchain.Height(),
		"peers":  s.network.PeerCount(),
	})

	observability.SetReady("rpc", ready)
}

func (s *RPCServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.ContentLength > 5*1024*1024 {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, nil, -32700, "Parse error")
		return
	}

	if req.JSONRPC != "2.0" {
		s.sendError(w, req.ID, -32600, "Invalid request")
		return
	}

	handler, exists := s.methods[req.Method]
	if !exists {
		s.sendError(w, req.ID, -32601, "Method not found")
		return
	}

	sensitiveMethods := map[string]bool{
		"eth_sendRawTransaction": true,
		"eth_sendUserOperation":  true,
		"debug_traceTransaction": true,
		"viri_addPeer":           true,
		"viri_removePeer":        true,
		"viri_getConsensusState": true,
	}

	if sensitiveMethods[req.Method] && s.apiKeyHash != "" {
		key := security.ExtractAPIKey(r)
		if key == "" {
			s.sendError(w, req.ID, -32000, "missing API key")
			return
		}
		auth := security.NewAPIKeyAuthFromHash(s.apiKeyHash)
		if !auth.IsValid(key) {
			s.sendError(w, req.ID, -32000, "invalid API key")
			return
		}
	}

	result, err := handler(r.Context(), req.Params)
	if err != nil {
		s.sendError(w, req.ID, -32000, err.Error())
		return
	}

	s.sendResult(w, req.ID, result)
}

func (s *RPCServer) sendResult(w http.ResponseWriter, id interface{}, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	})
}

func (s *RPCServer) sendError(w http.ResponseWriter, id interface{}, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
		ID: id,
	})
}

func (s *RPCServer) getBlockNumber(ctx context.Context, params json.RawMessage) (interface{}, error) {
	height := s.blockchain.Height()
	return fmt.Sprintf("0x%x", height), nil
}

func (s *RPCServer) getBlockByNumber(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params")
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("missing block number")
	}

	var height uint64
	switch v := args[0].(type) {
	case string:
		if v == "latest" || v == "pending" {
			height = s.blockchain.Height()
		} else {
			if _, err := fmt.Sscanf(v, "0x%x", &height); err != nil {
				return nil, fmt.Errorf("invalid block number")
			}
		}
	case float64:
		height = uint64(v)
	default:
		return nil, fmt.Errorf("invalid block number type")
	}

	if height > s.blockchain.Height() {
		return nil, fmt.Errorf("block not found")
	}

	block, err := s.blockchain.GetBlock(height)
	if err != nil {
		return nil, fmt.Errorf("block not found")
	}

	return formatBlock(block), nil
}

func (s *RPCServer) getBlockByHash(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params")
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("missing block hash")
	}

	block, err := s.blockchain.GetBlockByHash(args[0])
	if err != nil {
		return nil, fmt.Errorf("block not found")
	}

	return formatBlock(block), nil
}

func (s *RPCServer) getTransactionCount(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params")
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("missing address")
	}

	addrStr, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid address")
	}
	if len(addrStr) >= 2 && addrStr[:2] == "0x" {
		addrStr = addrStr[2:]
	}
	addrBytes, err := hex.DecodeString(addrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid address format")
	}

	blockParam := "latest"
	if len(args) > 1 {
		if bp, ok := args[1].(string); ok {
			blockParam = bp
		}
	}

	nonce, err := s.stateMgr.GetNonce(addrBytes)
	if err != nil {
		nonce = 0
	}

	// For "pending", include txs in the mempool from this sender
	if blockParam == "pending" {
		pendingTxs := s.blockchain.TxPool().GetPendingByAccount(addrBytes)
		for _, ptx := range pendingTxs {
			if ptx.Nonce >= nonce {
				nonce = ptx.Nonce + 1
			}
		}
	}

	return fmt.Sprintf("0x%x", nonce), nil
}

func (s *RPCServer) getBalance(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params")
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("missing address")
	}

	// Decode the hex address and look up real balance from state
	addrStr := args[0]
	if len(addrStr) >= 2 && addrStr[:2] == "0x" {
		addrStr = addrStr[2:]
	}
	addrBytes, err := hex.DecodeString(addrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid address format")
	}

	balance, err := s.stateMgr.GetBalance(addrBytes)
	if err != nil {
		// Account doesn't exist yet — balance is 0
		return "0x0", nil
	}
	return fmt.Sprintf("0x%x", balance), nil
}

func (s *RPCServer) sendRawTransaction(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params")
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("missing transaction data")
	}

	txHex := args[0]
	if len(txHex) >= 2 && txHex[:2] == "0x" {
		txHex = txHex[2:]
	}
	txData, err := hex.DecodeString(txHex)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction hex encoding")
	}

	tx, err := ledger.DeserializeTransaction(txData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode transaction: %w", err)
	}

	if !tx.Verify() {
		return nil, fmt.Errorf("invalid transaction signature")
	}

	txPool := s.blockchain.TxPool()
	if err := txPool.Add(tx); err != nil {
		return nil, fmt.Errorf("failed to add transaction to pool: %w", err)
	}

	txHash := fmt.Sprintf("0x%x", tx.Hash)
	if s.auditLog != nil {
		reqID := observability.RequestIDFromContext(ctx)
		s.auditLog.LogTransaction(reqID, "", txHash)
	}

	if err := s.network.PublishTransaction(txData); err != nil {
		s.logger.Warn(fmt.Sprintf("Failed to broadcast transaction: %v", err))
	}

	return txHash, nil
}

func (s *RPCServer) getChainID(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return fmt.Sprintf("0x%x", s.chainID), nil
}

func (s *RPCServer) getNetVersion(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return fmt.Sprintf("%d", s.chainID), nil
}

func (s *RPCServer) getCoinbase(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if len(s.coinbase) == 0 {
		return "0x0000000000000000000000000000000000000000", nil
	}
	return fmt.Sprintf("0x%x", s.coinbase), nil
}

func (s *RPCServer) getPeerCount(ctx context.Context, params json.RawMessage) (interface{}, error) {
	count := s.network.PeerCount()
	return fmt.Sprintf("0x%x", count), nil
}

func (s *RPCServer) getNodeInfo(ctx context.Context, params json.RawMessage) (interface{}, error) {
	isValidator := s.validator
	if s.engine != nil && s.engine.IsRunning() {
		isValidator = true
	}
	return map[string]interface{}{
		"version":      Version,
		"chain_id":     s.chainID,
		"peer_id":      s.network.ShortPeerID(),
		"full_peer_id": s.network.PeerID().String(),
		"multiaddr":    s.network.FullMultiaddr(),
		"peers":        s.network.PeerCount(),
		"height":       s.blockchain.Height(),
		"listening":    true,
		"validator":    isValidator,
	}, nil
}

func (s *RPCServer) getPeers(ctx context.Context, params json.RawMessage) (interface{}, error) {
	peers := s.network.Peers()
	result := make([]map[string]interface{}, 0, len(peers))
	for _, p := range peers {
		statusStr := "connected"
		if p.Status == 1 {
			statusStr = "disconnected"
		} else if p.Status == 2 {
			statusStr = "banned"
		} else if p.Status == 3 {
			statusStr = "penalized"
		}
		result = append(result, map[string]interface{}{
			"peer_id":   string(p.ID),
			"status":    statusStr,
		})
	}
	return result, nil
}

func (s *RPCServer) getConsensusState(ctx context.Context, params json.RawMessage) (interface{}, error) {
	state, err := s.engine.ExportState()
	if err != nil {
		return map[string]interface{}{
			"height":     s.blockchain.Height(),
			"validators": s.network.PeerCount(),
		}, nil
	}

	var stateMap map[string]interface{}
	if err := json.Unmarshal(state, &stateMap); err != nil {
		return map[string]interface{}{
			"height": s.blockchain.Height(),
		}, nil
	}
	return stateMap, nil
}

func (s *RPCServer) addPeer(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var raw []json.RawMessage
	if params != nil {
		if err := json.Unmarshal(params, &raw); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}
	if len(raw) < 1 {
		return nil, fmt.Errorf("missing multiaddress parameter")
	}

	var multiaddr string
	if err := json.Unmarshal(raw[0], &multiaddr); err != nil {
		return nil, fmt.Errorf("invalid multiaddress: %w", err)
	}

	if err := s.network.ConnectPeer(multiaddr); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, nil
	}

	return map[string]interface{}{
		"success": true,
		"peer":    multiaddr,
	}, nil
}

func (s *RPCServer) getClientVersion(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return fmt.Sprintf("Viri/%s", Version), nil
}

func (s *RPCServer) getGasPrice(ctx context.Context, params json.RawMessage) (interface{}, error) {
	fm := ledger.DefaultFeeMarket()
	return fmt.Sprintf("0x%x", fm.BaseFee()), nil
}

func (s *RPCServer) getTransactionByHash(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("invalid params")
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("missing transaction hash")
	}

	txHashStr := args[0]
	if len(txHashStr) >= 2 && txHashStr[:2] == "0x" {
		txHashStr = txHashStr[2:]
	}

	txHash, err := hex.DecodeString(txHashStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hash format")
	}

	for _, tx := range s.blockchain.TxPool().GetPending() {
		if bytes.Equal(tx.Hash, txHash) {
			return map[string]interface{}{
				"hash":      fmt.Sprintf("0x%x", tx.Hash),
				"nonce":     fmt.Sprintf("0x%x", tx.Nonce),
				"from":      fmt.Sprintf("0x%x", tx.From),
				"to":        fmt.Sprintf("0x%x", tx.To),
				"value":     fmt.Sprintf("0x%x", tx.Value),
				"gas":       fmt.Sprintf("0x%x", tx.GasLimit),
				"gasPrice":  fmt.Sprintf("0x%x", tx.GasPrice),
				"blockHash": nil,
				"status":    "pending",
			}, nil
		}
	}

	entry, err := s.blockchain.GetTransaction(txHash)
	if err == nil {
		block, err := s.blockchain.GetBlock(entry.Height)
		if err == nil && entry.Index < len(block.Transactions) {
			tx := block.Transactions[entry.Index]
			return map[string]interface{}{
				"hash":             fmt.Sprintf("0x%x", tx.Hash),
				"nonce":            fmt.Sprintf("0x%x", tx.Nonce),
				"from":             fmt.Sprintf("0x%x", tx.From),
				"to":               fmt.Sprintf("0x%x", tx.To),
				"value":            fmt.Sprintf("0x%x", tx.Value),
				"gas":              fmt.Sprintf("0x%x", tx.GasLimit),
				"gasPrice":         fmt.Sprintf("0x%x", tx.GasPrice),
				"blockHash":        fmt.Sprintf("0x%x", block.Hash()),
				"blockNumber":      fmt.Sprintf("0x%x", block.Header.Height),
				"transactionIndex": fmt.Sprintf("0x%x", entry.Index),
				"status":           "confirmed",
			}, nil
		}
	}

	return nil, nil
}

func (s *RPCServer) getSyncing(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.syncer == nil || s.syncer.IsComplete() {
		return false, nil
	}

	progress := s.syncer.Progress()
	return map[string]interface{}{
		"starting_block": fmt.Sprintf("0x%x", progress.StartingHeight),
		"current_block":  fmt.Sprintf("0x%x", progress.CurrentHeight),
		"highest_block":  fmt.Sprintf("0x%x", progress.HighestHeight),
		"phase":          progress.Phase,
		"pivot_block":    fmt.Sprintf("0x%x", progress.PivotBlock),
		"eta_seconds":    progress.ETA.Seconds(),
	}, nil
}

func (s *RPCServer) getLogs(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []map[string]interface{}
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, fmt.Errorf("invalid params")
	}
	filter := args[0]

	fromBlock := uint64(0)
	if v, ok := filter["fromBlock"]; ok {
		switch val := v.(type) {
		case string:
			if val == "latest" {
				fromBlock = s.blockchain.Height()
			} else if strings.HasPrefix(val, "0x") {
				fmt.Sscanf(val, "0x%x", &fromBlock)
			}
		case float64:
			fromBlock = uint64(val)
		}
	}

	toBlock := s.blockchain.Height()
	if v, ok := filter["toBlock"]; ok {
		switch val := v.(type) {
		case string:
			if val == "latest" {
				toBlock = s.blockchain.Height()
			} else if strings.HasPrefix(val, "0x") {
				fmt.Sscanf(val, "0x%x", &toBlock)
			}
		case float64:
			toBlock = uint64(val)
		}
	}

	if toBlock-fromBlock > 10000 {
		return nil, fmt.Errorf("block range too large (max 10000)")
	}

	address := ""
	if v, ok := filter["address"]; ok {
		address = v.(string)
	}

	topics := []string{}
	if v, ok := filter["topics"]; ok {
		if topicArr, ok := v.([]interface{}); ok {
			for _, t := range topicArr {
				if t != nil {
					topics = append(topics, t.(string))
				} else {
					topics = append(topics, "")
				}
			}
		}
	}

	var results []FilterLog
	for height := fromBlock; height <= toBlock; height++ {
		block, err := s.blockchain.GetBlock(height)
		if err != nil {
			continue
		}
		for txIdx, tx := range block.Transactions {
			receipt, err := s.blockchain.GetReceipt(tx.Hash)
			if err != nil || receipt == nil || len(receipt.Logs) == 0 {
				continue
			}
			for _, l := range receipt.Logs {
				logAddr := fmt.Sprintf("0x%x", l.Address)
				if address != "" && logAddr != address {
					continue
				}
				if len(topics) > 0 {
					match := true
					for ti, t := range topics {
						if t == "" {
							continue
						}
						if ti >= len(l.Topics) || !strings.EqualFold(t, fmt.Sprintf("0x%x", l.Topics[ti])) {
							match = false
							break
						}
					}
					if !match {
						continue
					}
				}
				logTopics := make([]string, len(l.Topics))
				for i, t := range l.Topics {
					logTopics[i] = fmt.Sprintf("0x%x", t)
				}
				results = append(results, FilterLog{
					Address:     logAddr,
					Topics:      logTopics,
					Data:        fmt.Sprintf("0x%x", l.Data),
					BlockNumber: fmt.Sprintf("0x%x", height),
					BlockHash:   fmt.Sprintf("0x%x", block.Hash()),
					TxHash:      fmt.Sprintf("0x%x", tx.Hash),
					TxIndex:     fmt.Sprintf("0x%x", txIdx),
				})
			}
		}
	}
	return results, nil
}

func (s *RPCServer) newFilter(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []map[string]interface{}
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, fmt.Errorf("invalid params")
	}
	filter := args[0]

	f := &LogFilter{ID: fmt.Sprintf("0x%x", time.Now().UnixNano()), LastPoll: time.Now()}

	if v, ok := filter["fromBlock"]; ok {
		switch val := v.(type) {
		case string:
			if val == "latest" {
				f.FromBlock = s.blockchain.Height()
			} else if strings.HasPrefix(val, "0x") {
				fmt.Sscanf(val, "0x%x", &f.FromBlock)
			}
		case float64:
			f.FromBlock = uint64(val)
		}
	}

	if v, ok := filter["toBlock"]; ok {
		switch val := v.(type) {
		case string:
			if val == "latest" {
				f.ToBlock = s.blockchain.Height()
			} else if strings.HasPrefix(val, "0x") {
				fmt.Sscanf(val, "0x%x", &f.ToBlock)
			}
		case float64:
			f.ToBlock = uint64(val)
		}
	}

	if v, ok := filter["address"]; ok {
		f.Address = v.(string)
	}

	if v, ok := filter["topics"]; ok {
		if topicArr, ok := v.([]interface{}); ok {
			for _, t := range topicArr {
				if t != nil {
					f.Topics = append(f.Topics, t.(string))
				} else {
					f.Topics = append(f.Topics, "")
				}
			}
		}
	}

	s.filters.Store(f.ID, f)
	go func() {
		time.Sleep(5 * time.Minute)
		s.filters.Delete(f.ID)
	}()
	return f.ID, nil
}

func (s *RPCServer) getFilterChanges(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []string
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, fmt.Errorf("invalid params")
	}
	filterID := args[0]

	v, ok := s.filters.Load(filterID)
	if !ok {
		return nil, fmt.Errorf("filter not found")
	}
	f := v.(*LogFilter)
	f.LastPoll = time.Now()

	results := s.queryLogs(f.FromBlock, f.ToBlock, f.Address, f.Topics)
	f.Logs = results
	return results, nil
}

func (s *RPCServer) uninstallFilter(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []string
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, fmt.Errorf("invalid params")
	}
	filterID := args[0]

	_, ok := s.filters.LoadAndDelete(filterID)
	return ok, nil
}

func (s *RPCServer) queryLogs(fromBlock, toBlock uint64, address string, topics []string) []FilterLog {
	var results []FilterLog
	for height := fromBlock; height <= toBlock; height++ {
		block, err := s.blockchain.GetBlock(height)
		if err != nil {
			continue
		}
		for txIdx, tx := range block.Transactions {
			if len(tx.Data) > 0 && address != "" {
				txAddr := fmt.Sprintf("0x%x", tx.To)
				if txAddr == address {
					results = append(results, FilterLog{
						Address:     txAddr,
						Topics:      []string{},
						Data:        fmt.Sprintf("0x%x", tx.Data),
						BlockNumber: fmt.Sprintf("0x%x", height),
						BlockHash:   fmt.Sprintf("0x%x", block.Hash()),
						TxHash:      fmt.Sprintf("0x%x", tx.Hash),
						TxIndex:     fmt.Sprintf("0x%x", txIdx),
					})
				}
			}
		}
	}
	return results
}

func logsToFilterLogs(logs []*ledger.Log, height uint64, blockHash, txHash []byte, txIdx int) []FilterLog {
	var fl []FilterLog
	for _, l := range logs {
		topics := make([]string, len(l.Topics))
		for i, t := range l.Topics {
			topics[i] = fmt.Sprintf("0x%x", t)
		}
		fl = append(fl, FilterLog{
			Address:     fmt.Sprintf("0x%x", l.Address),
			Topics:      topics,
			Data:        fmt.Sprintf("0x%x", l.Data),
			BlockNumber: fmt.Sprintf("0x%x", height),
			BlockHash:   fmt.Sprintf("0x%x", blockHash),
			TxHash:      fmt.Sprintf("0x%x", txHash),
			TxIndex:     fmt.Sprintf("0x%x", txIdx),
		})
	}
	return fl
}

func (s *RPCServer) getTransactionReceipt(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []string
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, fmt.Errorf("invalid params")
	}

	txHashStr := args[0]
	if len(txHashStr) >= 2 && txHashStr[:2] == "0x" {
		txHashStr = txHashStr[2:]
	}
	txHash, err := hex.DecodeString(txHashStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hash format")
	}

	entry, err := s.blockchain.GetTransaction(txHash)
	if err != nil {
		return nil, nil
	}

	block, err := s.blockchain.GetBlock(entry.Height)
	if err != nil {
		return nil, fmt.Errorf("block not found")
	}

	if entry.Index >= len(block.Transactions) {
		return nil, fmt.Errorf("invalid transaction index")
	}
	tx := block.Transactions[entry.Index]

	receipt, err := s.blockchain.GetReceipt(txHash)
	if err != nil {
		receipt = &ledger.Receipt{TxHash: txHash, BlockHeight: entry.Height, GasUsed: tx.GasLimit, Status: 1}
	}

	var logs []FilterLog
	if receipt != nil && len(receipt.Logs) > 0 {
		logs = logsToFilterLogs(receipt.Logs, entry.Height, block.Hash(), txHash, entry.Index)
	}

	result := map[string]interface{}{
		"transactionHash":   fmt.Sprintf("0x%x", tx.Hash),
		"transactionIndex":  fmt.Sprintf("0x%x", entry.Index),
		"blockHash":         fmt.Sprintf("0x%x", block.Hash()),
		"blockNumber":       fmt.Sprintf("0x%x", entry.Height),
		"from":              fmt.Sprintf("0x%x", tx.From),
		"to":                fmt.Sprintf("0x%x", tx.To),
		"gasUsed":           fmt.Sprintf("0x%x", receipt.GasUsed),
		"status":            fmt.Sprintf("0x%x", receipt.Status),
		"logs":              logs,
	}

	// For deploy transactions, compute contractAddress
	if len(tx.To) == 0 && len(tx.Data) > 0 {
		nonceBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(nonceBytes, tx.Nonce)
		contractAddr := crypto.Keccak256(append(tx.SenderAddress(), nonceBytes...))[12:]
		result["contractAddress"] = fmt.Sprintf("0x%x", contractAddr)
	}

	return result, nil
}

func (s *RPCServer) getBlockReceipts(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, fmt.Errorf("invalid params")
	}

	var height uint64
	switch v := args[0].(type) {
	case string:
		if v == "latest" {
			height = s.blockchain.Height()
		} else if strings.HasPrefix(v, "0x") {
			fmt.Sscanf(v, "0x%x", &height)
		}
	case float64:
		height = uint64(v)
	}

	block, err := s.blockchain.GetBlock(height)
	if err != nil {
		return nil, fmt.Errorf("block not found")
	}

	var receipts []map[string]interface{}
	for txIdx, tx := range block.Transactions {
		receipt, err := s.blockchain.GetReceipt(tx.Hash)
		gasUsed := tx.GasLimit
		status := uint8(1)
		var logs []FilterLog
		if err == nil && receipt != nil {
			gasUsed = receipt.GasUsed
			status = receipt.Status
			logs = logsToFilterLogs(receipt.Logs, height, block.Hash(), tx.Hash, txIdx)
		}
		receipts = append(receipts, map[string]interface{}{
			"transactionHash":   fmt.Sprintf("0x%x", tx.Hash),
			"transactionIndex":  fmt.Sprintf("0x%x", txIdx),
			"blockHash":         fmt.Sprintf("0x%x", block.Hash()),
			"blockNumber":       fmt.Sprintf("0x%x", height),
			"from":              fmt.Sprintf("0x%x", tx.From),
			"to":                fmt.Sprintf("0x%x", tx.To),
			"gasUsed":           fmt.Sprintf("0x%x", gasUsed),
			"status":            fmt.Sprintf("0x%x", status),
			"logs":              logs,
		})
	}
	return receipts, nil
}

func (s *RPCServer) call(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var rawArgs []interface{}
	if err := json.Unmarshal(params, &rawArgs); err != nil || len(rawArgs) == 0 {
		return nil, fmt.Errorf("invalid params")
	}
	callArg, ok := rawArgs[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid call args")
	}
	callData := callArg

	toStr := ""
	if v, ok := callData["to"]; ok {
		toStr = v.(string)
	}
	if len(toStr) >= 2 && toStr[:2] == "0x" {
		toStr = toStr[2:]
	}
	to, err := hex.DecodeString(toStr)
	if err != nil {
		return nil, fmt.Errorf("invalid to address")
	}

	data := []byte{}
	if v, ok := callData["data"]; ok {
		dataStr := v.(string)
		if len(dataStr) >= 2 && dataStr[:2] == "0x" {
			dataStr = dataStr[2:]
		}
		data, err = hex.DecodeString(dataStr)
		if err != nil {
			return nil, fmt.Errorf("invalid data")
		}
	}

	from := to
	if v, ok := callData["from"]; ok {
		fromStr := v.(string)
		if len(fromStr) >= 2 && fromStr[:2] == "0x" {
			fromStr = fromStr[2:]
		}
		fromBytes, err := hex.DecodeString(fromStr)
		if err == nil && len(fromBytes) == 20 {
			from = fromBytes
		}
	}

	gasLimit := uint64(5000000)
	if v, ok := callData["gas"]; ok {
		switch val := v.(type) {
		case string:
			gasStr := val
			if len(gasStr) >= 2 && gasStr[:2] == "0x" {
				gasStr = gasStr[2:]
			}
			fmt.Sscanf(gasStr, "%x", &gasLimit)
		case float64:
			gasLimit = uint64(val)
		}
	}

	value := uint64(0)
	if v, ok := callData["value"]; ok {
		switch val := v.(type) {
		case string:
			valStr := val
			if len(valStr) >= 2 && valStr[:2] == "0x" {
				valStr = valStr[2:]
			}
			fmt.Sscanf(valStr, "%x", &value)
		case float64:
			value = uint64(val)
		}
	}

	acct, err := s.stateMgr.GetAccount(to)
	if err != nil || len(acct.Code) == 0 {
		return "0x", nil
	}

	getAccount := func(addr []byte) (*execution.AccountState, error) {
		a, err := s.stateMgr.GetAccount(addr)
		if err != nil {
			return &execution.AccountState{
				Address: addr,
				Balance: new(big.Int),
				Nonce:   0,
				Storage: make(map[string][]byte),
			}, nil
		}
		storage := make(map[string][]byte, len(a.Storage))
		for k, v := range a.Storage {
			storage[k] = v
		}
		return &execution.AccountState{
			Address: a.Address,
			Balance: new(big.Int).Set(a.Balance),
			Nonce:   a.Nonce,
			Code:    a.Code,
			Storage: storage,
		}, nil
	}
	setAccount := func(addr []byte, acct *execution.AccountState) error {
		return nil
	}

	stateAdapter := &evmCallStateAdapter{
		getAccount: getAccount,
		setAccount: setAccount,
	}

	ctx2 := &vm.EVMContext{
		Caller:   from,
		Address:  to,
		Value:    new(big.Int).SetUint64(value),
		GasLimit: gasLimit,
		GasPrice: new(big.Int),
		Data:     data,
	}

	executor := vm.NewEVMExecutor(ctx2, stateAdapter)
	output, _, err := executor.Execute(acct.Code)
	if err != nil {
		return "0x", nil
	}

	return fmt.Sprintf("0x%x", output), nil
}

func (s *RPCServer) estimateGas(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []map[string]interface{}
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, fmt.Errorf("invalid params")
	}
	callData := args[0]

	gas := uint64(21000)
	if v, ok := callData["data"]; ok {
		dataStr := v.(string)
		if len(dataStr) >= 2 && dataStr[:2] == "0x" {
			dataStr = dataStr[2:]
		}
		gas += uint64(len(dataStr)/2) * 100
	}
	return fmt.Sprintf("0x%x", gas), nil
}

func (s *RPCServer) traceTransaction(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, fmt.Errorf("invalid params")
	}
	txHashStr, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid tx hash")
	}
	if len(txHashStr) >= 2 && txHashStr[:2] == "0x" {
		txHashStr = txHashStr[2:]
	}
	txHash, err := hex.DecodeString(txHashStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hash format")
	}

	entry, err := s.blockchain.GetTransaction(txHash)
	if err != nil {
		return nil, fmt.Errorf("transaction not found")
	}

	block, err := s.blockchain.GetBlock(entry.Height)
	if err != nil {
		return nil, fmt.Errorf("block not found")
	}

	if entry.Index >= len(block.Transactions) {
		return nil, fmt.Errorf("invalid transaction index")
	}
	tx := block.Transactions[entry.Index]

	txType := execution.ClassifyTransaction(tx)
	if txType != execution.TxContractCall {
		return nil, fmt.Errorf("only contract calls can be traced")
	}

	code, err := s.stateMgr.GetCode(tx.To)
	if err != nil || len(code) == 0 {
		return nil, fmt.Errorf("contract code not found")
	}

	getAccount := func(addr []byte) (*execution.AccountState, error) {
		a, err := s.stateMgr.GetAccount(addr)
		if err != nil {
			return &execution.AccountState{
				Address: addr,
				Balance: new(big.Int),
				Nonce:   0,
				Storage: make(map[string][]byte),
			}, nil
		}
		storage := make(map[string][]byte, len(a.Storage))
		for k, v := range a.Storage {
			storage[k] = v
		}
		return &execution.AccountState{
			Address: a.Address,
			Balance: new(big.Int).Set(a.Balance),
			Nonce:   a.Nonce,
			Code:    a.Code,
			Storage: storage,
		}, nil
	}
	setAccount := func(addr []byte, acct *execution.AccountState) error {
		return nil
	}

	stateAdapter := &evmCallStateAdapter{
		getAccount: getAccount,
		setAccount: setAccount,
	}

	ctx2 := &vm.EVMContext{
		Caller:   tx.SenderAddress(),
		Address:  tx.To,
		Value:    new(big.Int).SetUint64(tx.Value),
		GasLimit: tx.GasLimit,
		GasPrice: new(big.Int).SetUint64(tx.GasPrice),
		Data:     tx.Data,
	}

	var steps []vm.TraceStep
	executor := vm.NewEVMExecutor(ctx2, stateAdapter)
	executor.SetTraceCallback(func(step vm.TraceStep) {
		steps = append(steps, step)
	})

	_, _, err = executor.Execute(code)
	if err != nil {
		steps = append(steps, vm.TraceStep{
			OpName: "REVERT",
			Error:  err.Error(),
		})
	}

	return steps, nil
}

func (s *RPCServer) getCode(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []string
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, fmt.Errorf("invalid params")
	}

	addrStr := args[0]
	if len(addrStr) >= 2 && addrStr[:2] == "0x" {
		addrStr = addrStr[2:]
	}
	addrBytes, err := hex.DecodeString(addrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid address format")
	}

	if s.stateMgr != nil {
		code, err := s.stateMgr.GetCode(addrBytes)
		if err == nil && code != nil {
			return fmt.Sprintf("0x%x", code), nil
		}
	}
	return "0x", nil
}

func (s *RPCServer) getStorageAt(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []string
	if err := json.Unmarshal(params, &args); err != nil || len(args) < 2 {
		return nil, fmt.Errorf("invalid params")
	}

	addrStr := args[0]
	if len(addrStr) >= 2 && addrStr[:2] == "0x" {
		addrStr = addrStr[2:]
	}
	addrBytes, err := hex.DecodeString(addrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid address format")
	}

	if s.stateMgr != nil {
		account, err := s.stateMgr.GetAccount(addrBytes)
		if err == nil && account != nil && account.IsContract() {
			return "0x0", nil
		}
	}
	return "0x0", nil
}

func formatTx(tx *ledger.Transaction, blockHash []byte, height uint64, txIdx int) map[string]interface{} {
	return map[string]interface{}{
		"hash":             fmt.Sprintf("0x%x", tx.Hash),
		"nonce":            fmt.Sprintf("0x%x", tx.Nonce),
		"blockHash":        fmt.Sprintf("0x%x", blockHash),
		"blockNumber":      fmt.Sprintf("0x%x", height),
		"transactionIndex": fmt.Sprintf("0x%x", txIdx),
		"from":             fmt.Sprintf("0x%x", tx.From),
		"to":               fmt.Sprintf("0x%x", tx.To),
		"value":            fmt.Sprintf("0x%x", tx.Value),
		"gas":              fmt.Sprintf("0x%x", tx.GasLimit),
		"gasPrice":         fmt.Sprintf("0x%x", tx.GasPrice),
		"input":            fmt.Sprintf("0x%x", tx.Data),
	}
}

func formatBlockWithTxs(block *ledger.Block) map[string]interface{} {
	txs := make([]map[string]interface{}, 0, len(block.Transactions))
	blockHash := block.Hash()
	for i, tx := range block.Transactions {
		txs = append(txs, formatTx(tx, blockHash, block.Header.Height, i))
	}

	return map[string]interface{}{
		"number":       fmt.Sprintf("0x%x", block.Header.Height),
		"hash":         fmt.Sprintf("0x%x", blockHash),
		"parentHash":   fmt.Sprintf("0x%x", block.Header.PrevHash),
		"timestamp":    block.Header.Timestamp.Unix(),
		"proposer":     fmt.Sprintf("0x%x", block.Header.Proposer),
		"transactions": txs,
	}
}

func (s *RPCServer) getProtocolVersion(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return "0x41", nil
}

func (s *RPCServer) getHashrate(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return "0x0", nil
}

func (s *RPCServer) getMining(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return false, nil
}

func (s *RPCServer) getBlockTxCountByHash(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []string
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, fmt.Errorf("invalid params")
	}
	block, err := s.blockchain.GetBlockByHash(args[0])
	if err != nil {
		return nil, fmt.Errorf("block not found")
	}
	return fmt.Sprintf("0x%x", len(block.Transactions)), nil
}

func (s *RPCServer) getBlockTxCountByNumber(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil || len(args) == 0 {
		return nil, fmt.Errorf("invalid params")
	}

	var height uint64
	switch v := args[0].(type) {
	case string:
		if v == "latest" || v == "pending" {
			height = s.blockchain.Height()
		} else {
			fmt.Sscanf(v, "0x%x", &height)
		}
	case float64:
		height = uint64(v)
	}

	block, err := s.blockchain.GetBlock(height)
	if err != nil {
		return nil, fmt.Errorf("block not found")
	}
	return fmt.Sprintf("0x%x", len(block.Transactions)), nil
}

func (s *RPCServer) getTxByBlockHashAndIndex(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil || len(args) < 2 {
		return nil, fmt.Errorf("invalid params")
	}
	blockHashStr, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid block hash")
	}
	block, err := s.blockchain.GetBlockByHash(blockHashStr)
	if err != nil {
		return nil, fmt.Errorf("block not found")
	}

	var idx uint64
	switch v := args[1].(type) {
	case string:
		fmt.Sscanf(v, "0x%x", &idx)
	case float64:
		idx = uint64(v)
	}

	if idx >= uint64(len(block.Transactions)) {
		return nil, fmt.Errorf("tx index out of range")
	}

	tx := block.Transactions[idx]
	blockHash := block.Hash()
	return formatTx(tx, blockHash, block.Header.Height, int(idx)), nil
}

func (s *RPCServer) getTxByBlockNumberAndIndex(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var args []interface{}
	if err := json.Unmarshal(params, &args); err != nil || len(args) < 2 {
		return nil, fmt.Errorf("invalid params")
	}

	var height uint64
	switch v := args[0].(type) {
	case string:
		if v == "latest" || v == "pending" {
			height = s.blockchain.Height()
		} else {
			fmt.Sscanf(v, "0x%x", &height)
		}
	case float64:
		height = uint64(v)
	}

	block, err := s.blockchain.GetBlock(height)
	if err != nil {
		return nil, fmt.Errorf("block not found")
	}

	var idx uint64
	switch v := args[1].(type) {
	case string:
		fmt.Sscanf(v, "0x%x", &idx)
	case float64:
		idx = uint64(v)
	}

	if idx >= uint64(len(block.Transactions)) {
		return nil, fmt.Errorf("tx index out of range")
	}

	tx := block.Transactions[idx]
	blockHash := block.Hash()
	return formatTx(tx, blockHash, block.Header.Height, int(idx)), nil
}

func (s *RPCServer) getUncleCountByHash(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return "0x0", nil
}

func (s *RPCServer) getUncleCountByNumber(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return "0x0", nil
}

func formatBlock(block *ledger.Block) map[string]interface{} {
	txs := make([]string, 0, len(block.Transactions))
	for _, tx := range block.Transactions {
		txs = append(txs, fmt.Sprintf("0x%x", tx.Hash))
	}

	return map[string]interface{}{
		"number":       fmt.Sprintf("0x%x", block.Header.Height),
		"hash":         fmt.Sprintf("0x%x", block.Hash()),
		"parentHash":   fmt.Sprintf("0x%x", block.Header.PrevHash),
		"timestamp":    block.Header.Timestamp.Unix(),
		"proposer":     fmt.Sprintf("0x%x", block.Header.Proposer),
		"transactions": txs,
	}
}

func (s *RPCServer) sendUserOperation(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.entryPoint == nil {
		return nil, fmt.Errorf("account abstraction not enabled")
	}

	var rawArgs []map[string]interface{}
	if err := json.Unmarshal(params, &rawArgs); err != nil || len(rawArgs) == 0 {
		return nil, fmt.Errorf("invalid params")
	}

	opMap := rawArgs[0]
	op, err := parseUserOpFromMap(opMap)
	if err != nil {
		return nil, err
	}

	// Validate and execute the operation
	result, err := s.entryPoint.HandleOps([]accounts.UserOperation{*op}, s.coinbase)
	if err != nil {
		return nil, fmt.Errorf("handle op: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no result")
	}

	userOpHash := fmt.Sprintf("0x%x", accounts.UserOpHash(op, s.chainID))

	return map[string]interface{}{
		"userOpHash": userOpHash,
		"success":    result[0].Success,
		"gasUsed":    fmt.Sprintf("0x%x", result[0].GasUsed),
		"returnData": fmt.Sprintf("0x%x", result[0].ReturnData),
	}, nil
}

func (s *RPCServer) estimateUserOperationGas(ctx context.Context, params json.RawMessage) (interface{}, error) {
	if s.entryPoint == nil {
		return nil, fmt.Errorf("account abstraction not enabled")
	}

	var rawArgs []map[string]interface{}
	if err := json.Unmarshal(params, &rawArgs); err != nil || len(rawArgs) == 0 {
		return nil, fmt.Errorf("invalid params")
	}

	opMap := rawArgs[0]
	op, err := parseUserOpFromMap(opMap)
	if err != nil {
		return nil, err
	}

	// Simulate execution to estimate gas
	result, err := s.entryPoint.HandleOps([]accounts.UserOperation{*op}, s.coinbase)
	if err != nil {
		return nil, fmt.Errorf("estimate failed: %w", err)
	}

	gasEstimate := uint64(21000)
	if len(result) > 0 {
		gasEstimate = result[0].GasUsed
	}

	return map[string]interface{}{
		"gasLimit":          fmt.Sprintf("0x%x", gasEstimate),
		"maxFeePerGas":      fmt.Sprintf("0x%x", op.MaxFee),
		"maxPriorityFeePerGas": fmt.Sprintf("0x%x", op.MaxPriorityFee),
		"preVerificationGas":   "0x5208",
	}, nil
}

// parseUserOpFromMap parses a UserOperation from a JSON-RPC parameter map.
func parseUserOpFromMap(m map[string]interface{}) (*accounts.UserOperation, error) {
	op := &accounts.UserOperation{}

	if v, ok := m["sender"]; ok {
		s, _ := v.(string)
		s = strings.TrimPrefix(s, "0x")
		op.Sender, _ = hex.DecodeString(s)
	}
	if v, ok := m["nonce"]; ok {
		n, _ := v.(string)
		n = strings.TrimPrefix(n, "0x")
		fmt.Sscanf(n, "%x", &op.Nonce)
	}
	if v, ok := m["initCode"]; ok {
		s, _ := v.(string)
		s = strings.TrimPrefix(s, "0x")
		op.InitCode, _ = hex.DecodeString(s)
	}
	if v, ok := m["callData"]; ok {
		s, _ := v.(string)
		s = strings.TrimPrefix(s, "0x")
		op.CallData, _ = hex.DecodeString(s)
	}
	if v, ok := m["gasLimit"]; ok {
		s, _ := v.(string)
		s = strings.TrimPrefix(s, "0x")
		fmt.Sscanf(s, "%x", &op.GasLimit)
	}
	if v, ok := m["maxFeePerGas"]; ok {
		s, _ := v.(string)
		s = strings.TrimPrefix(s, "0x")
		fmt.Sscanf(s, "%x", &op.MaxFee)
	}
	if v, ok := m["maxPriorityFeePerGas"]; ok {
		s, _ := v.(string)
		s = strings.TrimPrefix(s, "0x")
		fmt.Sscanf(s, "%x", &op.MaxPriorityFee)
	}
	if v, ok := m["paymaster"]; ok {
		s, _ := v.(string)
		s = strings.TrimPrefix(s, "0x")
		op.Paymaster, _ = hex.DecodeString(s)
	}
	if v, ok := m["signature"]; ok {
		s, _ := v.(string)
		s = strings.TrimPrefix(s, "0x")
		op.Signature, _ = hex.DecodeString(s)
	}

	if len(op.Sender) == 0 {
		return nil, fmt.Errorf("missing sender")
	}

	return op, nil
}

type evmCallStateAdapter struct {
	getAccount func([]byte) (*execution.AccountState, error)
	setAccount func([]byte, *execution.AccountState) error
}

func (s *evmCallStateAdapter) GetNonce(addr []byte) uint64 {
	acct, err := s.getAccount(addr)
	if err != nil || acct == nil {
		return 0
	}
	return acct.Nonce
}

func (s *evmCallStateAdapter) GetBalance(addr []byte) *big.Int {
	acct, err := s.getAccount(addr)
	if err != nil || acct == nil {
		return new(big.Int)
	}
	return acct.Balance
}

func (s *evmCallStateAdapter) GetCode(addr []byte) []byte {
	acct, err := s.getAccount(addr)
	if err != nil || acct == nil {
		return nil
	}
	return acct.Code
}

func (s *evmCallStateAdapter) GetStorage(addr []byte, key []byte) []byte {
	acct, err := s.getAccount(addr)
	if err != nil || acct == nil || acct.Storage == nil {
		return nil
	}
	return acct.Storage[string(key)]
}

func (s *evmCallStateAdapter) SetStorage(addr []byte, key []byte, value []byte) {
	acct, err := s.getAccount(addr)
	if err != nil || acct == nil {
		return
	}
	if acct.Storage == nil {
		acct.Storage = make(map[string][]byte)
	}
	acct.Storage[string(key)] = value
}

func (s *evmCallStateAdapter) Transfer(from, to []byte, amount *big.Int) {}

func (s *evmCallStateAdapter) CreateAccount(addr []byte) {
	acct := &execution.AccountState{
		Address: addr,
		Balance: new(big.Int),
		Nonce:   0,
		Storage: make(map[string][]byte),
	}
	s.setAccount(addr, acct)
}
