package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func withArgs(t *testing.T, args []string, fn func()) {
	old := os.Args
	os.Args = args
	defer func() { os.Args = old }()
	fn()
}

func TestGetRPCURLDefault(t *testing.T) {
	withArgs(t, []string{"virictl"}, func() {
		if url := getRPCURL(); url != defaultRPCURL {
			t.Fatalf("expected default rpc url")
		}
	})
}

func TestGetRPCURLFlag(t *testing.T) {
	withArgs(t, []string{"virictl", "--rpc", "http://example"}, func() {
		if url := getRPCURL(); url != "http://example" {
			t.Fatalf("expected rpc url from flag")
		}
	})
}

func TestRpcCallError(t *testing.T) {
	withArgs(t, []string{"virictl", "--rpc", "http://127.0.0.1:0"}, func() {
		if _, err := rpcCall("test", nil); err == nil {
			t.Fatalf("expected rpc error")
		}
	})
}

func TestRpcCallSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": "ok"})
	}))
	defer srv.Close()

	withArgs(t, []string{"virictl", "--rpc", srv.URL}, func() {
		res, err := rpcCall("test", nil)
		if err != nil || res["result"] != "ok" {
			t.Fatalf("rpc call failed")
		}
	})
}

func TestApiGet(t *testing.T) {
	withArgs(t, []string{"virictl"}, func() {
		if _, err := apiGet("/"); err == nil {
			t.Fatalf("expected api error")
		}
	})
}

func setPassphrase(t *testing.T) {
	t.Helper()
	os.Setenv("VIRI_WALLET_PASSPHRASE", "test-passphrase-12345")
}

func TestWalletCreate(t *testing.T) {
	setPassphrase(t)
	output := captureStdout(t, func() {
		walletCreate()
	})
	if len(output) == 0 {
		t.Fatalf("expected output")
	}
}

func TestWalletExportAndImport(t *testing.T) {
	setPassphrase(t)
	root := t.TempDir()
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldCwd) }()

	if err := os.MkdirAll(filepath.Join(root, ".viri", "wallets"), 0700); err != nil {
		t.Fatalf("failed to create wallet dir: %v", err)
	}

	// Create wallet first
	output := captureStdout(t, func() {
		walletCreate()
	})
	if len(output) == 0 {
		t.Fatalf("expected output")
	}

	withArgs(t, []string{"virictl", "wallet", "export", "keyfile"}, func() {
		walletExport("keyfile")
	})
	keyPath := filepath.Join(root, "keyfile")
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("expected keyfile")
	}

	output = captureStdout(t, func() {
		walletImport(keyPath)
	})
	if len(output) == 0 {
		t.Fatalf("expected output")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	os.Stdout = w

	resultCh := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		resultCh <- buf.String()
	}()

	fn()
	_ = w.Close()
	os.Stdout = old
	out := <-resultCh
	_ = r.Close()
	return out
}
