package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

func main() {
	rpcURL := "http://localhost:8545"
	// WARNING: This is a publicly known test key. Do NOT use on any network with real value.
	faucetKeyHex := "a4d0b548f43c7034987abda0db71c715c123c1a521a9f53f482e45f0853ea1a2"

	keyBytes, _ := hex.DecodeString(faucetKeyHex)
	privKey, _ := crypto.PrivateKeyFromBytes(keyBytes)

	address := privKey.PubKey().Address()
	fmt.Printf("From address: 0x%x\n", address)

	// Get nonce
	nonceResp := rpcCall(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "method": "eth_getTransactionCount",
		"params": []interface{}{fmt.Sprintf("0x%x", address), "latest"}, "id": 1,
	})
	fmt.Printf("Nonce response: %v\n", nonceResp)

	var nonce uint64
	fmt.Sscanf(nonceResp["result"].(string), "0x%x", &nonce)
	fmt.Printf("Nonce: %d\n", nonce)

	// Create transfer to self
	tx, err := ledger.NewTransactionFromKey(nonce, address, 100, 100000, 10, nil, privKey)
	if err != nil {
		panic(err)
	}
	txData, err := ledger.SerializeTransaction(tx)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Tx data length: %d bytes\n", len(txData))
	fmt.Printf("Tx hash: 0x%x\n", tx.Hash)

	// Send
	rawTx := "0x" + hex.EncodeToString(txData)
	sendResp := rpcCall(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "method": "eth_sendRawTransaction",
		"params": []interface{}{rawTx}, "id": 2,
	})
	fmt.Printf("Send response: %v\n", sendResp)

	if sendResp["error"] != nil {
		fmt.Printf("Error: %v\n", sendResp["error"])
		return
	}

	txHash := sendResp["result"].(string)
	fmt.Printf("Transaction hash: %s\n", txHash)

	// Wait for receipt
	timeout := time.After(15 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			fmt.Println("Timeout waiting for receipt")
			return
		case <-ticker.C:
			receiptResp := rpcCall(rpcURL, map[string]interface{}{
				"jsonrpc": "2.0", "method": "eth_getTransactionReceipt",
				"params": []interface{}{txHash}, "id": 3,
			})
			if receiptResp["result"] != nil {
				fmt.Printf("Receipt: %v\n", receiptResp)
				return
			}
			fmt.Print(".")
		}
	}
}

func rpcCall(url string, reqBody map[string]interface{}) map[string]interface{} {
	reqData, _ := json.Marshal(reqBody)
	resp, err := http.Post(url, "application/json", bytes.NewReader(reqData))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result
}
