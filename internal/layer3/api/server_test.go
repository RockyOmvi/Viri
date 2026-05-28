package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/viri-chain/viri/internal/layer2/agents"
	"github.com/viri-chain/viri/internal/layer3/appchain"
	"github.com/viri-chain/viri/internal/layer3/bridge"
	"github.com/viri-chain/viri/internal/layer3/governance"
	"github.com/viri-chain/viri/internal/layer3/intent"
	"github.com/viri-chain/viri/internal/layer3/interop"
)

func newTestServer() *L3APIServer {
	gov := governance.NewGovernanceDAO(10*time.Millisecond, 1, 0.5)
	br := bridge.NewChainBridge(1)
	br.RegisterChain("a", "A", "a")
	br.RegisterChain("b", "B", "b")
	br.RegisterValidator("v1")
	ip := interop.NewInteropProtocol()
	is := intent.NewIntentSolver()
	return NewL3APIServer(0, gov, br, ip, is, appchain.NewAppChainManager(), agents.NewAgentManager())
}

func TestStartStop(t *testing.T) {
	s := newTestServer()
	if err := s.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("stop when nil server failed: %v", err)
	}
}

func TestGovernanceHandlers(t *testing.T) {
	s := newTestServer()
	createBody := map[string]interface{}{
		"title":       "t",
		"description": "d",
		"type":        0,
		"proposer":    "p",
		"stake":       2,
	}
	buf, _ := json.Marshal(createBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v3/governance/proposals", bytes.NewReader(buf))
	w := httptest.NewRecorder()
	s.handleProposals(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d", w.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v3/governance/proposals", nil)
	listW := httptest.NewRecorder()
	s.handleProposals(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status: %d", listW.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v3/governance/proposals?id=0", bytes.NewReader([]byte(`{"id":0}`)))
	getW := httptest.NewRecorder()
	s.handleProposals(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("get status: %d", getW.Code)
	}
}

func TestVoteHandler(t *testing.T) {
	s := newTestServer()
	_, _ = s.governance.SubmitProposal("t", "d", governance.ProposalTypeText, []byte("p"), 2)
	body := map[string]interface{}{"proposal_id": 0, "voter": "v", "choice": 0, "stake": 1}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v3/governance/vote", bytes.NewReader(buf))
	w := httptest.NewRecorder()
	s.handleVote(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("vote status: %d", w.Code)
	}
}

func TestBridgeHandlers(t *testing.T) {
	s := newTestServer()
	body := map[string]interface{}{
		"source_chain": "a",
		"dest_chain":   "b",
		"sender":       "s",
		"receiver":     "r",
		"amount":       1,
		"token":        "T",
	}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v3/bridge/transfers", bytes.NewReader(buf))
	w := httptest.NewRecorder()
	s.handleTransfers(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("init transfer status: %d", w.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v3/bridge/transfers", nil)
	listW := httptest.NewRecorder()
	s.handleTransfers(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status: %d", listW.Code)
	}
}

func TestInteropHandlers(t *testing.T) {
	s := newTestServer()
	createBody := map[string]interface{}{
		"port_a":  "pa",
		"port_b":  "pb",
		"chain_a": "a",
		"chain_b": "b",
		"version": "1",
	}
	buf, _ := json.Marshal(createBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v3/interop/channels", bytes.NewReader(buf))
	w := httptest.NewRecorder()
	s.handleChannels(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create channel status: %d", w.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v3/interop/channels", nil)
	listW := httptest.NewRecorder()
	s.handleChannels(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status: %d", listW.Code)
	}
}

func TestIntentHandlers(t *testing.T) {
	s := newTestServer()
	createBody := map[string]interface{}{
		"user":          "u",
		"type":          0,
		"input":         "i",
		"output":        "o",
		"max_slippage":  0.1,
		"deadline":      uint64(time.Now().Add(time.Hour).UnixNano()),
		"fee":           1,
	}
	buf, _ := json.Marshal(createBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v3/intents", bytes.NewReader(buf))
	w := httptest.NewRecorder()
	s.handleIntents(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create intent status: %d", w.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v3/intents", nil)
	listW := httptest.NewRecorder()
	s.handleIntents(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list status: %d", listW.Code)
	}
}

func TestHealthHandler(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v3/health", nil)
	w := httptest.NewRecorder()
	s.handleHealth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("health status: %d", w.Code)
	}
}

func TestAPIKeyAuth(t *testing.T) {
	s := newTestServer()
	s.SetAPIKey("test-key-123")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/health", s.handleHealth)
	handler := s.corsMiddleware(mux)

	t.Run("missing key returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v3/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("invalid key returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v3/health", nil)
		req.Header.Set("X-API-Key", "wrong-key")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	})

	t.Run("valid key in header passes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v3/health", nil)
		req.Header.Set("X-API-Key", "test-key-123")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("valid key in query string passes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v3/health?api_key=test-key-123", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})
}

func TestRateLimiter(t *testing.T) {
	s := newTestServer()
	s.SetRateLimit(1000, 5)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/health", s.handleHealth)
	handler := s.corsMiddleware(mux)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v3/health", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("unexpected rate limit on request %d", i+1)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v3/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 6th request, got %d", w.Code)
	}
}

func TestSetAPIKeyEmpty(t *testing.T) {
	s := newTestServer()
	s.SetAPIKey("")
	s.SetAPIKey("key1")
	s.SetAPIKey("key2")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/health", s.handleHealth)
	handler := s.corsMiddleware(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v3/health", nil)
	req.Header.Set("X-API-Key", "key2")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
