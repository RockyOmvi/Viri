package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer3/bridge"
	"github.com/viri-chain/viri/internal/layer3/governance"
	"github.com/viri-chain/viri/internal/layer3/intent"
	"github.com/viri-chain/viri/internal/layer3/interop"
)

type rateLimiter struct {
	mu       sync.Mutex
	requests map[string]*tokenBucket
	rate     float64
	burst    int
}

type tokenBucket struct {
	tokens    float64
	lastCheck time.Time
}

func newRateLimiter(rate float64, burst int) *rateLimiter {
	return &rateLimiter{
		requests: make(map[string]*tokenBucket),
		rate:     rate,
		burst:    burst,
	}
}

func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.requests[key]
	if !exists {
		bucket = &tokenBucket{tokens: float64(rl.burst), lastCheck: time.Now()}
		rl.requests[key] = bucket
	}

	now := time.Now()
	elapsed := now.Sub(bucket.lastCheck).Seconds()
	bucket.tokens += elapsed * rl.rate
	if bucket.tokens > float64(rl.burst) {
		bucket.tokens = float64(rl.burst)
	}
	bucket.lastCheck = now

	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}
	return false
}

type L3APIServer struct {
	mu         sync.Mutex
	port       int
	governance *governance.GovernanceDAO
	bridge     *bridge.ChainBridge
	interop    *interop.InteropProtocol
	intent     *intent.IntentSolver
	server     *http.Server
	apiKeys    map[string]bool
	rateLimiter   *rateLimiter
}

func NewL3APIServer(port int, gov *governance.GovernanceDAO, br *bridge.ChainBridge, ip *interop.InteropProtocol, is *intent.IntentSolver) *L3APIServer {
	return &L3APIServer{
		port:        port,
		governance:  gov,
		bridge:      br,
		interop:     ip,
		intent:      is,
		apiKeys:     make(map[string]bool),
		rateLimiter: newRateLimiter(10, 20),
	}
}

func (s *L3APIServer) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v3/governance/proposals", s.handleProposals)
	mux.HandleFunc("/api/v3/governance/vote", s.handleVote)
	mux.HandleFunc("/api/v3/bridge/transfers", s.handleTransfers)
	mux.HandleFunc("/api/v3/bridge/validate", s.handleValidateTransfer)
	mux.HandleFunc("/api/v3/interop/channels", s.handleChannels)
	mux.HandleFunc("/api/v3/interop/packets", s.handlePackets)
	mux.HandleFunc("/api/v3/intents", s.handleIntents)
	mux.HandleFunc("/api/v3/intents/solve", s.handleSolveIntent)
	mux.HandleFunc("/api/v3/health", s.handleHealth)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.corsMiddleware(mux),
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("L3 API server error: %v\n", err)
		}
	}()

	return nil
}

func (s *L3APIServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

func (s *L3APIServer) SetAPIKey(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key != "" {
		s.apiKeys[key] = true
	}
}

func (s *L3APIServer) SetRateLimit(rate float64, burst int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rateLimiter = newRateLimiter(rate, burst)
}

func (s *L3APIServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if len(s.apiKeys) > 0 {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				key = r.URL.Query().Get("api_key")
			}
			s.mu.Lock()
			valid := s.apiKeys[key]
			s.mu.Unlock()
			if !valid {
				s.sendError(w, http.StatusUnauthorized, "invalid or missing API key")
				return
			}
		}

		ip := r.RemoteAddr
		if !s.rateLimiter.Allow(ip) {
			w.Header().Set("Retry-After", "1")
			s.sendError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *L3APIServer) handleProposals(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.URL.Query().Get("id") != "" {
			s.getProposal(w, r)
		} else {
			s.listProposals(w, r)
		}
	case http.MethodPost:
		s.createProposal(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *L3APIServer) listProposals(w http.ResponseWriter, r *http.Request) {
	proposals := s.governance.GetActiveProposals()
	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"proposals": proposals,
		"total":     len(proposals),
	})
}

func (s *L3APIServer) getProposal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID uint64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid JSON request body: "+err.Error())
		return
	}

	proposal, exists := s.governance.GetProposal(req.ID)
	if !exists {
		s.sendError(w, http.StatusNotFound, "proposal not found")
		return
	}

	s.sendJSON(w, http.StatusOK, proposal)
}

