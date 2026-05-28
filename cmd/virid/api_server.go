package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/viri-chain/viri/cmd/virid/explorer"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
	"github.com/viri-chain/viri/internal/layer1/p2p"
	"github.com/viri-chain/viri/internal/layer1/state"
	"github.com/viri-chain/viri/internal/pkg/observability"
	"github.com/viri-chain/viri/internal/pkg/security"
)

type APIServer struct {
	mu         sync.Mutex
	port       int
	blockchain *ledger.PersistentBlockchain
	stateMgr   *state.StateManager
	network    *p2p.ViriNetwork
	logger     *logging.Logger
	server     *http.Server
	tlsCert    string
	tlsKey     string
	apiKeyHash string
	drainer    *security.ConnectionDrainer
}

func NewAPIServer(port int, bc *ledger.PersistentBlockchain, sm *state.StateManager, net *p2p.ViriNetwork, log *logging.Logger, tlsCert, tlsKey, apiKeyHash string) *APIServer {
	return &APIServer{
		port:       port,
		blockchain: bc,
		stateMgr:   sm,
		network:    net,
		logger:     log,
		tlsCert:    tlsCert,
		tlsKey:     tlsKey,
		apiKeyHash: apiKeyHash,
		drainer:    security.NewConnectionDrainer(30 * time.Second),
	}
}

func (s *APIServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/blocks", s.getBlocks)
	mux.HandleFunc("/api/v1/blocks/", s.getBlock)
	mux.HandleFunc("/api/v1/transactions/", s.getTransaction)
	mux.HandleFunc("/api/v1/accounts/", s.getAccount)
	mux.HandleFunc("/api/v1/peers", s.getPeers)
	mux.HandleFunc("/api/v1/status", s.getStatus)
	mux.HandleFunc("/api/v1/health", s.healthCheck)
	mux.Handle("/metrics", observability.LocalOnly(observability.MetricsHandler()))
	mux.Handle("/explorer/", http.StripPrefix("/explorer/", explorer.Handler()))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/explorer/", http.StatusFound)
	})

	if s.apiKeyHash == "" {
		s.logger.Warn("REST API key auth is disabled")
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

	baseHandler := observability.InstrumentHandler("api", mux, func() {
		observability.SetChainStats("api", s.blockchain.Height(), s.network.PeerCount())
		observability.UpdateReadiness(s.network.PeerCount(), s.blockchain.Height())
	})

	tlsEnabled := s.tlsCert != "" && s.tlsKey != ""

	handler := observability.RequestIDMiddleware(
		security.HTTPSRedirectMiddleware(tlsEnabled,
			security.NewAPIKeyAuthFromHash(s.apiKeyHash).Middleware(
				security.ConnectionLimitMiddleware(connLimiter, getClientID)(
					security.DDoSProtectionMiddleware(ddosDetector, getClientID)(
						security.RateLimitMiddleware(rateLimiter, getClientID)(
							observability.ErrorLoggingMiddleware(
								s.corsMiddleware(baseHandler),
								s.logger,
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
		s.logger.WithField("port", s.port).Info("REST API server started")
		var err error
		if s.tlsCert != "" && s.tlsKey != "" {
			s.logger.Info("TLS enabled for API server")
			err = s.server.ListenAndServeTLS(s.tlsCert, s.tlsKey)
		} else {
			err = s.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			s.logger.Error(fmt.Sprintf("API server error: %v", err))
		}
	}()

	return nil
}

func (s *APIServer) Stop() error {
	if s.server != nil {
		s.drainer.StartDrain()

		if err := s.drainer.Wait(); err != nil {
			s.logger.WithField("error", err.Error()).Warn("Timeout waiting for API connections to drain")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *APIServer) corsMiddleware(next http.Handler) http.Handler {
	allowedOrigins := map[string]bool{
		"http://localhost":      true,
		"http://localhost:3000": true,
		"http://localhost:8080": true,
		"http://127.0.0.1":      true,
		"http://127.0.0.1:3000": true,
		"http://127.0.0.1:8080": true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) getBlocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.ContentLength > 2*1024*1024 {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	if s.blockchain == nil {
		http.Error(w, "blockchain not available", http.StatusServiceUnavailable)
		return
	}
	from := s.blockchain.Height()
	to := s.blockchain.Height()

	if r.URL.Query().Get("from") != "" {
		fmt.Sscanf(r.URL.Query().Get("from"), "%d", &from)
	}
	if r.URL.Query().Get("to") != "" {
		fmt.Sscanf(r.URL.Query().Get("to"), "%d", &to)
	}

	limit := 100
	if r.URL.Query().Get("limit") != "" {
		fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)
	}

	if to-from+1 > uint64(limit) {
		to = from + uint64(limit) - 1
	}

	blocks, err := s.blockchain.GetBlocks(from, to)
	if err != nil {
		s.sendJSON(w, http.StatusNotFound, map[string]string{"error": "no blocks found"})
		return
	}

	result := make([]map[string]interface{}, 0, len(blocks))
	for _, b := range blocks {
		result = append(result, formatBlock(b))
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"blocks": result,
		"total":  len(result),
	})
}

func (s *APIServer) getBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.ContentLength > 2*1024*1024 {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	heightStr := r.URL.Path[len("/api/v1/blocks/"):]
	if heightStr == "" {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "missing block height"})
		return
	}

	var height uint64
	if _, err := fmt.Sscanf(heightStr, "%d", &height); err != nil {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid block height"})
		return
	}

	if s.blockchain == nil {
		s.sendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "blockchain not available"})
		return
	}
	block, err := s.blockchain.GetBlock(height)
	if err != nil {
		s.sendJSON(w, http.StatusNotFound, map[string]string{"error": "block not found"})
		return
	}

	s.sendJSON(w, http.StatusOK, formatBlock(block))
}

func (s *APIServer) getTransaction(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength > 2*1024*1024 {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	txHashStr := r.URL.Path[len("/api/v1/transactions/"):]
	if txHashStr == "" {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "missing transaction hash"})
		return
	}

	if strings.HasPrefix(txHashStr, "0x") {
		txHashStr = txHashStr[2:]
	}

	txHash, err := hex.DecodeString(txHashStr)
	if err != nil {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid transaction hash"})
		return
	}

	if s.blockchain == nil || s.blockchain.TxPool() == nil {
		s.sendJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "blockchain not available"})
		return
	}
	for _, tx := range s.blockchain.TxPool().GetPending() {
		if bytes.Equal(tx.Hash, txHash) {
			s.sendJSON(w, http.StatusOK, map[string]interface{}{
				"hash":   fmt.Sprintf("0x%x", tx.Hash),
				"nonce":  tx.Nonce,
				"from":   fmt.Sprintf("0x%x", tx.SenderAddress()),
				"to":     fmt.Sprintf("0x%x", tx.To),
				"value":  tx.Value,
				"status": "pending",
			})
			return
		}
	}

	entry, err := s.blockchain.GetTransaction(txHash)
	if err == nil {
		block, err := s.blockchain.GetBlock(entry.Height)
		if err == nil && entry.Index < len(block.Transactions) {
			tx := block.Transactions[entry.Index]
			s.sendJSON(w, http.StatusOK, map[string]interface{}{
				"hash":         fmt.Sprintf("0x%x", tx.Hash),
				"nonce":        tx.Nonce,
				"from":         fmt.Sprintf("0x%x", tx.SenderAddress()),
				"to":           fmt.Sprintf("0x%x", tx.To),
				"value":        tx.Value,
				"gas_limit":    tx.GasLimit,
				"gas_price":    tx.GasPrice,
				"block_height": block.Header.Height,
				"tx_index":     entry.Index,
				"status":       "confirmed",
			})
			return
		}
	}

	height := s.blockchain.Height()
	searchFrom := uint64(0)
	if height > 100 {
		searchFrom = height - 100
	}

	for h := height; h >= searchFrom; h-- {
		block, err := s.blockchain.GetBlock(h)
		if err != nil {
			continue
		}
		for _, tx := range block.Transactions {
			if bytes.Equal(tx.Hash, txHash) {
				s.sendJSON(w, http.StatusOK, map[string]interface{}{
					"hash":         fmt.Sprintf("0x%x", tx.Hash),
					"nonce":        tx.Nonce,
					"from":         fmt.Sprintf("0x%x", tx.SenderAddress()),
					"to":           fmt.Sprintf("0x%x", tx.To),
					"value":        tx.Value,
					"gas_limit":    tx.GasLimit,
					"gas_price":    tx.GasPrice,
					"block_height": block.Header.Height,
				})
				return
			}
		}
		if h == 0 {
			break
		}
	}

	s.sendJSON(w, http.StatusNotFound, map[string]string{"error": "transaction not found"})
}

