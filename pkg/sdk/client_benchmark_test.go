package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkRPCCall(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", Result: "0x1", ID: 1})
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.RPCCall("eth_blockNumber", nil)
	}
}
