package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      interface{}     `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func makeRPCRequest(method string, params interface{}) *http.Request {
	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  mustMarshal(params),
		ID:      1,
	}
	body := mustMarshal(reqBody)
	return httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return json.RawMessage(data)
}

func TestEthBlockNumber(t *testing.T) {
	req := makeRPCRequest("eth_blockNumber", []interface{}{})
	rec := httptest.NewRecorder()

	handleRPCRequest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp JSONRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	result, ok := resp.Result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", resp.Result)
	}

	if result != "0x0" {
		t.Errorf("expected 0x0, got %s", result)
	}
}

func TestEthGetBlockByNumber(t *testing.T) {
	tests := []struct {
		name    string
		params  []interface{}
		wantErr bool
	}{
		{"latest", []interface{}{"latest"}, false},
		{"specific block 0", []interface{}{"0x0"}, false},
		{"numeric zero", []interface{}{float64(0)}, false},
		{"block not found", []interface{}{"0x999"}, true},
		{"invalid block", []interface{}{"invalid"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeRPCRequest("eth_getBlockByNumber", tt.params)
			rec := httptest.NewRecorder()

			handleRPCRequest(rec, req)

			if tt.wantErr {
				var resp JSONRPCResponse
				json.NewDecoder(rec.Body).Decode(&resp)
				if resp.Error == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if rec.Code != http.StatusOK {
					t.Fatalf("expected 200, got %d", rec.Code)
				}
			}
		})
	}
}

func TestInvalidMethod(t *testing.T) {
	req := makeRPCRequest("invalid_method", []interface{}{})
	rec := httptest.NewRecorder()

	handleRPCRequest(rec, req)

	var resp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error == nil {
		t.Error("expected error for invalid method")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()

	handleRPCRequest(rec, req)

	var resp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Error == nil {
		t.Error("expected error for invalid JSON")
	}

	if resp.Error.Code != -32700 {
		t.Errorf("expected error code -32700, got %d", resp.Error.Code)
	}
}

func TestInvalidRequestMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handleRPCRequest(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestLargeRequestBody(t *testing.T) {
	largeBody := make([]byte, 6*1024*1024)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(largeBody))
	rec := httptest.NewRecorder()

	handleRPCRequest(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", rec.Code)
	}
}

func handleRPCRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.ContentLength > 5*1024*1024 {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, nil, -32700, "Parse error")
		return
	}

	if req.JSONRPC != "2.0" {
		sendError(w, req.ID, -32600, "Invalid request")
		return
	}

	handler, exists := getMockHandler(req.Method)
	if !exists {
		sendError(w, req.ID, -32601, "Method not found")
		return
	}

	ctx := r.Context()
	result, err := handler(ctx, req.Params)
	if err != nil {
		sendError(w, req.ID, -32000, err.Error())
		return
	}

	sendResult(w, req.ID, result)
}

func getMockHandler(method string) (func(ctx context.Context, params json.RawMessage) (interface{}, error), bool) {
	handlers := map[string]func(ctx context.Context, params json.RawMessage) (interface{}, error){
		"eth_blockNumber": func(ctx context.Context, params json.RawMessage) (interface{}, error) {
			return "0x0", nil
		},
		"eth_getBlockByNumber": func(ctx context.Context, params json.RawMessage) (interface{}, error) {
			var args []interface{}
			json.Unmarshal(params, &args)
			if len(args) == 0 {
				return nil, fmt.Errorf("missing block number")
			}
			if args[0] == "latest" || args[0] == "0x0" || args[0] == float64(0) {
				return map[string]interface{}{"number": "0x0"}, nil
			}
			return nil, fmt.Errorf("block not found")
		},
	}
	handler, exists := handlers[method]
	return handler, exists
}

func sendResult(w http.ResponseWriter, id interface{}, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	})
}

func sendError(w http.ResponseWriter, id interface{}, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
		ID: id,
	})
}
