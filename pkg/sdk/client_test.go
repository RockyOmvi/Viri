package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClientEndpoints(t *testing.T) {
	client := NewClient("http://localhost:8545")
	if client.RPCEndpoint == "" || client.APIEndpoint == "" {
		t.Fatalf("endpoints not set")
	}
	if client.APIEndpoint == client.RPCEndpoint {
		t.Fatalf("expected different api endpoint")
	}
}

func TestHexHelpers(t *testing.T) {
	bytes, err := HexToBytes("0x0a0b")
	if err != nil || len(bytes) != 2 {
		t.Fatalf("hex to bytes failed")
	}
	if BytesToHex(bytes) != "0x0a0b" {
		t.Fatalf("bytes to hex mismatch")
	}
}

func TestRPCCallError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", Error: "bad", ID: 1})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.RPCCall("test", nil); err == nil {
		t.Fatalf("expected rpc error")
	}
}

func TestGetBlockNumberUnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", Result: 5, ID: 1})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.GetBlockNumber(); err == nil {
		t.Fatalf("expected unexpected response error")
	}
}

func TestGetBlockByNumberUnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", Result: "bad", ID: 1})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.GetBlockByNumber(1); err == nil {
		t.Fatalf("expected block not found")
	}
}

func TestGetBalanceUnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", Result: 10, ID: 1})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.GetBalance("0x0"); err == nil {
		t.Fatalf("expected unexpected response")
	}
}

func TestGetPeersUnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", Result: "bad", ID: 1})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.GetPeers(); err == nil {
		t.Fatalf("expected unexpected response")
	}
}

func TestGetNodeInfoUnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", Result: "bad", ID: 1})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	if _, err := client.GetNodeInfo(); err == nil {
		t.Fatalf("expected unexpected response")
	}
}

func TestGetStatusAndHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		case "/api/v1/health":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	client.APIEndpoint = srv.URL
	status, err := client.GetStatus()
	if err != nil || status["ok"] != true {
		t.Fatalf("status failed")
	}
	ok, err := client.HealthCheck()
	if err != nil || !ok {
		t.Fatalf("health failed")
	}
}

func TestGetBlocksUnexpectedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"blocks": "bad"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	client.APIEndpoint = srv.URL
	if _, err := client.GetBlocks(0, 1, 10); err == nil {
		t.Fatalf("expected unexpected response")
	}
}
