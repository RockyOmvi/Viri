package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/viri-chain/viri/internal/layer1/consensus"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
	"github.com/viri-chain/viri/internal/layer1/p2p"
	"github.com/viri-chain/viri/internal/layer1/state"
	"github.com/viri-chain/viri/internal/pkg/security"
)

type AdminServer struct {
	mu         sync.Mutex
	port       int
	blockchain *ledger.PersistentBlockchain
	stateMgr   *state.StateManager
	network    *p2p.ViriNetwork
	engine     *consensus.HotStuffEngine
	logger     *logging.Logger
	server     *http.Server
	apiKeyHash string
	drainer    *security.ConnectionDrainer
	stopChan   chan struct{}
}

func NewAdminServer(port int, bc *ledger.PersistentBlockchain, sm *state.StateManager, net *p2p.ViriNetwork, engine *consensus.HotStuffEngine, log *logging.Logger, apiKeyHash string) *AdminServer {
	return &AdminServer{
		port:       port,
		blockchain: bc,
		stateMgr:   sm,
		network:    net,
		engine:     engine,
		logger:     log,
		apiKeyHash: apiKeyHash,
		drainer:    security.NewConnectionDrainer(30 * time.Second),
		stopChan:   make(chan struct{}),
	}
}

func (s *AdminServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/admin/status", s.handleStatus)
	mux.HandleFunc("/admin/peers", s.handlePeers)
	mux.HandleFunc("/admin/peers/connect", s.handlePeerConnect)
	mux.HandleFunc("/admin/log-level", s.handleLogLevel)
	mux.HandleFunc("/admin/shutdown", s.handleShutdown)
	mux.HandleFunc("/admin/health", s.handleHealth)

	auth := security.NewAPIKeyAuthFromHash(s.apiKeyHash)

	var handler http.Handler = mux
	if s.apiKeyHash != "" {
		handler = auth.Middleware(handler)
	} else {
		s.logger.Warn("Admin API key auth is disabled")
	}

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
		s.logger.WithField("port", s.port).Info("Admin API server started")
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error(fmt.Sprintf("Admin server error: %v", err))
		}
	}()

	return nil
}

func (s *AdminServer) Stop() error {
	if s.server != nil {
		s.drainer.StartDrain()
		s.drainer.Wait()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *AdminServer) StopChan() <-chan struct{} {
	return s.stopChan
}

func (s *AdminServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engineRunning := s.engine != nil && s.engine.IsRunning()
	height := uint64(0)
	if s.blockchain != nil {
		height = s.blockchain.Height()
	}
	peerCount := 0
	if s.network != nil {
		peerCount = s.network.PeerCount()
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":       Version,
		"height":        height,
		"peers":         peerCount,
		"engine_active": engineRunning,
		"uptime_sec":    time.Now().Unix(),
	})
}

func (s *AdminServer) handlePeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.network == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"peers": []interface{}{}, "total": 0})
		return
	}
	peers := s.network.Peers()
	result := make([]map[string]interface{}, 0, len(peers))
	for _, p := range peers {
		result = append(result, map[string]interface{}{
			"peer_id":   string(p.ID),
			"status":    p.Status,
			"height":    p.Height,
			"last_seen": p.LastSeen.Unix(),
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"peers": result,
		"total": len(result),
	})
}

func (s *AdminServer) handlePeerConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Multiaddr string `json:"multiaddr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	if req.Multiaddr == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "multiaddr required"})
		return
	}

	if s.network == nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "network not available"})
		return
	}

	if err := s.network.ConnectPeer(req.Multiaddr); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"peer":    req.Multiaddr,
	})
}

func (s *AdminServer) handleLogLevel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Level string `json:"level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	level := logging.ParseLogLevel(req.Level)
	s.logger.SetLevel(level)

	s.logger.WithField("level", req.Level).Info("Log level changed via admin API")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"level":   req.Level,
	})
}

func (s *AdminServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	peerCount := 0
	if s.network != nil {
		peerCount = s.network.PeerCount()
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"peers":  peerCount,
	})
}

func (s *AdminServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Node shutting down...",
	})

	close(s.stopChan)
	go func() {
		time.Sleep(500 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(syscall.SIGINT)
	}()
}