func (s *APIServer) getAccount(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength > 2*1024*1024 {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	addrStr := r.URL.Path[len("/api/v1/accounts/"):]
	if addrStr == "" {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "missing address"})
		return
	}

	if strings.HasPrefix(addrStr, "0x") {
		addrStr = addrStr[2:]
	}

	addrBytes, err := hex.DecodeString(addrStr)
	if err != nil {
		s.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid address format"})
		return
	}

	account, err := s.stateMgr.GetAccount(addrBytes)
	if err == nil && account != nil {
		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"address":  fmt.Sprintf("0x%x", account.Address),
			"balance":  account.Balance.String(),
			"nonce":    account.Nonce,
			"type":     account.Type,
			"has_code": len(account.Code) > 0,
		})
		return
	}

	balance, _ := s.stateMgr.GetBalance(addrBytes)
	nonce, _ := s.stateMgr.GetNonce(addrBytes)

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"address":  fmt.Sprintf("0x%x", addrBytes),
		"balance":  balance.String(),
		"nonce":    nonce,
		"type":     0,
		"has_code": false,
	})
}

func (s *APIServer) getPeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.network == nil {
		s.sendJSON(w, http.StatusOK, map[string]interface{}{"peers": []interface{}{}, "total": 0})
		return
	}
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
			"last_seen": p.LastSeen.Unix(),
		})
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"peers": result,
		"total": len(result),
	})
}

func (s *APIServer) getStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := s.network.Stats().Snapshot()

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"version":    Version,
		"height":     s.blockchain.Height(),
		"tip":        fmt.Sprintf("0x%x", s.blockchain.TipHash()),
		"peers":      stats.CurrentPeers,
		"blocks_in":  stats.TotalBlocksIn,
		"blocks_out": stats.TotalBlocksOut,
		"txs_in":     stats.TotalTxsIn,
		"txs_out":    stats.TotalTxsOut,
		"uptime":     stats.Uptime.String(),
	})
}

func (s *APIServer) healthCheck(w http.ResponseWriter, r *http.Request) {
	observability.SetReady("api", true)
	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"status": "healthy",
		"height": s.blockchain.Height(),
		"peers":  s.network.PeerCount(),
	})
}

func (s *APIServer) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