func (s *L3APIServer) createProposal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Type        uint8  `json:"type"`
		Proposer    string `json:"proposer"`
		Stake       uint64 `json:"stake"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid JSON request body: "+err.Error())
		return
	}

	proposal, err := s.governance.SubmitProposal(req.Title, req.Description, governance.ProposalType(req.Type), []byte(req.Proposer), req.Stake)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.sendJSON(w, http.StatusCreated, proposal)
}

func (s *L3APIServer) handleVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProposalID uint64 `json:"proposal_id"`
		Voter      string `json:"voter"`
		Choice     uint8  `json:"choice"`
		Stake      uint64 `json:"stake"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid JSON request body: "+err.Error())
		return
	}

	err := s.governance.Vote(req.ProposalID, []byte(req.Voter), governance.VoteChoice(req.Choice), req.Stake)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.sendJSON(w, http.StatusOK, map[string]string{"status": "vote recorded"})
}

func (s *L3APIServer) handleTransfers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		transfers := s.bridge.GetPendingTransfers()
		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"transfers": transfers,
			"total":     len(transfers),
		})
	case http.MethodPost:
		s.initiateTransfer(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *L3APIServer) initiateTransfer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceChain string `json:"source_chain"`
		DestChain   string `json:"dest_chain"`
		Sender      string `json:"sender"`
		Receiver    string `json:"receiver"`
		Amount      uint64 `json:"amount"`
		Token       string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid JSON request body: "+err.Error())
		return
	}

	transfer, err := s.bridge.InitiateTransfer(req.SourceChain, req.DestChain, []byte(req.Sender), []byte(req.Receiver), req.Amount, []byte(req.Token))
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.sendJSON(w, http.StatusCreated, transfer)
}

func (s *L3APIServer) handleValidateTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TransferID  string `json:"transfer_id"`
		ValidatorID string `json:"validator_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid JSON request body: "+err.Error())
		return
	}

	err := s.bridge.AddValidatorSignature(req.TransferID, req.ValidatorID)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.sendJSON(w, http.StatusOK, map[string]string{"status": "signature added"})
}

func (s *L3APIServer) handleChannels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		channels := s.interop.GetActiveChannels()
		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"channels": channels,
			"total":    len(channels),
		})
	case http.MethodPost:
		s.createChannel(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *L3APIServer) createChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PortA   string `json:"port_a"`
		PortB   string `json:"port_b"`
		ChainA  string `json:"chain_a"`
		ChainB  string `json:"chain_b"`
		Version string `json:"version"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid JSON request body: "+err.Error())
		return
	}

	channel, err := s.interop.CreateChannel(req.PortA, req.PortB, req.ChainA, req.ChainB, req.Version)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.sendJSON(w, http.StatusCreated, channel)
}

func (s *L3APIServer) handlePackets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ChannelID string `json:"channel_id"`
		Type      uint8  `json:"type"`
		Data      string `json:"data"`
		Timeout   uint64 `json:"timeout"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid JSON request body: "+err.Error())
		return
	}

	packet, err := s.interop.SendPacket(req.ChannelID, interop.PacketType(req.Type), []byte(req.Data), req.Timeout)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.sendJSON(w, http.StatusCreated, packet)
}

func (s *L3APIServer) handleIntents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		intents := s.intent.GetOpenIntents()
		s.sendJSON(w, http.StatusOK, map[string]interface{}{
			"intents": intents,
			"total":   len(intents),
		})
	case http.MethodPost:
		s.createIntent(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *L3APIServer) createIntent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		User        string  `json:"user"`
		Type        uint8   `json:"type"`
		Input       string  `json:"input"`
		Output      string  `json:"output"`
		MaxSlippage float64 `json:"max_slippage"`
		Deadline    uint64  `json:"deadline"`
		Fee         uint64  `json:"fee"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid JSON request body: "+err.Error())
		return
	}

	intent, err := s.intent.SubmitIntent([]byte(req.User), intent.IntentType(req.Type), []byte(req.Input), []byte(req.Output), req.MaxSlippage, req.Deadline, req.Fee)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.sendJSON(w, http.StatusCreated, intent)
}

func (s *L3APIServer) handleSolveIntent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IntentID string `json:"intent_id"`
		SolverID string `json:"solver_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "invalid JSON request body: "+err.Error())
		return
	}

	result, err := s.intent.SolveIntent(req.IntentID, req.SolverID)
	if err != nil {
		s.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.sendJSON(w, http.StatusOK, result)
}

func (s *L3APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"status": "healthy",
		"layer":  "L3",
	})
}

func (s *L3APIServer) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *L3APIServer) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
