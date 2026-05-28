package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

// FaucetServer provides a testnet token faucet with rate limiting.
type FaucetServer struct {
	mu          sync.Mutex
	port        int
	rpcURL      string
	walletKey   *crypto.PrivateKey
	perClaim    uint64
	dailyLimit  uint64
	cooldown    time.Duration
	globalRate  time.Duration
	claims      map[string]time.Time // address -> last claim time
	ipClaims    map[string]time.Time // ip -> last claim time
	dailyTotal  uint64
	dailyReset  time.Time
	nextNonce   uint64
	nonceInit   bool
	server      *http.Server
	tlsCert     string
	tlsKey      string
}

func NewFaucetServer(port int, rpcURL string, key *crypto.PrivateKey, perClaim, dailyLimit uint64, cooldown time.Duration, tlsCert, tlsKey string) *FaucetServer {
	return &FaucetServer{
		port:       port,
		rpcURL:     rpcURL,
		walletKey:  key,
		perClaim:   perClaim,
		dailyLimit: dailyLimit,
		cooldown:   cooldown,
		globalRate: time.Second,
		claims:     make(map[string]time.Time),
		ipClaims:   make(map[string]time.Time),
		dailyReset: time.Now().Add(24 * time.Hour),
		tlsCert:    tlsCert,
		tlsKey:     tlsKey,
	}
}

func (f *FaucetServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", f.handleIndex)
	mux.HandleFunc("/api/claim", f.handleClaim)
	mux.HandleFunc("/api/info", f.handleInfo)

	f.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", f.port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	fmt.Printf("Faucet address: 0x%x\n", f.walletKey.PubKey().Address())
	fmt.Printf("Per claim: %d | Daily limit: %d | Cooldown: %s\n", f.perClaim, f.dailyLimit, f.cooldown)

	var err error
	if f.tlsCert != "" && f.tlsKey != "" {
		fmt.Printf("Faucet running at https://localhost:%d (TLS)\n", f.port)
		err = f.server.ListenAndServeTLS(f.tlsCert, f.tlsKey)
	} else {
		fmt.Printf("Faucet running at http://localhost:%d\n", f.port)
		err = f.server.ListenAndServe()
	}
	return err
}

func (f *FaucetServer) rpcCall(method string, params []interface{}) (interface{}, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}
	reqData, _ := json.Marshal(reqBody)

	resp, err := http.Post(f.rpcURL, "application/json", bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("RPC error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if errObj, exists := result["error"]; exists && errObj != nil {
		return nil, fmt.Errorf("RPC error: %v", errObj)
	}

	return result["result"], nil
}

type ClaimResponse struct {
	Success    bool   `json:"success"`
	TxHash     string `json:"tx_hash,omitempty"`
	TokenTxHash string `json:"token_tx_hash,omitempty"`
	Amount     string `json:"amount,omitempty"`
	Error      string `json:"error,omitempty"`
	Wait       string `json:"wait,omitempty"`
}

var erc20TokenAddr []byte

func init() {
	var err error
	erc20TokenAddr, err = hex.DecodeString("00000000000000000000000000000000000000E0")
	if err != nil {
		panic("failed to decode ERC-20 token address: " + err.Error())
	}
}

func pad32(data []byte) []byte {
	b := make([]byte, 32)
	copy(b[32-len(data):], data)
	return b
}

func erc20TransferData(to []byte, amount *big.Int) []byte {
	selector := []byte{0xa9, 0x05, 0x9c, 0xbb}
	data := append(selector, pad32(to)...)
	return append(data, pad32(amount.Bytes())...)
}

