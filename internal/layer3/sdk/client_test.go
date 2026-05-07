package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewL3Client(t *testing.T) {
	client := NewL3Client("http://localhost")
	if client.Endpoint == "" || client.HTTPClient == nil {
		t.Fatalf("client not initialized")
	}
}

func TestHealthCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewL3Client(srv.URL)
	ok, err := client.HealthCheck()
	if err != nil || !ok {
		t.Fatalf("health check failed: %v", err)
	}
}

func TestPostJSONError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "bad"})
	}))
	defer srv.Close()

	client := NewL3Client(srv.URL)
	if _, err := client.SubmitProposal("t", "d", 0, "p", 1); err == nil {
		t.Fatalf("expected API error")
	}
}

func TestGetProposalsUnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"proposals": "wrong"})
	}))
	defer srv.Close()

	client := NewL3Client(srv.URL)
	if _, err := client.GetProposals(); err == nil {
		t.Fatalf("expected unexpected response error")
	}
}

func TestGetPendingTransfersUnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"transfers": "wrong"})
	}))
	defer srv.Close()

	client := NewL3Client(srv.URL)
	if _, err := client.GetPendingTransfers(); err == nil {
		t.Fatalf("expected unexpected response error")
	}
}

func TestGetActiveChannelsUnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"channels": "wrong"})
	}))
	defer srv.Close()

	client := NewL3Client(srv.URL)
	if _, err := client.GetActiveChannels(); err == nil {
		t.Fatalf("expected unexpected response error")
	}
}

func TestGetOpenIntentsUnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"intents": "wrong"})
	}))
	defer srv.Close()

	client := NewL3Client(srv.URL)
	if _, err := client.GetOpenIntents(); err == nil {
		t.Fatalf("expected unexpected response error")
	}
}
