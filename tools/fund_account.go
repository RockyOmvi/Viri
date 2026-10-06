package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./tools/fund_account.go <recipient_address> [amount_in_viri]")
		return
	}

	recipientHex := strings.TrimPrefix(os.Args[1], "0x")
	recipientBytes, err := hex.DecodeString(recipientHex)
	if err != nil || len(recipientBytes) != 20 {
		fmt.Fprintf(os.Stderr, "Invalid recipient 20-byte address: %v\n", err)
		os.Exit(1)
	}

	amountViri := uint64(50)
	if len(os.Args) >= 3 {
		if n, err := strconv.ParseUint(os.Args[2], 10, 64); err == nil {
			amountViri = n
		}
	}

	valKeyHex := "e0c6a3a0b5b297e5ea86409765363f71b26f3fa2e0e61f439e7e6e589839f290"
	valKeyBytes, _ := hex.DecodeString(valKeyHex)
	privKey, err := crypto.PrivateKeyFromBytes(valKeyBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load validator private key: %v\n", err)
		os.Exit(1)
	}

	senderAddr := "0x" + hex.EncodeToString(privKey.PubKey().Address())

	rpcURL := "http://127.0.0.1:8545"
	nonceReq := fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_getTransactionCount","params":["%s","latest"],"id":1}`, senderAddr)
	resp, err := http.Post(rpcURL, "application/json", strings.NewReader(nonceReq))
	if err != nil {
		fmt.Fprintf(os.Stderr, "RPC connection failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var nonceResp struct {
		Result string `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&nonceResp)

	var nonce uint64
	fmt.Sscanf(nonceResp.Result, "0x%x", &nonce)

	value := amountViri * 1_000_000_000_000_000_000
	gasLimit := uint64(50000)
	gasPrice := uint64(1_000_000_000)
	chainID := uint64(1337)

	tx, err := ledger.NewTransactionFromKey(nonce, recipientBytes, value, gasLimit, gasPrice, nil, chainID, privKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create tx: %v\n", err)
		os.Exit(1)
	}

	txBytes, err := ledger.SerializeTransaction(tx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to serialize tx: %v\n", err)
		os.Exit(1)
	}

	rawTxHex := "0x" + hex.EncodeToString(txBytes)

	sendReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_sendRawTransaction",
		"params":  []string{rawTxHex},
		"id":      2,
	}
	reqBody, _ := json.Marshal(sendReq)

	sendResp, err := http.Post(rpcURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Send tx request failed: %v\n", err)
		os.Exit(1)
	}
	defer sendResp.Body.Close()

	var rpcRes map[string]interface{}
	json.NewDecoder(sendResp.Body).Decode(&rpcRes)

	if errVal, ok := rpcRes["error"]; ok {
		fmt.Printf("ERROR sending funds: %v\n", errVal)
	} else {
		fmt.Printf("SUCCESS! Sent %d VIRI from %s to 0x%s\nTxHash: %v\n", amountViri, senderAddr, recipientHex, rpcRes["result"])
	}
}
