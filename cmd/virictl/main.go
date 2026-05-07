package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/state"
)

const Version = "0.1.0"
const defaultRPCURL = "http://localhost:8545"
const defaultWalletDir = ".viri"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "wallet":
		handleWallet()
	case "block":
		handleBlock()
	case "tx":
		handleTx()
	case "account":
		handleAccount()
	case "peer":
		handlePeer()
	case "status":
		handleStatus()
	case "backup":
		handleBackup()
	case "genesis":
		handleGenesis()
	case "version":
		fmt.Printf("virictl v%s\n", Version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Viri CLI - Control and interact with the Viri blockchain")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  virictl <command> [subcommand] [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  wallet                        Wallet management")
	fmt.Println("    create                      Create a new wallet")
	fmt.Println("    export <keyfile>            Export wallet to file")
	fmt.Println("    import <keyfile>            Import wallet from file")
	fmt.Println()
	fmt.Println("  block                         Block queries")
	fmt.Println("    latest                      Get latest block")
	fmt.Println("    get <height>                Get block by height")
	fmt.Println()
	fmt.Println("  tx                            Transaction management")
	fmt.Println("    send <to> <amount>          Send tokens")
	fmt.Println("    status                      Show node status")
	fmt.Println()
	fmt.Println("  account                       Account queries")
	fmt.Println("    balance <address>           Get account balance")
	fmt.Println()
	fmt.Println("  peer                          Peer management")
	fmt.Println("    list                        List connected peers")
	fmt.Println()
	fmt.Println("  status                        Show node status")
	fmt.Println()
	fmt.Println("  backup                        Database backup management")
	fmt.Println("    create [--dir <path>]       Create a new backup")
	fmt.Println("    list [--dir <path>]         List existing backups")
	fmt.Println("    restore <file> [--dir <path>] Restore from a backup")
	fmt.Println("    delete <file>               Delete a backup")
	fmt.Println()
	fmt.Println("  genesis                       Genesis ceremony management")
	fmt.Println("    init                        Initialize a new ceremony")
	fmt.Println("    add-validator               Add validator to ceremony")
	fmt.Println("    sign                        Sign the genesis manifest")
	fmt.Println("    verify                      Verify all signatures")
	fmt.Println("    status                      Show ceremony progress")
	fmt.Println("    finalize                    Finalize genesis")
	fmt.Println("    export                      Export genesis config")
	fmt.Println("    export-payload              Export signing payload for offline signing")
	fmt.Println("    import-signature            Import signature from offline signer")
	fmt.Println()
	fmt.Println("  version                       Show version information")
	fmt.Println("  help                          Show this help message")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --rpc <url>                   RPC endpoint (default: http://localhost:8545)")
}

func getRPCURL() string {
	for i, arg := range os.Args {
		if arg == "--rpc" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return defaultRPCURL
}

func parseRPCURL() (*url.URL, error) {
	raw := getRPCURL()
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid RPC URL: %w", err)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid RPC URL host: %s", raw)
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}
	return parsed, nil
}

func rpcCall(method string, params []interface{}) (map[string]interface{}, error) {
	parsed, err := parseRPCURL()
	if err != nil {
		return nil, err
	}

	rpcURL := parsed.String()

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}

	reqData, _ := json.Marshal(reqBody)

	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("RPC connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if errData, exists := result["error"]; exists && errData != nil {
		return nil, fmt.Errorf("RPC error: %v", errData)
	}

	return result, nil
}

func apiGet(path string) (map[string]interface{}, error) {
	parsed, err := parseRPCURL()
	if err != nil {
		return nil, err
	}

	// Replace the port with API port (default: 8546)
	host := parsed.Hostname()
	apiURL := fmt.Sprintf("%s://%s:8546%s", parsed.Scheme, host, path)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

func handleWallet() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: virictl wallet <create|export|import>")
		return
	}

	switch os.Args[2] {
	case "create":
		walletCreate()
	case "export":
		if len(os.Args) < 4 {
			fmt.Println("Usage: virictl wallet export <keyfile>")
			return
		}
		walletExport(os.Args[3])
	case "import":
		if len(os.Args) < 4 {
			fmt.Println("Usage: virictl wallet import <keyfile>")
			return
		}
		walletImport(os.Args[3])
	default:
		fmt.Fprintf(os.Stderr, "Unknown wallet command: %s\n", os.Args[2])
	}
}

