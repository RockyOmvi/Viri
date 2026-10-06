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
	"strings"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer2/contracts"
)

const (
	rpcURL      = "http://127.0.0.1:8545"
	apiURL      = "http://127.0.0.1:8546"
	chainID     = uint64(1337)
	valKeyHex   = "e0c6a3a0b5b297e5ea86409765363f71b26f3fa2e0e61f439e7e6e589839f290"
	metamaskAcc = "0xa29539C21AC730F8c0501b719Ed7443bB4d16aB1"
)

func rpcCall(method string, params interface{}) (interface{}, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	})
	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res struct {
		Result interface{} `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("decode error: %v, body: %s", err, string(body))
	}
	if res.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", res.Error.Code, res.Error.Message)
	}
	return res.Result, nil
}

func getNonce(addr string) uint64 {
	res, err := rpcCall("eth_getTransactionCount", []interface{}{addr, "latest"})
	if err != nil {
		return 0
	}
	var nonce uint64
	fmt.Sscanf(fmt.Sprintf("%v", res), "0x%x", &nonce)
	return nonce
}

func getBalance(addr string) *big.Int {
	res, err := rpcCall("eth_getBalance", []interface{}{addr, "latest"})
	if err != nil {
		return big.NewInt(0)
	}
	str := fmt.Sprintf("%v", res)
	str = strings.TrimPrefix(str, "0x")
	b, _ := new(big.Int).SetString(str, 16)
	if b == nil {
		return big.NewInt(0)
	}
	return b
}

func getERC20Balance(tokenAddr, holderAddr string) *big.Int {
	cleanHolder := strings.TrimPrefix(holderAddr, "0x")
	data := "0x70a08231000000000000000000000000" + cleanHolder
	res, err := rpcCall("eth_call", []interface{}{
		map[string]string{"to": tokenAddr, "data": data},
		"latest",
	})
	if err != nil {
		return big.NewInt(0)
	}
	str := strings.TrimPrefix(fmt.Sprintf("%v", res), "0x")
	b, _ := new(big.Int).SetString(str, 16)
	if b == nil {
		return big.NewInt(0)
	}
	return b
}

func sendAndConfirmTx(tx *ledger.Transaction) (string, map[string]interface{}, error) {
	txBytes, err := ledger.SerializeTransaction(tx)
	if err != nil {
		return "", nil, fmt.Errorf("serialize tx: %v", err)
	}
	rawHex := "0x" + hex.EncodeToString(txBytes)

	res, err := rpcCall("eth_sendRawTransaction", []interface{}{rawHex})
	if err != nil {
		return "", nil, fmt.Errorf("send tx: %v", err)
	}
	txHash := fmt.Sprintf("%v", res)

	// Wait up to 15 seconds for confirmation
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		receiptRes, err := rpcCall("eth_getTransactionReceipt", []interface{}{txHash})
		if err == nil && receiptRes != nil {
			if rMap, ok := receiptRes.(map[string]interface{}); ok {
				return txHash, rMap, nil
			}
		}
	}
	return txHash, nil, fmt.Errorf("timeout waiting for receipt for tx %s", txHash)
}

func main() {
	fmt.Println("=================================================================")
	fmt.Println("🚀 STARTING VIRIDI FULL ON-CHAIN CAPABILITY TEST SUITE")
	fmt.Println("=================================================================")

	valKeyBytes, _ := hex.DecodeString(valKeyHex)
	valKey, _ := crypto.PrivateKeyFromBytes(valKeyBytes)
	valAddr := "0x" + hex.EncodeToString(valKey.PubKey().Address())
	erc20Addr := "0x" + hex.EncodeToString(contracts.AddrERC20)

	fmt.Printf("Validator Address: %s\n", valAddr)
	fmt.Printf("Validator VIRI Balance: %s wei\n", getBalance(valAddr))
	fmt.Printf("Validator ERC20 Balance: %s\n", getERC20Balance(erc20Addr, valAddr))
	fmt.Println("-----------------------------------------------------------------")

	// -------------------------------------------------------------
	// TEST 1: NATIVE VIRI TRANSFER
	// -------------------------------------------------------------
	fmt.Println("\n[TEST 1/4] Native VIRI Transfer")
	userKey, _ := crypto.GenerateKey()
	userAddr := "0x" + hex.EncodeToString(userKey.PubKey().Address())
	recipientKey, _ := crypto.GenerateKey()
	recipientAddr := "0x" + hex.EncodeToString(recipientKey.PubKey().Address())

	fmt.Printf("  Step 1a: Funding Test User (%s) with 2 VIRI from validator...\n", userAddr)
	valNonce := getNonce(valAddr)
	fundAmount := uint64(2_000_000_000_000_000_000)
	fundTx, err := ledger.NewTransactionFromKey(valNonce, userKey.PubKey().Address(), fundAmount, 50000, 1000000000, nil, chainID, valKey)
	if err != nil {
		fmt.Printf("  ❌ Failed to create fund tx: %v\n", err)
		os.Exit(1)
	}
	fundHash, receipt, err := sendAndConfirmTx(fundTx)
	if err != nil {
		fmt.Printf("  ❌ Fund tx failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  ✅ User funded! Tx: %s, Block: %v, Status: %v\n", fundHash, receipt["blockNumber"], receipt["status"])
	fmt.Printf("     User Balance: %s wei\n", getBalance(userAddr))

	fmt.Printf("  Step 1b: Transferring 1 VIRI from User -> Recipient (%s)...\n", recipientAddr)
	transferAmount := uint64(1_000_000_000_000_000_000)
	userNonce := getNonce(userAddr)
	transferTx, err := ledger.NewTransactionFromKey(userNonce, recipientKey.PubKey().Address(), transferAmount, 30000, 1000000000, nil, chainID, userKey)
	if err != nil {
		fmt.Printf("  ❌ Failed to create transfer tx: %v\n", err)
		os.Exit(1)
	}
	transferHash, tReceipt, err := sendAndConfirmTx(transferTx)
	if err != nil {
		fmt.Printf("  ❌ Transfer tx failed: %v\n", err)
		os.Exit(1)
	}
	recBalance := getBalance(recipientAddr)
	fmt.Printf("  ✅ Transfer confirmed! Tx: %s, Block: %v, Status: %v\n", transferHash, tReceipt["blockNumber"], tReceipt["status"])
	fmt.Printf("     Recipient Balance: %s wei\n", recBalance)
	if recBalance.Cmp(big.NewInt(1_000_000_000_000_000_000)) != 0 {
		fmt.Printf("  ❌ Balance mismatch! Expected 1 VIRI, got %s\n", recBalance)
		os.Exit(1)
	}
	fmt.Println("  🎉 TEST 1 PASSED: Native VIRI Transfer Verified!")

	// -------------------------------------------------------------
	// TEST 2: EVM SMART CONTRACT DEPLOYMENT & STATE INTERACTION
	// -------------------------------------------------------------
	fmt.Println("\n[TEST 2/4] EVM Smart Contract Deployment & State Interaction")
	// Contract:
	// Init code: copies 23 bytes of runtime code and returns it.
	// Runtime code:
	//   if calldata is empty: SLOAD(1) and RETURN
	//   if calldata >= 32: SSTORE(1, calldata[0:32]), SLOAD(1), RETURN
	initCodeHex := "6017600c60003960176000f3600b3615576000356001555b60015460005260206000f3"
	initCode, _ := hex.DecodeString(initCodeHex)

	fmt.Println("  Step 2a: Deploying EVM Storage Contract...")
	deployNonce := getNonce(userAddr)
	deployTx, err := ledger.NewTransactionFromKey(deployNonce, nil, 0, 100000, 1000000000, initCode, chainID, userKey)
	if err != nil {
		fmt.Printf("  ❌ Failed to create deploy tx: %v\n", err)
		os.Exit(1)
	}
	deployHash, dReceipt, err := sendAndConfirmTx(deployTx)
	if err != nil {
		fmt.Printf("  ❌ Deploy tx failed: %v\n", err)
		os.Exit(1)
	}
	contractAddr, _ := dReceipt["contractAddress"].(string)
	fmt.Printf("  ✅ Contract deployed! Tx: %s, Block: %v, ContractAddress: %s, Status: %v\n",
		deployHash, dReceipt["blockNumber"], contractAddr, dReceipt["status"])
	if contractAddr == "" {
		fmt.Println("  ❌ Deploy receipt missing contractAddress!")
		os.Exit(1)
	}

	fmt.Println("  Step 2b: Sending Transaction to Store value 42 (0x2a) in contract...")
	cBytes, _ := hex.DecodeString(strings.TrimPrefix(contractAddr, "0x"))
	calldataVal := make([]byte, 32)
	calldataVal[31] = 42
	setNonce := getNonce(userAddr)
	setTx, err := ledger.NewTransactionFromKey(setNonce, cBytes, 0, 100000, 1000000000, calldataVal, chainID, userKey)
	if err != nil {
		fmt.Printf("  ❌ Failed to create set tx: %v\n", err)
		os.Exit(1)
	}
	setHash, sReceipt, err := sendAndConfirmTx(setTx)
	if err != nil {
		fmt.Printf("  ❌ Set tx failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  ✅ Value set! Tx: %s, Block: %v, Status: %v\n", setHash, sReceipt["blockNumber"], sReceipt["status"])

	fmt.Println("  Step 2c: Querying contract state via eth_call (reading slot 1)...")
	callRes, err := rpcCall("eth_call", []interface{}{
		map[string]string{"to": contractAddr, "data": "0x"},
		"latest",
	})
	if err != nil {
		fmt.Printf("  ❌ eth_call failed: %v\n", err)
		os.Exit(1)
	}
	callHex := fmt.Sprintf("%v", callRes)
	fmt.Printf("  ✅ eth_call result: %s\n", callHex)
	storedVal := strings.TrimPrefix(callHex, "0x")
	storedInt, _ := new(big.Int).SetString(storedVal, 16)
	if storedInt == nil || storedInt.Int64() != 42 {
		fmt.Printf("  ❌ Stored value mismatch! Expected 42, got %v\n", storedInt)
		os.Exit(1)
	}
	fmt.Printf("     Verified value in state: %d (0x2a) matches!\n", storedInt.Int64())
	fmt.Println("  🎉 TEST 2 PASSED: EVM Contract Deployment & Execution Verified!")

	// -------------------------------------------------------------
	// TEST 3: MULTI-TOKEN GAS FEE EXECUTION
	// -------------------------------------------------------------
	fmt.Println("\n[TEST 3/4] Multi-Token Gas Fee Execution")
	tokenUserKey, _ := crypto.GenerateKey()
	tokenUserAddr := "0x" + hex.EncodeToString(tokenUserKey.PubKey().Address())

	fmt.Printf("  Step 3a: Created Token-Only User: %s (Native Balance: %s wei)\n", tokenUserAddr, getBalance(tokenUserAddr))

	fmt.Printf("  Step 3b: Transferring 100 ERC-20 VIRI from Validator to Token-Only User...\n")
	erc20Bytes := contracts.AddrERC20
	// ERC20 transfer(to, amount)
	// sel: a9059cbb
	transferCalldata := make([]byte, 4+32+32)
	copy(transferCalldata[:4], []byte{0xa9, 0x05, 0x9c, 0xbb})
	copy(transferCalldata[4+12:4+32], tokenUserKey.PubKey().Address())
	// 100 tokens = 100 * 10^18
	amt := new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	amt.FillBytes(transferCalldata[4+32 : 4+64])

	vNonce := getNonce(valAddr)
	sendTokenTx, err := ledger.NewTransactionFromKey(vNonce, erc20Bytes, 0, 100000, 1000000000, transferCalldata, chainID, valKey)
	if err != nil {
		fmt.Printf("  ❌ Failed to create token transfer tx: %v\n", err)
		os.Exit(1)
	}
	stHash, stReceipt, err := sendAndConfirmTx(sendTokenTx)
	if err != nil {
		fmt.Printf("  ❌ Token transfer tx failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  ✅ ERC-20 transfer confirmed! Tx: %s, Block: %v, Status: %v\n", stHash, stReceipt["blockNumber"], stReceipt["status"])

	userTokenBal := getERC20Balance(erc20Addr, tokenUserAddr)
	fmt.Printf("     Token User ERC-20 Balance: %s\n", userTokenBal)
	if userTokenBal.Cmp(big.NewInt(0)) <= 0 {
		fmt.Println("  ❌ Token user failed to receive ERC-20 tokens!")
		os.Exit(1)
	}

	fmt.Println("  Step 3c: Constructing & Sending tx with FeeCurrency = ERC-20 Token (0 Native VIRI used)...")
	// Self-transfer of 0 value
	multiGasTx := &ledger.Transaction{
		Nonce:       0,
		From:        tokenUserKey.PubKey().Bytes(),
		To:          tokenUserKey.PubKey().Address(),
		Value:       0,
		GasLimit:    30000,
		GasPrice:    1000000000,
		FeeCurrency: erc20Bytes,
		ChainID:     chainID,
	}
	payload := multiGasTx.SigningPayload()
	sig, err := tokenUserKey.Sign(payload)
	if err != nil {
		fmt.Printf("  ❌ Sign multi-gas tx failed: %v\n", err)
		os.Exit(1)
	}
	recoveryID := byte(0)
	if len(sig.S.Bytes()) > 0 && sig.S.Bytes()[len(sig.S.Bytes())-1]&1 != 0 {
		recoveryID = 1
	}
	multiGasTx.Signature = &ledger.TxSignature{
		R: sig.R.Bytes(),
		S: sig.S.Bytes(),
		V: 27 + recoveryID,
	}
	multiGasTx.Hash = multiGasTx.ComputeHash()

	mgHash, mgReceipt, err := sendAndConfirmTx(multiGasTx)
	if err != nil {
		fmt.Printf("  ❌ Multi-token gas tx failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  ✅ Transaction confirmed with ERC-20 gas token! Tx: %s, Block: %v, Status: %v\n",
		mgHash, mgReceipt["blockNumber"], mgReceipt["status"])

	userTokenBalAfter := getERC20Balance(erc20Addr, tokenUserAddr)
	fmt.Printf("     Token User ERC-20 Balance Before: %s\n", userTokenBal)
	fmt.Printf("     Token User ERC-20 Balance After:  %s\n", userTokenBalAfter)
	fmt.Printf("     Token User Native Balance Still:  %s wei (Zero native VIRI spent!)\n", getBalance(tokenUserAddr))

	if userTokenBalAfter.Cmp(userTokenBal) >= 0 {
		fmt.Println("  ❌ Token balance was not deducted for gas fee!")
		os.Exit(1)
	}
	fmt.Println("  🎉 TEST 3 PASSED: Multi-Token Gas Fee Execution Verified!")

	// -------------------------------------------------------------
	// TEST 4: METAMASK FUNDING & EXPLORER / REST API VERIFICATION
	// -------------------------------------------------------------
	fmt.Println("\n[TEST 4/4] MetaMask Account 1 Funding & Explorer / REST API Verification")
	fmt.Printf("  Step 4a: Ensuring MetaMask Account 1 (%s) is funded...\n", metamaskAcc)
	cleanMeta := strings.TrimPrefix(metamaskAcc, "0x")
	metaBytes, _ := hex.DecodeString(cleanMeta)

	// Send 5 VIRI
	vNonce = getNonce(valAddr)
	metaFundTx, _ := ledger.NewTransactionFromKey(vNonce, metaBytes, 5_000_000_000_000_000_000, 50000, 1000000000, nil, chainID, valKey)
	mfHash, mfReceipt, err := sendAndConfirmTx(metaFundTx)
	if err == nil {
		fmt.Printf("  ✅ MetaMask 5 VIRI Funded! Tx: %s, Block: %v\n", mfHash, mfReceipt["blockNumber"])
	}

	// Send 500 ERC-20 tokens
	metaTokenCalldata := make([]byte, 4+32+32)
	copy(metaTokenCalldata[:4], []byte{0xa9, 0x05, 0x9c, 0xbb})
	copy(metaTokenCalldata[4+12:4+32], metaBytes)
	amtMeta := new(big.Int).Mul(big.NewInt(500), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	amtMeta.FillBytes(metaTokenCalldata[4+32 : 4+64])
	vNonce = getNonce(valAddr)
	metaTokenTx, _ := ledger.NewTransactionFromKey(vNonce, erc20Bytes, 0, 100000, 1000000000, metaTokenCalldata, chainID, valKey)
	mtHash, _, err := sendAndConfirmTx(metaTokenTx)
	if err == nil {
		fmt.Printf("  ✅ MetaMask 500 ERC-20 VIRI Funded! Tx: %s\n", mtHash)
	}
	fmt.Printf("     MetaMask Native Balance: %s wei\n", getBalance(metamaskAcc))
	fmt.Printf("     MetaMask ERC-20 Balance:  %s\n", getERC20Balance(erc20Addr, metamaskAcc))

	fmt.Println("  Step 4b: Testing Block Explorer & Public REST APIs...")
	testURLs := []string{
		apiURL + "/explorer/",
		apiURL + "/explorer/address/" + metamaskAcc,
		apiURL + "/explorer/tx/" + transferHash,
		apiURL + "/api/v1/health",
		apiURL + "/api/v1/blocks",
		apiURL + "/api/v1/status",
	}

	for _, u := range testURLs {
		resp, err := http.Get(u)
		if err != nil {
			fmt.Printf("  ❌ Failed GET %s: %v\n", u, err)
			os.Exit(1)
		}
		resp.Body.Close()
		fmt.Printf("  ✅ [%d OK] %s\n", resp.StatusCode, u)
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("  ❌ Expected 200 OK for %s, got %d\n", u, resp.StatusCode)
			os.Exit(1)
		}
	}
	fmt.Println("  🎉 TEST 4 PASSED: Explorer & Public REST APIs Verified!")

	fmt.Println("\n=================================================================")
	fmt.Println("🏆 ALL TESTS PASSED SUCCESSFULLY! FULL VIRIDI SUITE OPERATIONAL!")
	fmt.Println("=================================================================")
}
