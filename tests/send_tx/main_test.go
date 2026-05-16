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
		w.Write([]byte(`{"jsonrpc":"2.0","result":"0xabc","id":1}`))
	}))
	defer srv.Close()

	result := rpcCall(srv.URL, map[string]interface{}{"method": "test", "id": 1})
	if result["result"] != "0xabc" {
		t.Fatalf("expected result 0xabc, got %v", result["result"])
	}
}

func TestRPCCallWithError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"jsonrpc":"2.0","error":{"code":-32000,"message":"some error"},"id":1}`))
	}))
	defer srv.Close()

	result := rpcCall(srv.URL, map[string]interface{}{"method": "test", "id": 1})
	if result["error"] == nil {
		t.Fatal("expected error in response")
	}
	errObj := result["error"].(map[string]interface{})
	if errObj["message"] != "some error" {
		t.Fatalf("expected message 'some error', got %v", errObj["message"])
	}
}

func TestRPCCallContentType(t *testing.T) {
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		w.Write([]byte(`{"jsonrpc":"2.0","result":"ok","id":1}`))
	}))
	defer srv.Close()

	rpcCall(srv.URL, map[string]interface{}{"method": "test", "id": 1})
	if contentType != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestRPCCallRequestBody(t *testing.T) {
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.Write([]byte(`{"jsonrpc":"2.0","result":"ok","id":1}`))
	}))
	defer srv.Close()

	rpcCall(srv.URL, map[string]interface{}{"method": "eth_blockNumber", "params": []interface{}{}, "id": 5})
	if body["method"] != "eth_blockNumber" {
		t.Fatalf("expected method eth_blockNumber, got %v", body["method"])
	}
	if body["id"] != float64(5) {
		t.Fatalf("expected id 5, got %v", body["id"])
	}
}

func TestRPCCallMalformedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`this is not valid json`))
	}))
	defer srv.Close()

	result := rpcCall(srv.URL, map[string]interface{}{"method": "test", "id": 1})
	if result != nil {
		t.Fatal("expected nil result for malformed response")
	}
}
