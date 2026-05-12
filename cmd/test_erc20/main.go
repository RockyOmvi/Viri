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
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
)

const erc20Bin = "60c0604052600860809081526726b4b72a37b5b2b760c11b60a0525f906100269082610177565b5060408051808201909152600381526226a4a760e91b602082015260019061004e9082610177565b506002805460ff19166012179055348015610067575f5ffd5b5060405161092138038061092183398101604081905261008691610235565b6003819055335f818152600460209081526040808320859055518481527fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef910160405180910390a35061024c565b634e487b7160e01b5f52604160045260245ffd5b600181811c908216806100fc57607f821691505b60208210810361011a57634e487b7160e01b5f52602260045260245ffd5b50919050565b601f821115610172578282111561017257805f5260205f20601f840160051c602085101561014b57505f5b90810190601f840160051c035f5b8181101561016e575f83820155600101610159565b5050505b505050565b81516001600160401b03811115610190576101906100d4565b6101a48161019e84546100e8565b84610120565b6020601f8211600181146101d6575f83156101bf5750848201515b5f19600385901b1c1916600184901b17845561022e565b5f84815260208120601f198516915b8281101561020557878501518255602094850194600190920191016101e5565b508482101561022257868401515f19600387901b60f8161c191681555b505060018360011b0184555b5050505050565b5f60208284031215610245575f5ffd5b5051919050565b6106c8806102595f395ff3fe608060405234801561000f575f5ffd5b5060043610610090575f3560e01c8063313ce56711610063578063313ce567146100ff57806370a082311461011e57806395d89b411461013d578063a9059cbb14610145578063dd62ed3e14610158575f5ffd5b806306fdde0314610094578063095ea7b3146100b257806318160ddd146100d557806323b872dd146100ec575b5f5ffd5b61009c610182565b6040516100a9919061051d565b60405180910390f35b6100c56100c036600461056d565b61020d565b60405190151581526020016100a9565b6100de60035481565b6040519081526020016100a9565b6100c56100fa366004610595565b610279565b60025461010c9060ff1681565b60405160ff90911681526020016100a9565b6100de61012c3660046105cf565b60046020525f908152604090205481565b61009c61042f565b6100c561015336600461056d565b61043c565b6100de6101663660046105ef565b600560209081525f928352604080842090915290825290205481565b5f805461018e90610620565b80601f01602080910402602001604051908101604052809291908181526020018280546101ba90610620565b80156102055780601f106101dc57610100808354040283529160200191610205565b820191905f5260205f20905b8154815290600101906020018083116101e857829003601f168201915b505050505081565b335f8181526005602090815260408083206001600160a01b038716808552925280832085905551919290917f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925906102679086815260200190565b60405180910390a35060015b92915050565b6001600160a01b0383165f9081526005602090815260408083203384529091528120548211156102e95760405162461bcd60e51b8152602060048201526016602482015275696e73756666696369656e7420616c6c6f77616e636560501b60448201526064015b60405180910390fd5b6001600160a01b0384165f908152600460205260409020548211156103475760405162461bcd60e51b8152602060048201526014602482015273696e73756666696369656e742062616c616e636560601b60448201526064016102e0565b6001600160a01b0384165f9081526005602090815260408083203384529091528120805484929061037990849061066c565b90915550506001600160a01b0384165f90815260046020526040812080548492906103a590849061066c565b90915550506001600160a01b0383165f90815260046020526040812080548492906103d190849061067f565b92505081905550826001600160a01b0316846001600160a01b03167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef8460405161041d91815260200190565b60405180910390a35060019392505050565b6001805461018e90610620565b335f908152600460205260408120548211156104915760405162461bcd60e51b8152602060048201526014602482015273696e73756666696369656e742062616c616e636560601b60448201526064016102e0565b335f90815260046020526040812080548492906104af90849061066c565b90915550506001600160a01b0383165f90815260046020526040812080548492906104db90849061067f565b90915550506040518281526001600160a01b0384169033907fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef90602001610267565b602081525f82518060208401528060208501604085015e5f604082850101526040601f19601f83011684010191505092915050565b80356001600160a01b0381168114610568575f5ffd5b919050565b5f5f6040838503121561057e575f5ffd5b61058783610552565b946020939093013593505050565b5f5f5f606084860312156105a7575f5ffd5b6105b084610552565b92506105be60208501610552565b929592945050506040919091013590565b5f602082840312156105df575f5ffd5b6105e882610552565b9392505050565b5f5f60408385031215610600575f5ffd5b61060983610552565b915061061760208401610552565b90509250929050565b600181811c9082168061063457607f821691505b60208210810361065257634e487b7160e01b5f52602260045260245ffd5b50919050565b634e487b7160e01b5f52601160045260245ffd5b8181038181111561027357610273610658565b808201808211156102735761027361065856fea264697066735822122050e241a545e818c2e899dbe03a15f265ef122c73c43ae4596e758357b60a1daf64736f6c63430008230033"

