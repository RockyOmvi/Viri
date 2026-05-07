package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

// EVM Bytecode for a simple contract that returns 0x42 (66)
// Init code + Runtime code
const contractBytecode = "600a600c600039600a6000f3604260005260206000f3"

func main() {
	faucetKeyHex := os.Getenv("FAUCET_WALLET_KEY")
	if faucetKeyHex == "" {
		faucetKeyHex = "a4d0b548f43c7034987abda0db71c715c123c1a521a9f53f482e45f0853ea1a2" // Using standard testnet key
	}

	keyBytes, err := hex.DecodeString(faucetKeyHex)
	if err != nil {
		panic(err)
	}

	privKey, err := crypto.PrivateKeyFromBytes(keyBytes)
	if err != nil {
		panic(err)
	}

	address := privKey.PubKey().Address()
	addrHex := "0x" + hex.EncodeToString(address)
	fmt.Printf("Deploying from address: %s\n", addrHex)

	rpcURL := "http://localhost:8545"

	// 1. Get Nonce
	nonceReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getTransactionCount",
		"params":  []interface{}{addrHex, "latest"},
		"id":      1,
	}
	nonceResp := rpcCall(rpcURL, nonceReq)
	nonceHex := nonceResp["result"].(string)
	var nonce uint64
	fmt.Sscanf(nonceHex, "0x%x", &nonce)
	fmt.Printf("Current nonce: %d\n", nonce)

	// 2. Create Deploy Transaction
	code, _ := hex.DecodeString(contractBytecode)
	tx, err := ledger.NewTransactionFromKey(nonce, nil, 0, 500000, 10, code, privKey)
	if err != nil {
		panic(err)
	}

	txData, _ := ledger.SerializeTransaction(tx)
	rawTx := "0x" + hex.EncodeToString(txData)

	// 3. Send Transaction
	sendReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_sendRawTransaction",
		"params":  []interface{}{rawTx},
		"id":      2,
	}
	sendResp := rpcCall(rpcURL, sendReq)
	if sendResp["error"] != nil {
		fmt.Printf("Error sending tx: %v\n", sendResp["error"])
		return
	}
	txHash := sendResp["result"].(string)
	fmt.Printf("Transaction sent! Hash: %s\n", txHash)

	// Poll for transaction receipt (with timeout)
	fmt.Println("Polling for transaction receipt...")
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	receiptFound := false
pollLoop:
	for {
		select {
		case <-timeout:
			fmt.Println("\nTimeout waiting for transaction receipt. Proceeding anyway...")
			break pollLoop
		case <-ticker.C:
			receiptReq := map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "eth_getTransactionReceipt",
				"params":  []interface{}{txHash},
				"id":      3,
			}
			receiptResp := rpcCall(rpcURL, receiptReq)
			if receiptResp["result"] != nil {
				fmt.Printf("\nTransaction receipt found!\n")
				receiptFound = true
				break pollLoop
			}
			fmt.Print(".")
		}
	}
	if !receiptFound {
		fmt.Println()
	}

	// Calculate expected contract address
	nonceBytes := make([]byte, 8)
	// We need to use binary.BigEndian
	importBinary := true
	_ = importBinary
	for i := uint64(0); i < 8; i++ {
		nonceBytes[7-i] = byte(nonce >> (i * 8))
	}
	addrHash := crypto.SHA256(append(address, nonceBytes...))[:20]
	contractAddress := "0x" + hex.EncodeToString(addrHash)
	fmt.Printf("Expected contract deployed at: %s\n", contractAddress)

	// Get Code to verify
	codeReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getCode",
		"params":  []interface{}{contractAddress, "latest"},
		"id":      3,
	}
	codeResp := rpcCall(rpcURL, codeReq)
	if codeResp["result"] != nil {
		fmt.Printf("Code at address: %v\n", codeResp["result"])
	}

	// 5. Call the Contract
	callReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_call",
		"params": []interface{}{
			map[string]interface{}{
				"to": contractAddress,
			},
			"latest",
		},
		"id": 4,
	}
	callResp := rpcCall(rpcURL, callReq)
	if callResp["error"] != nil {
		fmt.Printf("Error calling contract: %v\n", callResp["error"])
		return
	}

	callResult := callResp["result"].(string)
	fmt.Printf("Call result: %s\n", callResult)
}

func rpcCall(url string, reqBody map[string]interface{}) map[string]interface{} {
	reqData, _ := json.Marshal(reqBody)
	resp, err := http.Post(url, "application/json", bytes.NewReader(reqData))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result
}