func walletCreate() {
	key, err := crypto.GenerateKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate wallet: %v\n", err)
		os.Exit(1)
	}

	// Save to encrypted keystore
	walletDir := filepath.Join(defaultWalletDir, "wallets")
	addr := hex.EncodeToString(key.PubKey().Address())
	keyFile := filepath.Join(walletDir, addr+".key.enc")

	passphrase := os.Getenv("VIRI_WALLET_PASSPHRASE")
	if passphrase == "" {
		passphrase = "default-wallet-passphrase" // User should set env var
	}

	if err := crypto.EncryptKey(key, passphrase, keyFile); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save wallet: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Wallet created successfully!")
	fmt.Printf("Address:     0x%s\n", addr)
	fmt.Printf("Public Key:  0x%s\n", key.PubKey().Hex())
	fmt.Printf("Keystore:    %s\n", keyFile)
	fmt.Println()
	fmt.Println("WARNING: Set VIRI_WALLET_PASSPHRASE env var for a custom passphrase.")
	fmt.Println("Your private key is encrypted and stored in the keystore file.")
}

func walletExport(keyfile string) {
	if len(os.Args) < 4 {
		fmt.Println("Usage: virictl wallet export <keyfile>")
		return
	}

	walletDir := filepath.Join(defaultWalletDir, "wallets")
	files, err := os.ReadDir(walletDir)
	if err != nil || len(files) == 0 {
		fmt.Fprintf(os.Stderr, "No wallet found to export. Create one first.\n")
		os.Exit(1)
	}

	passphrase := os.Getenv("VIRI_WALLET_PASSPHRASE")
	if passphrase == "" {
		passphrase = "default-wallet-passphrase"
	}

	// Use first wallet found
	encKeyFile := filepath.Join(walletDir, files[0].Name())
	key, err := crypto.DecryptKey(encKeyFile, passphrase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to decrypt wallet: %v\n", err)
		os.Exit(1)
	}

	keyPath := keyfile // Export path
	if _, err := os.Stat(keyPath); err == nil {
		fmt.Fprintf(os.Stderr, "Keyfile already exists: %s\n", keyPath)
		os.Exit(1)
	}

	data := []byte(key.Hex())
	if err := os.WriteFile(keyPath, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write keyfile: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=========================================================")
	fmt.Println("WARNING: You have exported your UNENCRYPTED private key.")
	fmt.Println("Anyone with access to this file can control your funds!")
	fmt.Println("=========================================================")
	fmt.Printf("Wallet exported to: %s\n", keyPath)
	fmt.Printf("Address: 0x%x\n", key.PubKey().Address())
}

func walletImport(keyfile string) {
	data, err := os.ReadFile(keyfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read keyfile: %v\n", err)
		os.Exit(1)
	}

	keyBytes, err := hex.DecodeString(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid private key format: %v\n", err)
		os.Exit(1)
	}

	key, err := crypto.PrivateKeyFromBytes(keyBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse private key: %v\n", err)
		os.Exit(1)
	}

	passphrase := os.Getenv("VIRI_WALLET_PASSPHRASE")
	if passphrase == "" {
		passphrase = "default-wallet-passphrase"
	}

	walletDir := filepath.Join(defaultWalletDir, "wallets")
	if err := os.MkdirAll(walletDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create wallet directory: %v\n", err)
		os.Exit(1)
	}

	addr := hex.EncodeToString(key.PubKey().Address())
	encKeyFile := filepath.Join(walletDir, addr+".key.enc")

	if err := crypto.EncryptKey(key, passphrase, encKeyFile); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save imported wallet: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Wallet imported and encrypted successfully!")
	fmt.Printf("Address: 0x%s\n", addr)
	fmt.Printf("Keystore: %s\n", encKeyFile)
}

func handleBlock() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: virictl block <latest|get>")
		return
	}

	switch os.Args[2] {
	case "latest":
		blockLatest()
	case "get":
		if len(os.Args) < 4 {
			fmt.Println("Usage: virictl block get <height>")
			return
		}
		blockGet(os.Args[3])
	default:
		fmt.Fprintf(os.Stderr, "Unknown block command: %s\n", os.Args[2])
	}
}

func blockLatest() {
	result, err := rpcCall("eth_blockNumber", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get block number: %v\n", err)
		os.Exit(1)
	}

	if res, exists := result["result"]; exists {
		fmt.Printf("Latest block: %v\n", res)
	}
}

