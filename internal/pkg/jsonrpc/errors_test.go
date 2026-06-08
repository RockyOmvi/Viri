package jsonrpc

import (
	"errors"
	"testing"
)

func TestCodeForHandlerError(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{errors.New("invalid params"), CodeInvalidParams},
		{errors.New("missing block number"), CodeInvalidParams},
		{errors.New("internal failure"), CodeServerError},
	}
	for _, tc := range tests {
		if got := CodeForHandlerError(tc.err); got != tc.want {
			t.Errorf("CodeForHandlerError(%q) = %d, want %d", tc.err, got, tc.want)
		}
	}
}

func TestRequestID(t *testing.T) {
	id := RequestID([]byte(`{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":42}`))
	if id == nil {
		t.Fatal("expected id 42")
	}
	if f, ok := id.(float64); !ok || f != 42 {
		t.Fatalf("id = %v (%T), want 42", id, id)
	}
}