func (f *FaucetServer) handleClaim(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(200)
		return
	}

	if r.Method != "POST" {
		json.NewEncoder(w).Encode(ClaimResponse{Error: "POST required"})
		return
	}

	var req struct {
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(ClaimResponse{Error: "Invalid request body"})
		return
	}

	addr := strings.TrimSpace(req.Address)
	if addr == "" {
		json.NewEncoder(w).Encode(ClaimResponse{Error: "Address is required"})
		return
	}

	// Normalize address
	if strings.HasPrefix(addr, "0x") {
		addr = addr[2:]
	}

	if len(addr) < 20 {
		json.NewEncoder(w).Encode(ClaimResponse{Error: "Invalid address format"})
		return
	}

	addrBytes, err := hex.DecodeString(addr)
	if err != nil {
		json.NewEncoder(w).Encode(ClaimResponse{Error: "Invalid hex address"})
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Reset daily counter
	if time.Now().After(f.dailyReset) {
		f.dailyTotal = 0
		f.dailyReset = time.Now().Add(24 * time.Hour)
	}

	// Check daily limit
	if f.dailyTotal+f.perClaim > f.dailyLimit {
		json.NewEncoder(w).Encode(ClaimResponse{Error: "Daily faucet limit reached. Try again tomorrow."})
		return
	}

	// Global rate limit: at most one claim per globalRate
	now := time.Now()
	if f.globalRate > 0 && !f.dailyReset.IsZero() {
		lastGlobal := f.dailyReset // reuse dailyReset as approximate global timestamp
		if since := now.Sub(lastGlobal); since < f.globalRate {
			wait := f.globalRate - since
			json.NewEncoder(w).Encode(ClaimResponse{
				Error: "Too many requests. Please wait.",
				Wait:  wait.Round(time.Millisecond).String(),
			})
			return
		}
	}

	// IP-based rate limit: max 1 claim per 30s per IP
	clientIP := r.RemoteAddr
	if lastIP, exists := f.ipClaims[clientIP]; exists {
		if time.Since(lastIP) < 30*time.Second {
			json.NewEncoder(w).Encode(ClaimResponse{
				Error: "Too many requests from this IP. Please wait.",
				Wait:  (30*time.Second - time.Since(lastIP)).Round(time.Second).String(),
			})
			return
		}
	}

	// Per-address cooldown
	normalizedAddr := strings.ToLower(addr)
	if lastClaim, exists := f.claims[normalizedAddr]; exists {
		remaining := f.cooldown - time.Since(lastClaim)
		if remaining > 0 {
			json.NewEncoder(w).Encode(ClaimResponse{
				Error: fmt.Sprintf("Please wait before claiming again"),
				Wait:  remaining.Round(time.Second).String(),
			})
			return
		}
	}

	// Get nonce for the faucet wallet — use local counter with initial RPC fetch
	f.mu.Lock()
	if !f.nonceInit {
		faucetAddr := hex.EncodeToString(f.walletKey.PubKey().Address())
		if result, err := f.rpcCall("eth_getTransactionCount", []interface{}{"0x" + faucetAddr, "latest"}); err != nil {
			f.mu.Unlock()
			json.NewEncoder(w).Encode(ClaimResponse{Error: "Failed to get nonce: " + err.Error()})
			return
		} else {
			nonceHex, ok := result.(string)
			if !ok {
				f.mu.Unlock()
				json.NewEncoder(w).Encode(ClaimResponse{Error: "Invalid nonce response"})
				return
			}
			if _, err := fmt.Sscanf(nonceHex, "0x%x", &f.nextNonce); err != nil {
				f.mu.Unlock()
				json.NewEncoder(w).Encode(ClaimResponse{Error: "Failed to parse nonce: " + err.Error()})
				return
			}
		}
		f.nonceInit = true
	}
	nonce := f.nextNonce
	f.nextNonce++
	f.mu.Unlock()

	// Create and sign the native VIRI transfer
	tx, err := ledger.NewTransactionFromKey(nonce, addrBytes, f.perClaim, 21000, 1, nil, uint64(1), f.walletKey)
	if err != nil {
		json.NewEncoder(w).Encode(ClaimResponse{Error: "Failed to create transaction: " + err.Error()})
		return
	}

	txBytes, err := ledger.SerializeTransaction(tx)
	if err != nil {
		json.NewEncoder(w).Encode(ClaimResponse{Error: "Failed to serialize transaction: " + err.Error()})
		return
	}

	// Submit native transfer via RPC
	result, err := f.rpcCall("eth_sendRawTransaction", []interface{}{"0x" + hex.EncodeToString(txBytes)})
	if err != nil {
		json.NewEncoder(w).Encode(ClaimResponse{Error: "Failed to send transaction: " + err.Error()})
		return
	}

	txHash := fmt.Sprintf("%v", result)

	// Also send ERC-20 VIRI token transfer
	tokenAmount := new(big.Int).SetUint64(f.perClaim)
	transferData := erc20TransferData(addrBytes, tokenAmount)
	tokenTx, err := ledger.NewTransactionFromKey(nonce+1, erc20TokenAddr, 0, 100000, 1, transferData, uint64(1), f.walletKey)
	var tokenTxHash string
	if err == nil {
		tokenTxBytes, err := ledger.SerializeTransaction(tokenTx)
		if err == nil {
			result2, err := f.rpcCall("eth_sendRawTransaction", []interface{}{"0x" + hex.EncodeToString(tokenTxBytes)})
			if err == nil {
				tokenTxHash = fmt.Sprintf("%v", result2)
			}
		}
	}

	// Record the claim
	f.claims[normalizedAddr] = time.Now()
	f.ipClaims[clientIP] = time.Now()
	f.dailyTotal += f.perClaim

	json.NewEncoder(w).Encode(ClaimResponse{
		Success:     true,
		TxHash:      txHash,
		TokenTxHash: tokenTxHash,
		Amount:      fmt.Sprintf("%d", f.perClaim),
	})
}