func blockGet(heightStr string) {
	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid block height: %s\n", heightStr)
		os.Exit(1)
	}

	params := []interface{}{fmt.Sprintf("0x%x", height), true}
	result, err := rpcCall("eth_getBlockByNumber", params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get block: %v\n", err)
		os.Exit(1)
	}

	if res, exists := result["result"]; exists {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
	}
}

func handleTx() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: virictl tx <send|status>")
		return
	}

	switch os.Args[2] {
	case "send":
		if len(os.Args) < 5 {
			fmt.Println("Usage: virictl tx send <to> <amount>")
			return
		}
		txSend(os.Args[3], os.Args[4])
	case "status":
		txStatus()
	default:
		fmt.Fprintf(os.Stderr, "Unknown tx command: %s\n", os.Args[2])
	}
}

func txSend(to string, amountStr string) {
	amount, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid amount: %s\n", amountStr)
		os.Exit(1)
	}

	// Strip 0x prefix if present
	if len(to) >= 2 && to[:2] == "0x" {
		to = to[2:]
	}

	toBytes, err := hex.DecodeString(to)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid address: %s\n", to)
		os.Exit(1)
	}

	// Load wallet key
	passphrase := os.Getenv("VIRI_WALLET_PASSPHRASE")
	if passphrase == "" {
		passphrase = "default-wallet-passphrase"
	}

	walletDir := filepath.Join(defaultWalletDir, "wallets")
	files, err := os.ReadDir(walletDir)
	if err != nil || len(files) == 0 {
		fmt.Fprintf(os.Stderr, "No wallet found. Create one first with: virictl wallet create\n")
		os.Exit(1)
	}

	// Use first wallet found
	keyFile := filepath.Join(walletDir, files[0].Name())
	key, err := crypto.DecryptKey(keyFile, passphrase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to decrypt wallet: %v\n", err)
		os.Exit(1)
	}

	// Get nonce from RPC
	addr := hex.EncodeToString(key.PubKey().Address())
	nonceResult, err := rpcCall("eth_getTransactionCount", []interface{}{"0x" + addr, "latest"})
	var nonce uint64
	if err == nil {
		if nonceHex, ok := nonceResult["result"].(string); ok {
			fmt.Sscanf(nonceHex, "0x%x", &nonce)
		}
	}

	// Create and sign transaction
	tx, err := ledger.NewTransactionFromKey(nonce, toBytes, amount, 21000, 1, nil, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create transaction: %v\n", err)
		os.Exit(1)
	}
	
	txBytes, err := ledger.SerializeTransaction(tx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to serialize transaction: %v\n", err)
		os.Exit(1)
	}

	// Submit via RPC
	result, err := rpcCall("eth_sendRawTransaction", []interface{}{"0x" + hex.EncodeToString(txBytes)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send transaction: %v\n", err)
		os.Exit(1)
	}

	if _, ok := result["result"]; !ok {
		fmt.Fprintf(os.Stderr, "RPC did not return a tx hash\n")
		os.Exit(1)
	}

	fmt.Printf("Transaction sent successfully!\n")
	fmt.Printf("TX Hash: %v\n", result["result"])
	fmt.Printf("To:      0x%s\n", to)
	fmt.Printf("Amount:  %d\n", amount)
}

func txStatus() {
	result, err := apiGet("/api/v1/status")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get status: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func handleAccount() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: virictl account balance <address>")
		return
	}

	if os.Args[2] == "balance" {
		accountBalance(os.Args[3])
	}
}

func accountBalance(address string) {
	if strings.HasPrefix(address, "0x") {
		address = address[2:]
	}
	addrBytes, err := hex.DecodeString(address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid address: %s\n", address)
		os.Exit(1)
	}

	params := []interface{}{fmt.Sprintf("0x%x", addrBytes), "latest"}
	result, err := rpcCall("eth_getBalance", params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get balance: %v\n", err)
		os.Exit(1)
	}

	if res, exists := result["result"]; exists {
		fmt.Printf("Balance: %v\n", res)
	}
}

func handlePeer() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: virictl peer list")
		return
	}

	if os.Args[2] == "list" {
		peerList()
	}
}

func peerList() {
	result, err := rpcCall("viri_getPeers", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get peers: %v\n", err)
		os.Exit(1)
	}

	if res, exists := result["result"]; exists {
		peers, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(peers))
	}
}

