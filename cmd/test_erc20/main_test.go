package main

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPad32(t *testing.T) {
	tests := []struct {
		input    []byte
		expected string
	}{
		{[]byte{0x01}, "0000000000000000000000000000000000000000000000000000000000000001"},
		{[]byte{}, "0000000000000000000000000000000000000000000000000000000000000000"},
		{big.NewInt(100).Bytes(), "0000000000000000000000000000000000000000000000000000000000000064"},
		{[]byte("hello"), "00000000000000000000000000000000000000000000000000000068656c6c6f"},
	}

	for _, tc := range tests {
		result := pad32(tc.input)
		if hex.EncodeToString(result) != tc.expected {
			t.Fatalf("pad32(%x) = %x, want %s", tc.input, result, tc.expected)
		}
	}
}

func TestPad32Length(t *testing.T) {
	for i := 0; i <= 32; i++ {
		input := make([]byte, i)
		result := pad32(input)
		if len(result) != 32 {
			t.Fatalf("pad32(%d bytes) = %d bytes, expected 32", i, len(result))
		}
	}
}

func TestPad32PreservesRightAlignment(t *testing.T) {
	input := []byte{0xde, 0xad, 0xbe, 0xef}
	result := pad32(input)
	if result[28] != 0xde || result[29] != 0xad || result[30] != 0xbe || result[31] != 0xef {
		t.Fatalf("pad32 did not right-align: %x", result)
	}
}

func TestBigEndianU64(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0000000000000000"},
		{1, "0000000000000001"},
		{255, "00000000000000ff"},
		{256, "0000000000000100"},
		{0xdeadbeefcafe, "0000deadbeefcafe"},
		{0xffffffffffffffff, "ffffffffffffffff"},
	}

	for _, tc := range tests {
		result := bigEndianU64(tc.input)
		if hex.EncodeToString(result) != tc.expected {
			t.Fatalf("bigEndianU64(%d) = %x, want %s", tc.input, result, tc.expected)
		}
	}
}

func TestBigEndianU64Length(t *testing.T) {
	result := bigEndianU64(0x12345678)
	if len(result) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(result))
	}
}

func TestMustHex(t *testing.T) {
	tests := []struct {
		input    string
		expected []byte
	}{
		{"", []byte{}},
		{"ff", []byte{0xff}},
		{"deadbeef", []byte{0xde, 0xad, 0xbe, 0xef}},
		{"00", []byte{0x00}},
	}

	for _, tc := range tests {
		result := mustHex(tc.input)
		if len(result) != len(tc.expected) {
			t.Fatalf("mustHex(%q) length = %d, want %d", tc.input, len(result), len(tc.expected))
		}
		for i := range result {
			if result[i] != tc.expected[i] {
				t.Fatalf("mustHex(%q)[%d] = %x, want %x", tc.input, i, result[i], tc.expected[i])
			}
		}
	}
}

func TestRPCCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"jsonrpc":"2.0","result":"0x123","id":1}`))
	}))
	defer srv.Close()

	result := rpcCall(srv.URL, map[string]interface{}{"method": "test", "id": 1})
	if result["result"] != "0x123" {
		t.Fatalf("expected result 0x123, got %v", result["result"])
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
		t.Fatalf("expected application/json, got %s", contentType)
	}
}

func TestCallRPC(t *testing.T) {
	var receivedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		receivedMethod = req["method"].(string)
		w.Write([]byte(`{"jsonrpc":"2.0","result":"0x","id":1}`))
	}))
	defer srv.Close()

	callRPC(srv.URL, "eth_blockNumber", []interface{}{}, 5)
	if receivedMethod != "eth_blockNumber" {
		t.Fatalf("expected eth_blockNumber, got %s", receivedMethod)
	}
}

func TestGetNonce(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"jsonrpc":"2.0","result":"0x5","id":1}`))
	}))
	defer srv.Close()

	nonce := getNonce(srv.URL, "0xabc")
	if nonce != 5 {
		t.Fatalf("expected nonce 5, got %d", nonce)
	}
}

func TestInitialSupplyCalculation(t *testing.T) {
	initialSupply := new(big.Int).Mul(big.NewInt(1_000_000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	expected := new(big.Int)
	expected.SetString("1000000000000000000000000", 10)
	if initialSupply.Cmp(expected) != 0 {
		t.Fatalf("expected %s, got %s", expected, initialSupply)
	}
}