func main() {
	faucetKeyHex := os.Getenv("FAUCET_WALLET_KEY")
	if faucetKeyHex == "" {
		// WARNING: This is a publicly known test key. Do NOT use on any network with real value.
		faucetKeyHex = "a4d0b548f43c7034987abda0db71c715c123c1a521a9f53f482e45f0853ea1a2"
	}

	keyBytes, err := hex.DecodeString(faucetKeyHex)
	if err != nil {
		panic(err)
	}

	privKey, err := crypto.PrivateKeyFromBytes(keyBytes)
	if err != nil {
		panic(err)
	}

	deployer := privKey.PubKey().Address()
	deployerHex := "0x" + hex.EncodeToString(deployer)
	fmt.Printf("=== ERC-20 Deploy & Test ===\n")
	fmt.Printf("Deployer: %s\n", deployerHex)

	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		rpcURL = "http://localhost:8545"
	}

	// 1. Get Nonce
	nonce := getNonce(rpcURL, deployerHex)
	fmt.Printf("Nonce: %d\n", nonce)

	// 2. Build deploy data: init code + constructor args (initialSupply = 10^24)
	initCode, _ := hex.DecodeString(erc20Bin)
	initialSupply := new(big.Int).Mul(big.NewInt(1_000_000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	constructorArgs := pad32(initialSupply.Bytes())
	deployData := append(initCode, constructorArgs...)

	// 3. Create and send deploy transaction
	tx, err := ledger.NewTransactionFromKey(nonce, nil, 0, 1_000_000, 10, deployData, privKey)
	if err != nil {
		panic(err)
	}
	txData, _ := ledger.SerializeTransaction(tx)
	rawTx := "0x" + hex.EncodeToString(txData)

	sendReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_sendRawTransaction",
		"params":  []interface{}{rawTx},
		"id":      1,
	}
	sendResp := rpcCall(rpcURL, sendReq)
	if sendResp["error"] != nil {
		fmt.Printf("Deploy failed: %v\n", sendResp["error"])
		return
	}
	txHash := sendResp["result"].(string)
	fmt.Printf("Deploy tx sent: %s\n", txHash)

	// 4. Poll for receipt
	contractAddr := pollForReceipt(rpcURL, txHash, nonce, deployer)
	fmt.Printf("Contract deployed at: %s\n", contractAddr)

	// 5. Verify code exists
	codeResp := callRPC(rpcURL, "eth_getCode", []interface{}{contractAddr, "latest"}, 3)
	if codeResp["result"] != nil && codeResp["result"].(string) != "0x" {
		fmt.Printf("Contract code verified at %s\n", contractAddr)
	} else {
		fmt.Printf("WARNING: No code at contract address!\n")
	}

	// 6. Test balanceOf(deployer) = initialSupply
	balanceData := "0x70a08231" + hex.EncodeToString(pad32(deployer))
	balResp := callRPC(rpcURL, "eth_call", []interface{}{
		map[string]interface{}{"to": contractAddr, "data": balanceData},
		"latest",
	}, 4)
	if balResp["error"] != nil {
		fmt.Printf("balanceOf call error: %v\n", balResp["error"])
	} else {
		balHex := balResp["result"].(string)
		bal := new(big.Int)
		bal.SetString(balHex[2:], 16)
		fmt.Printf("balanceOf(deployer) = %s (expected %s)\n", bal.String(), initialSupply.String())
		if bal.Cmp(initialSupply) == 0 {
			fmt.Printf("  ✓ Correct!\n")
		} else {
			fmt.Printf("  ✗ MISMATCH!\n")
		}
	}

	// 7. Test transfer 100 tokens to a secondary address
	recipient := make([]byte, 20)
	recipient[19] = 0xBB
	transferAmount := big.NewInt(100)
	transferData := "0xa9059cbb" + hex.EncodeToString(pad32(recipient)) + hex.EncodeToString(pad32(transferAmount.Bytes()))
	transferNonce := getNonce(rpcURL, deployerHex)
	contractAddrBytes, _ := hex.DecodeString(contractAddr[2:])
	tx2, _ := ledger.NewTransactionFromKey(transferNonce, contractAddrBytes, 0, 100000, 10, mustHex(transferData[2:]), privKey)
	tx2Data, _ := ledger.SerializeTransaction(tx2)
	rawTx2 := "0x" + hex.EncodeToString(tx2Data)

	sendReq2 := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_sendRawTransaction",
		"params":  []interface{}{rawTx2},
		"id":      5,
	}
	sendResp2 := rpcCall(rpcURL, sendReq2)
	if sendResp2["error"] != nil {
		fmt.Printf("Transfer failed: %v\n", sendResp2["error"])
	} else {
		txHash2 := sendResp2["result"].(string)
		fmt.Printf("Transfer tx sent: %s\n", txHash2)
		pollForReceipt(rpcURL, txHash2, 0, nil)

		// 8. Verify balances after transfer
		balRespD := callRPC(rpcURL, "eth_call", []interface{}{
			map[string]interface{}{"to": contractAddr, "data": balanceData},
			"latest",
		}, 6)
		if balRespD["result"] != nil {
			balHex := balRespD["result"].(string)
			bal := new(big.Int)
			bal.SetString(balHex[2:], 16)
			expectedDeployer := new(big.Int).Sub(initialSupply, transferAmount)
			fmt.Printf("balanceOf(deployer) after transfer = %s (expected %s)\n", bal.String(), expectedDeployer.String())
			if bal.Cmp(expectedDeployer) == 0 {
				fmt.Printf("  ✓ Correct!\n")
			} else {
				fmt.Printf("  ✗ MISMATCH!\n")
			}
		}

		balDataR := "0x70a08231" + hex.EncodeToString(pad32(recipient))
		balRespR := callRPC(rpcURL, "eth_call", []interface{}{
			map[string]interface{}{"to": contractAddr, "data": balDataR},
			"latest",
		}, 7)
		if balRespR["result"] != nil {
			balHex := balRespR["result"].(string)
			bal := new(big.Int)
			bal.SetString(balHex[2:], 16)
			fmt.Printf("balanceOf(recipient) = %s (expected %s)\n", bal.String(), transferAmount.String())
			if bal.Cmp(transferAmount) == 0 {
				fmt.Printf("  ✓ Correct!\n")
			} else {
				fmt.Printf("  ✗ MISMATCH!\n")
			}
		}

		// 9. Test totalSupply()
		tsData := "0x18160ddd"
		tsResp := callRPC(rpcURL, "eth_call", []interface{}{
			map[string]interface{}{"to": contractAddr, "data": tsData},
			"latest",
		}, 8)
		if tsResp["result"] != nil {
			tsHex := tsResp["result"].(string)
			ts := new(big.Int)
			ts.SetString(tsHex[2:], 16)
			fmt.Printf("totalSupply() = %s (expected %s)\n", ts.String(), initialSupply.String())
			if ts.Cmp(initialSupply) == 0 {
				fmt.Printf("  ✓ Correct!\n")
			} else {
				fmt.Printf("  ✗ MISMATCH!\n")
			}
		}
	}

	fmt.Println("\n=== ERC-20 Test Complete ===")
}