func handleStatus() {
	result, err := rpcCall("viri_nodeInfo", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get node info: %v\n", err)
		os.Exit(1)
	}

	if res, exists := result["result"]; exists {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
	}
}

func handleBackup() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: virictl backup <create|list|restore|delete>")
		return
	}

	dataDir := getDefaultDataDir()
	backupDir := getDefaultBackupDir()
	maxBackups := 10

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--data-dir":
			if i+1 < len(os.Args) {
				dataDir = os.Args[i+1]
				i++
			}
		case "--backup-dir":
			if i+1 < len(os.Args) {
				backupDir = os.Args[i+1]
				i++
			}
		case "--max-backups":
			if i+1 < len(os.Args) {
				fmt.Sscanf(os.Args[i+1], "%d", &maxBackups)
				i++
			}
		}
	}

	switch os.Args[2] {
	case "create":
		backupCreate(dataDir, backupDir, maxBackups)
	case "list":
		backupList(backupDir)
	case "restore":
		if len(os.Args) < 4 {
			fmt.Println("Usage: virictl backup restore <backup-file>")
			return
		}
		backupRestore(os.Args[3], dataDir)
	case "delete":
		if len(os.Args) < 4 {
			fmt.Println("Usage: virictl backup delete <backup-file>")
			return
		}
		backupDelete(os.Args[3], backupDir)
	default:
		fmt.Fprintf(os.Stderr, "Unknown backup command: %s\n", os.Args[2])
	}
}

func backupCreate(dataDir, backupDir string, maxBackups int) {
	mgr := state.NewBackupManager(dataDir, backupDir, maxBackups)

	fmt.Printf("Creating backup from: %s\n", dataDir)
	fmt.Printf("Backup destination: %s\n", backupDir)

	path, err := mgr.CreateBackup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Backup failed: %v\n", err)
		os.Exit(1)
	}

	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stat backup: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Backup created successfully: %s (%.2f MB)\n", path, float64(info.Size())/1024/1024)
}

func backupList(backupDir string) {
	mgr := state.NewBackupManager("", backupDir, 0)

	backups, err := mgr.ListBackups()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list backups: %v\n", err)
		os.Exit(1)
	}

	if len(backups) == 0 {
		fmt.Println("No backups found.")
		return
	}

	fmt.Printf("%-40s %12s %s\n", "NAME", "SIZE", "CREATED")
	fmt.Println(strings.Repeat("-", 75))

	for _, b := range backups {
		fmt.Printf("%-40s %10.2f MB %s\n", b.Name, float64(b.Size)/1024/1024, b.CreatedAt.Format("2006-01-02 15:04:05"))
	}
}

func backupRestore(backupPath, targetDir string) {
	mgr := state.NewBackupManager("", "", 0)

	fmt.Printf("Restoring backup from: %s\n", backupPath)
	fmt.Printf("Target directory: %s\n", targetDir)

	if err := mgr.RestoreBackup(backupPath, targetDir); err != nil {
		fmt.Fprintf(os.Stderr, "Restore failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Restore completed successfully.")
	fmt.Println("WARNING: Ensure the node is stopped before starting it with restored data.")
}

func backupDelete(backupName, backupDir string) {
	mgr := state.NewBackupManager("", backupDir, 0)

	if err := mgr.DeleteBackup(backupName); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to delete backup: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Backup deleted: %s\n", backupName)
}

func getDefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".viri"
	}
	return filepath.Join(home, ".viri")
}

func getDefaultBackupDir() string {
	dataDir := getDefaultDataDir()
	return filepath.Join(dataDir, "backups")
}

func handleGenesis() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: virictl genesis <init|add-validator|sign|verify|status|finalize|export|export-payload|import-signature>")
		return
	}

	switch os.Args[2] {
	case "init":
		genesisInit()
	case "add-validator":
		genesisAddValidator()
	case "sign":
		genesisSign()
	case "verify":
		genesisVerify()
	case "status":
		genesisStatus()
	case "finalize":
		genesisFinalize()
	case "export":
		genesisExport()
	case "export-payload":
		genesisExportPayload()
	case "import-signature":
		genesisImportSignature()
	default:
		fmt.Fprintf(os.Stderr, "Unknown genesis command: %s\n", os.Args[2])
	}
}
