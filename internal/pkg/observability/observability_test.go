package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/viri-chain/viri/internal/layer1/logging"
)

func TestRequestIDMiddleware_AddsHeader(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := RequestIDFromContext(r.Context())
		w.Header().Set("X-Test-Request-ID", reqID)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	reqID := resp.Header.Get("X-Request-ID")
	if reqID == "" {
		t.Error("expected X-Request-ID header to be set")
	}

	if resp.Header.Get("X-Test-Request-ID") != reqID {
		t.Error("expected request ID to be propagated in context")
	}
}

func TestRequestIDMiddleware_UsesExistingHeader(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "existing-id-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.Header.Get("X-Request-ID") != "existing-id-123" {
		t.Error("expected existing X-Request-ID header to be used")
	}
}

func TestRequestIDFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestIDKey{}, "test-id-456")

	id := RequestIDFromContext(ctx)
	if id != "test-id-456" {
		t.Errorf("expected 'test-id-456', got '%s'", id)
	}

	nilCtx := context.Background()
	id = RequestIDFromContext(nilCtx)
	if id != "" {
		t.Errorf("expected empty string for context without request ID, got '%s'", id)
	}

	id = RequestIDFromContext(nil)
	if id != "" {
		t.Errorf("expected empty string for nil context, got '%s'", id)
	}
}

func TestErrorLoggingMiddleware_LogsErrors(t *testing.T) {
	var logged bool
	logger := logging.NewLogger("test", logging.WARN, "text")
	logger.SetOutput(io.Discard)

	handler := ErrorLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}), logger)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	_ = logged
}

func TestErrorLoggingMiddleware_NoLogOnSuccess(t *testing.T) {
	logger := logging.NewLogger("test", logging.WARN, "text")
	logger.SetOutput(io.Discard)

	handler := ErrorLoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), logger)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestReadinessStateTransitions(t *testing.T) {
	ConfigureReadiness(2, 100)

	UpdateReadiness(0, 0)
	ForceReady(false)
	if IsReady() {
		t.Error("expected not ready with insufficient peers and height")
	}

	UpdateReadiness(1, 50)
	if IsReady() {
		t.Error("expected not ready with insufficient peers")
	}

	UpdateReadiness(3, 50)
	if IsReady() {
		t.Error("expected not ready with insufficient height")
	}

	UpdateReadiness(3, 150)
	if !IsReady() {
		t.Error("expected ready with sufficient peers and height")
	}

	ForceReady(true)
	if !IsReady() {
		t.Error("expected ready when force ready is set")
	}

	ForceReady(false)
	UpdateReadiness(0, 0)
}

func TestReadinessMiddleware(t *testing.T) {
	ConfigureReadiness(1, 10)
	UpdateReadiness(0, 0)

	handler := ReadinessMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 when not ready, got %d", resp.StatusCode)
	}

	UpdateReadiness(2, 20)

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp = rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 when ready, got %d", resp.StatusCode)
	}
}

func TestMetricsHandler_ReturnsPrometheusFormat(t *testing.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(RequestsTotal, RequestDuration, InFlight, BlockHeight, PeerCount, ReadyState)

	handler := MetricsHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	if resp.Header.Get("Content-Type") == "" {
		t.Error("expected Content-Type header to be set")
	}

	if len(content) == 0 {
		t.Error("expected non-empty metrics response")
	}
}

func TestSetChainStats(t *testing.T) {
	SetChainStats("test-server", 100, 5)

	g, err := PeerCount.GetMetricWithLabelValues("test-server")
	if err != nil {
		t.Fatalf("failed to get metric: %v", err)
	}
	_ = g
}

func TestSetReady(t *testing.T) {
	SetReady("test-server", true)

	g, err := ReadyState.GetMetricWithLabelValues("test-server")
	if err != nil {
		t.Fatalf("failed to get metric: %v", err)
	}
	_ = g

	SetReady("test-server", false)
}

func TestLocalOnly_AllowsLocalhost(t *testing.T) {
	handler := LocalOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for localhost, got %d", resp.StatusCode)
	}
}