func (f *FaucetServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	f.mu.Lock()
	info := map[string]interface{}{
		"faucet_address": fmt.Sprintf("0x%x", f.walletKey.PubKey().Address()),
		"per_claim":      f.perClaim,
		"daily_limit":    f.dailyLimit,
		"daily_used":     f.dailyTotal,
		"cooldown":       f.cooldown.String(),
	}
	f.mu.Unlock()

	// Get faucet balance
	faucetAddr := hex.EncodeToString(f.walletKey.PubKey().Address())
	if result, err := f.rpcCall("eth_getBalance", []interface{}{"0x" + faucetAddr, "latest"}); err == nil {
		info["balance"] = result
	}

	json.NewEncoder(w).Encode(info)
}

func (f *FaucetServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, _ := template.New("faucet").Parse(faucetHTML)

	data := map[string]interface{}{
		"FaucetAddress": fmt.Sprintf("0x%x", f.walletKey.PubKey().Address()),
		"PerClaim":      f.perClaim,
		"Cooldown":      f.cooldown.String(),
		"NetworkName":   "Viri Testnet",
	}

	tmpl.Execute(w, data)
}

// RunFaucet starts the faucet service standalone.
func RunFaucet() {
	port := 8081
	if p := os.Getenv("FAUCET_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	rpcURL := os.Getenv("VIRI_RPC_URL")
	if rpcURL == "" {
		rpcURL = "http://validator-0:8545"
	}

	// TLS support
	tlsCert := os.Getenv("VIRI_TLS_CERT")
	tlsKey := os.Getenv("VIRI_TLS_KEY")

	// Load faucet wallet key
	keyHex := os.Getenv("FAUCET_WALLET_KEY")
	if keyHex == "" {
		fmt.Fprintln(os.Stderr, "FAUCET_WALLET_KEY env var is required")
		fmt.Fprintln(os.Stderr, "Set it to the hex-encoded private key for the faucet wallet")
		os.Exit(1)
	}

	if strings.HasPrefix(keyHex, "0x") {
		keyHex = keyHex[2:]
	}

	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid faucet key: %v\n", err)
		os.Exit(1)
	}

	key, err := crypto.PrivateKeyFromBytes(keyBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid faucet key bytes: %v\n", err)
		os.Exit(1)
	}

	perClaim := uint64(10_000_000_000_000_000_000) // 10 tokens
	if v := os.Getenv("FAUCET_PER_CLAIM"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			perClaim = n
		}
	}

	dailyLimit := uint64(100_000_000_000_000_000_00) // 100 tokens total daily
	if v := os.Getenv("FAUCET_DAILY_LIMIT"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			dailyLimit = n
		}
	}

	cooldown := 24 * time.Hour
	if v := os.Getenv("FAUCET_COOLDOWN"); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
			cooldown = time.Duration(secs) * time.Second
		}
	}

	fmt.Printf("Viri Faucet v%s\n", Version)
	fmt.Printf("RPC: %s | Port: %d\n", rpcURL, port)

	faucet := NewFaucetServer(port, rpcURL, key, perClaim, dailyLimit, cooldown, tlsCert, tlsKey)
	if err := faucet.Start(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Faucet error: %v\n", err)
		os.Exit(1)
	}
}

// ============================================================
// Embedded Faucet HTML
// ============================================================

const faucetHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Viri Testnet Faucet</title>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600;700;800&family=Fira+Code:wght@400;500&display=swap" rel="stylesheet">
  <style>
    :root {
      --bg: #0a0e17;
      --bg2: #111827;
      --card: #1a2332;
      --text: #e2e8f0;
      --text2: #94a3b8;
      --accent: #6366f1;
      --accent2: #818cf8;
      --success: #10b981;
      --error: #ef4444;
      --border: #1e293b;
    }
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body {
      font-family: 'Inter', sans-serif;
      background: var(--bg);
      color: var(--text);
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
    }
    .faucet-card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 16px;
      padding: 2.5rem;
      width: 100%;
      max-width: 480px;
      text-align: center;
    }
    .faucet-card h1 {
      font-size: 2rem;
      font-weight: 800;
      background: linear-gradient(135deg, var(--accent2), var(--success));
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      margin-bottom: 0.5rem;
    }
    .faucet-card .subtitle {
      color: var(--text2);
      font-size: 0.9rem;
      margin-bottom: 2rem;
    }
    .faucet-card input {
      width: 100%;
      padding: 0.875rem 1rem;
      background: var(--bg2);
      border: 1px solid var(--border);
      border-radius: 10px;
      color: var(--text);
      font-size: 0.9rem;
      font-family: 'Fira Code', monospace;
      outline: none;
      transition: border-color 0.2s;
      margin-bottom: 1rem;
    }
    .faucet-card input:focus { border-color: var(--accent); }
    .faucet-card button {
      width: 100%;
      padding: 0.875rem;
      background: linear-gradient(135deg, var(--accent), #4f46e5);
      color: white;
      border: none;
      border-radius: 10px;
      font-size: 1rem;
      font-weight: 700;
      cursor: pointer;
      transition: all 0.2s;
    }
    .faucet-card button:hover { transform: translateY(-2px); box-shadow: 0 8px 25px rgba(99,102,241,0.3); }
    .faucet-card button:disabled { opacity: 0.5; cursor: not-allowed; transform: none; }
    .info {
      display: flex;
      justify-content: space-between;
      padding: 0.75rem 0;
      border-bottom: 1px solid rgba(255,255,255,0.05);
      font-size: 0.85rem;
    }
    .info .label { color: var(--text2); }
    .info .value { font-family: 'Fira Code', monospace; }
    .result {
      margin-top: 1.5rem;
      padding: 1rem;
      border-radius: 10px;
      font-size: 0.85rem;
      display: none;
    }
    .result.success { background: rgba(16,185,129,0.1); border: 1px solid rgba(16,185,129,0.3); color: #6ee7b7; display: block; }
    .result.error { background: rgba(239,68,68,0.1); border: 1px solid rgba(239,68,68,0.3); color: #fca5a5; display: block; }
    .result a { color: var(--accent2); word-break: break-all; }
    .drip-emoji { font-size: 3rem; margin-bottom: 1rem; }
  </style>
</head>
<body>
  <div class="faucet-card">
    <div class="drip-emoji">💧</div>
    <h1>Viri Faucet</h1>
    <p class="subtitle">{{.NetworkName}} — Get free testnet tokens</p>

    <input type="text" id="address" placeholder="0x... your wallet address">
    <button id="claim-btn" onclick="claim()">Request Tokens</button>

    <div id="result" class="result"></div>

    <div style="margin-top: 1.5rem;">
      <div class="info">
        <span class="label">Per Claim</span>
        <span class="value">{{.PerClaim}} wei</span>
      </div>
      <div class="info">
        <span class="label">Cooldown</span>
        <span class="value">{{.Cooldown}}</span>
      </div>
      <div class="info">
        <span class="label">Faucet Address</span>
        <span class="value" style="font-size:0.7rem">{{.FaucetAddress}}</span>
      </div>
    </div>
  </div>

  <script>
  async function claim() {
    var addr = document.getElementById('address').value.trim();
    if (!addr) { showResult('error', 'Please enter a wallet address'); return; }
    var btn = document.getElementById('claim-btn');
    btn.disabled = true;
    btn.textContent = 'Sending...';
    try {
      var resp = await fetch('/api/claim', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ address: addr })
      });
      var data = await resp.json();
      if (data.success) {
        var msg = 'Tokens sent!<br>Native TX: <a href="/tx/' + data.tx_hash + '">' + data.tx_hash + '</a>';
        if (data.token_tx_hash) {
          msg += '<br>Token TX: <a href="/tx/' + data.token_tx_hash + '">' + data.token_tx_hash + '</a>';
        }
        showResult('success', msg);
      } else {
        var msg = data.error;
        if (data.wait) msg += ' (wait ' + data.wait + ')';
        showResult('error', msg);
      }
    } catch(e) {
      showResult('error', 'Network error: ' + e.message);
    }
    btn.disabled = false;
    btn.textContent = 'Request Tokens';
  }
  function showResult(type, msg) {
    var el = document.getElementById('result');
    el.className = 'result ' + type;
    el.innerHTML = msg;
  }
  </script>
</body>
</html>`