func pad32(data []byte) []byte {
	b := make([]byte, 32)
	copy(b[32-len(data):], data)
	return b
}

func getNonce(rpcURL, addr string) uint64 {
	nonceResp := callRPC(rpcURL, "eth_getTransactionCount", []interface{}{addr, "latest"}, 0)
	nonceHex := nonceResp["result"].(string)
	var nonce uint64
	fmt.Sscanf(nonceHex, "0x%x", &nonce)
	return nonce
}

func pollForReceipt(rpcURL, txHash string, deployNonce uint64, deployer []byte) string {
	fmt.Print("Polling for receipt")
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			fmt.Println("\nTimeout")
			return "0xunknown"
		case <-ticker.C:
			receiptResp := callRPC(rpcURL, "eth_getTransactionReceipt", []interface{}{txHash}, 3)
			if receiptResp["result"] != nil {
				fmt.Println("\nReceipt found!")
				result := receiptResp["result"].(map[string]interface{})
				if contractHex, ok := result["contractAddress"]; ok && contractHex != nil {
					return contractHex.(string)
				}
				if deployer != nil {
					addrHash := crypto.SHA256(append(deployer, bigEndianU64(deployNonce)...))[:20]
					return "0x" + hex.EncodeToString(addrHash)
				}
				return "0xunknown"
			}
			fmt.Print(".")
		}
	}
}

func bigEndianU64(v uint64) []byte {
	b := make([]byte, 8)
	for i := uint64(0); i < 8; i++ {
		b[7-i] = byte(v >> (i * 8))
	}
	return b
}

func mustHex(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}

func callRPC(url, method string, params []interface{}, id int) map[string]interface{} {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      id,
	}
	return rpcCall(url, req)
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
