package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRPCCallSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jsonrpc":"2.0","result":"0x123","id":1}`))
	}))
	defer srv.Close()

	result := rpcCall(srv.URL, map[string]interface{}{"method": "test", "id": 1})
	if result["result"] != "0x123" {
		t.Fatalf("expected result 0x123, got %v", result["result"])
	}
}

func TestRPCCallError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32000,"message":"err"},"id":1}`))
	}))
	defer srv.Close()

	result := rpcCall(srv.URL, map[string]interface{}{"method": "test", "id": 1})
	if result["error"] == nil {
		t.Fatal("expected error in response")
	}
}

func TestRPCCallServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32000,"message":"internal"},"id":1}`))
	}))
	defer srv.Close()

	result := rpcCall(srv.URL, map[string]interface{}{"method": "test", "id": 1})
	if result["error"] == nil {
		t.Fatal("expected error in server error response")
	}
}

func TestRPCCallInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	result := rpcCall(srv.URL, map[string]interface{}{"method": "test", "id": 1})
	if result != nil {
		t.Fatal("expected nil result for invalid JSON")
	}
}

func TestRPCCallMethod(t *testing.T) {
	var received map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.Write([]byte(`{"jsonrpc":"2.0","result":"ok","id":1}`))
	}))
	defer srv.Close()

	rpcCall(srv.URL, map[string]interface{}{"method": "eth_blockNumber", "id": 2})
	if received["method"] != "eth_blockNumber" {
		t.Fatalf("expected method eth_blockNumber, got %v", received["method"])
	}
	if received["id"] != float64(2) {
		t.Fatalf("expected id 2, got %v", received["id"])
	}
}