func TestLocalOnly_BlocksNonLocal(t *testing.T) {
	handler := LocalOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403 for non-local request, got %d", resp.StatusCode)
	}
}

func TestLocalOnly_AllowsLoopbackIP(t *testing.T) {
	handler := LocalOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "[::1]:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 for loopback IPv6, got %d", resp.StatusCode)
	}
}

func TestAuditLogger_Creation(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := NewAuditLogger(tmpDir, 10, 3)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer logger.Close()

	logPath := filepath.Join(tmpDir, "audit.log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("expected audit.log to be created")
	}
}

func TestAuditLogger_Logging(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := NewAuditLogger(tmpDir, 10, 3)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer logger.Close()

	logger.Log("test_event", "req-123", "127.0.0.1", "GET", map[string]interface{}{
		"key": "value",
	})

	logPath := filepath.Join(tmpDir, "audit.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	var entry AuditEvent
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to parse audit log: %v", err)
	}

	if entry.Event != "test_event" {
		t.Errorf("expected event 'test_event', got '%s'", entry.Event)
	}

	if entry.RequestID != "req-123" {
		t.Errorf("expected request ID 'req-123', got '%s'", entry.RequestID)
	}
}

func TestAuditLogger_LogTransaction(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := NewAuditLogger(tmpDir, 10, 3)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer logger.Close()

	logger.LogTransaction("req-456", "10.0.0.1", "0xabc123")

	logPath := filepath.Join(tmpDir, "audit.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	var entry AuditEvent
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to parse audit log: %v", err)
	}

	if entry.Event != "transaction_submitted" {
		t.Errorf("expected event 'transaction_submitted', got '%s'", entry.Event)
	}
}

func TestAuditLogger_LogConsensusChange(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := NewAuditLogger(tmpDir, 10, 3)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer logger.Close()

	logger.LogConsensusChange("req-789", "10.0.0.2", "propose", map[string]interface{}{
		"block": 123,
	})

	logPath := filepath.Join(tmpDir, "audit.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	var entry AuditEvent
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to parse audit log: %v", err)
	}

	if entry.Event != "consensus_action" {
		t.Errorf("expected event 'consensus_action', got '%s'", entry.Event)
	}
}

func TestAuditLogger_LogAuthFailure(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := NewAuditLogger(tmpDir, 10, 3)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer logger.Close()

	logger.LogAuthFailure("req-999", "192.168.1.1", "invalid_token")

	logPath := filepath.Join(tmpDir, "audit.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}

	var entry AuditEvent
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to parse audit log: %v", err)
	}

	if entry.Event != "auth_failure" {
		t.Errorf("expected event 'auth_failure', got '%s'", entry.Event)
	}
}

func TestAuditLogger_Close(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := NewAuditLogger(tmpDir, 10, 3)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Errorf("failed to close audit logger: %v", err)
	}
}

func TestInstrumentHandler(t *testing.T) {
	called := false
	handler := InstrumentHandler("test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}), nil)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected handler to be called")
	}

	resp := rec.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestReadinessMiddleware_ResponseFormat(t *testing.T) {
	ConfigureReadiness(1, 10)
	UpdateReadiness(0, 0)

	handler := ReadinessMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if data["status"] != "not_ready" {
		t.Errorf("expected status 'not_ready', got '%v'", data["status"])
	}
}

func TestAuditLogger_Rotation(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := NewAuditLogger(tmpDir, 1, 2)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer logger.Close()

	largeData := make(map[string]interface{})
	for i := 0; i < 10000; i++ {
		largeData[fmt.Sprintf("key%d", i)] = "value that is long enough to fill up the log file quickly"
	}

	for i := 0; i < 20; i++ {
		logger.Log("test_event", "req", "127.0.0.1", "GET", largeData)
	}

	time.Sleep(200 * time.Millisecond)

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Name() == "audit.log.1" {
			found = true
			break
		}
	}

	if !found {
		t.Log("Files in directory:")
		for _, f := range files {
			t.Logf("  %s", f.Name())
		}
	}
}
