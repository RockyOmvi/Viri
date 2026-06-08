package jsonrpc

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Standard JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeServerError    = -32000
)

// Error is a JSON-RPC 2.0 error object.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Response is a JSON-RPC 2.0 response envelope.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// RequestID extracts the request id field from a JSON-RPC request body.
func RequestID(body []byte) interface{} {
	var req struct {
		ID interface{} `json:"id"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}
	return req.ID
}

// CodeForHandlerError maps handler errors to JSON-RPC error codes.
func CodeForHandlerError(err error) int {
	if err == nil {
		return CodeServerError
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.HasPrefix(msg, "invalid"),
		strings.HasPrefix(msg, "missing"),
		strings.HasPrefix(msg, "unknown block"),
		strings.HasPrefix(msg, "block range"):
		return CodeInvalidParams
	default:
		return CodeServerError
	}
}

// WriteError writes a JSON-RPC error response.
func WriteError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Response{
		JSONRPC: "2.0",
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
		ID: id,
	})
}
