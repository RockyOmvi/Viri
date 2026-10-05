package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	rpcAPIKey   string
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
	chainID     uint64
	server      *http.Server
	tlsCert     string
	tlsKey      string
}

func NewFaucetServer(port int, rpcURL, rpcAPIKey string, key *crypto.PrivateKey, perClaim, dailyLimit uint64, cooldown time.Duration, tlsCert, tlsKey string) *FaucetServer {
	return &FaucetServer{
		port:       port,
		rpcURL:     rpcURL,
		rpcAPIKey:  rpcAPIKey,
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
	// Detect chain ID from RPC
	if res, err := f.rpcCall("eth_chainId", nil); err == nil {
		if cidHex, ok := res.(string); ok {
			fmt.Sscanf(cidHex, "0x%x", &f.chainID)
		}
	}
	if f.chainID == 0 {
		f.chainID = 99997
	}
	fmt.Printf("Chain ID: %d\n", f.chainID)

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

	req, err := http.NewRequest("POST", f.rpcURL, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("RPC request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if f.rpcAPIKey != "" {
		req.Header.Set("X-API-Key", f.rpcAPIKey)
	}

	resp, err := http.DefaultClient.Do(req)
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

	// Get nonce for the faucet wallet (mutex already held)
	if !f.nonceInit {
		faucetAddr := hex.EncodeToString(f.walletKey.PubKey().Address())
		if result, err := f.rpcCall("eth_getTransactionCount", []interface{}{"0x" + faucetAddr, "latest"}); err != nil {
			json.NewEncoder(w).Encode(ClaimResponse{Error: "Failed to get nonce: " + err.Error()})
			return
		} else {
			nonceHex, ok := result.(string)
			if !ok {
				json.NewEncoder(w).Encode(ClaimResponse{Error: "Invalid nonce response"})
				return
			}
			if _, err := fmt.Sscanf(nonceHex, "0x%x", &f.nextNonce); err != nil {
				json.NewEncoder(w).Encode(ClaimResponse{Error: "Failed to parse nonce: " + err.Error()})
				return
			}
		}
		f.nonceInit = true
	}
	nonce := f.nextNonce
	f.nextNonce++

	// Create and sign the native VIRI transfer
	tx, err := ledger.NewTransactionFromKey(nonce, addrBytes, f.perClaim, 50000, 1, nil, f.chainID, f.walletKey)
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

	// Send ERC-20 VIRI test tokens (0x...E0) so user can immediately test multi-token gas
	tokenTxHash := ""
	tokenNonce := f.nextNonce
	f.nextNonce++
	tokenAmount := new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	tokenData := erc20TransferData(addrBytes, tokenAmount)
	if tokenTx, err := ledger.NewTransactionFromKey(tokenNonce, erc20TokenAddr, 0, 100000, 1, tokenData, f.chainID, f.walletKey); err == nil {
		if tokenTxBytes, err := ledger.SerializeTransaction(tokenTx); err == nil {
			if tokenRes, err := f.rpcCall("eth_sendRawTransaction", []interface{}{"0x" + hex.EncodeToString(tokenTxBytes)}); err == nil {
				tokenTxHash = fmt.Sprintf("%v", tokenRes)
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
	fmt.Fprintf(w, `<html><body style="font-family:monospace;background:#020408;color:#e8f4f8;padding:40px">
<h1>Viri Faucet</h1>
<p>Faucet API is available at /api/claim</p>
<p>Faucet address: 0x%x</p>
</body></html>`, f.walletKey.PubKey().Address())
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

	dailyLimit := uint64(18_000_000_000_000_000_000) // 18 tokens total daily (uint64 max bound)
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

	rpcAPIKey := os.Getenv("FAUCET_RPC_API_KEY")

	fmt.Printf("Viri Faucet v%s\n", Version)
	fmt.Printf("RPC: %s | Port: %d\n", rpcURL, port)

	faucet := NewFaucetServer(port, rpcURL, rpcAPIKey, key, perClaim, dailyLimit, cooldown, tlsCert, tlsKey)
	if err := faucet.Start(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Faucet error: %v\n", err)
		os.Exit(1)
	}
}


