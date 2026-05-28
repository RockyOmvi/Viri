package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ExplorerServer serves a web-based block explorer UI that polls the RPC node.
type ExplorerServer struct {
	port    int
	rpcURL  string
	server  *http.Server
	tlsCert string
	tlsKey  string
}

func NewExplorerServer(port int, rpcURL string, tlsCert, tlsKey string) *ExplorerServer {
	return &ExplorerServer{
		port:    port,
		rpcURL:  rpcURL,
		tlsCert: tlsCert,
		tlsKey:  tlsKey,
	}
}

func (e *ExplorerServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", e.handleIndex)
	mux.HandleFunc("/block/", e.handleBlock)
	mux.HandleFunc("/tx/", e.handleTx)
	mux.HandleFunc("/address/", e.handleAddress)
	mux.HandleFunc("/api/blocks", e.handleAPIBlocks)
	mux.HandleFunc("/api/stats", e.handleAPIStats)
	mux.HandleFunc("/static/style.css", e.handleCSS)

	e.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", e.port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	if e.tlsCert != "" && e.tlsKey != "" {
		fmt.Printf("Block Explorer running at https://localhost:%d (TLS)\n", e.port)
		return e.server.ListenAndServeTLS(e.tlsCert, e.tlsKey)
	}
	fmt.Printf("Block Explorer running at http://localhost:%d\n", e.port)
	return e.server.ListenAndServe()
}

func (e *ExplorerServer) rpcCall(method string, params []interface{}) (interface{}, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}
	reqData, _ := json.Marshal(reqBody)

	resp, err := http.Post(e.rpcURL, "application/json", bytes.NewReader(reqData))
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

type ExplorerData struct {
	ChainID      string
	BlockHeight  string
	PeerCount    string
	NetworkName  string
	Blocks       []BlockSummary
	NodeInfo     map[string]interface{}
	Error        string
}

type BlockSummary struct {
	Number    string
	Hash      string
	Timestamp string
	TxCount   int
	Miner     string
}

type BlockDetail struct {
	Raw       map[string]interface{}
	Number    string
	Hash      string
	ParentHash string
	Timestamp string
	TxCount   int
	GasUsed   string
	Miner     string
	Txs       []map[string]interface{}
	Error     string
}

type AddressData struct {
	Address string
	Balance string
	Nonce   string
	Error   string
}

func (e *ExplorerServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := ExplorerData{
		NetworkName: "Viri Testnet",
	}

	// Get block height
	if result, err := e.rpcCall("eth_blockNumber", nil); err == nil {
		data.BlockHeight = fmt.Sprintf("%v", result)
	}

	// Get peer count
	if result, err := e.rpcCall("net_peerCount", nil); err == nil {
		data.PeerCount = fmt.Sprintf("%v", result)
	}

	// Get chain ID
	if result, err := e.rpcCall("eth_chainId", nil); err == nil {
		data.ChainID = fmt.Sprintf("%v", result)
	}

	// Get recent blocks (last 10)
	if data.BlockHeight != "" {
		heightStr := data.BlockHeight
		if strings.HasPrefix(heightStr, "0x") {
			if h, err := strconv.ParseUint(heightStr[2:], 16, 64); err == nil {
				limit := 10
				if int(h) < limit {
					limit = int(h) + 1
				}
				for i := 0; i < limit; i++ {
					blockNum := h - uint64(i)
					hexNum := fmt.Sprintf("0x%x", blockNum)
					if result, err := e.rpcCall("eth_getBlockByNumber", []interface{}{hexNum, true}); err == nil {
						if blockMap, ok := result.(map[string]interface{}); ok {
							bs := BlockSummary{
								Number: hexNum,
								Hash:   safeStr(blockMap["hash"]),
							}
							if ts, ok := blockMap["timestamp"]; ok {
								bs.Timestamp = fmt.Sprintf("%v", ts)
							}
							if txs, ok := blockMap["transactions"].([]interface{}); ok {
								bs.TxCount = len(txs)
							}
							if miner, ok := blockMap["miner"]; ok {
								bs.Miner = fmt.Sprintf("%v", miner)
							} else if proposer, ok := blockMap["proposer"]; ok {
								bs.Miner = fmt.Sprintf("%v", proposer)
							}
							data.Blocks = append(data.Blocks, bs)
						}
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, err := template.New("index").Parse(explorerIndexHTML)
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), 500)
		return
	}
	tmpl.Execute(w, data)
}

func (e *ExplorerServer) handleBlock(w http.ResponseWriter, r *http.Request) {
	blockID := strings.TrimPrefix(r.URL.Path, "/block/")
	if blockID == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	data := BlockDetail{}
	var result interface{}
	var err error

	if strings.HasPrefix(blockID, "0x") && len(blockID) == 66 {
		result, err = e.rpcCall("eth_getBlockByHash", []interface{}{blockID, true})
	} else {
		// Normalize: if numeric, convert to hex
		if !strings.HasPrefix(blockID, "0x") {
			if n, parseErr := strconv.ParseUint(blockID, 10, 64); parseErr == nil {
				blockID = fmt.Sprintf("0x%x", n)
			}
		}
		result, err = e.rpcCall("eth_getBlockByNumber", []interface{}{blockID, true})
	}

	if err != nil {
		data.Error = err.Error()
	} else if blockMap, ok := result.(map[string]interface{}); ok {
		data.Raw = blockMap
		data.Number = safeStr(blockMap["number"])
		data.Hash = safeStr(blockMap["hash"])
		data.ParentHash = safeStr(blockMap["parentHash"])
		data.Timestamp = safeStr(blockMap["timestamp"])
		data.GasUsed = safeStr(blockMap["gasUsed"])
		data.Miner = safeStr(blockMap["miner"])
		if txs, ok := blockMap["transactions"].([]interface{}); ok {
			data.TxCount = len(txs)
			for _, tx := range txs {
				if txMap, ok := tx.(map[string]interface{}); ok {
					data.Txs = append(data.Txs, txMap)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, _ := template.New("block").Parse(explorerBlockHTML)
	tmpl.Execute(w, data)
}

func (e *ExplorerServer) handleTx(w http.ResponseWriter, r *http.Request) {
	txHash := strings.TrimPrefix(r.URL.Path, "/tx/")
	if txHash == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	result, err := e.rpcCall("eth_getTransactionByHash", []interface{}{txHash})
	data := map[string]interface{}{"hash": txHash}
	if err != nil {
		data["error"] = err.Error()
	} else if result == nil {
		data["error"] = "Transaction not found"
	} else {
		data["tx"] = result
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, _ := template.New("tx").Parse(explorerTxHTML)
	tmpl.Execute(w, data)
}

func (e *ExplorerServer) handleAddress(w http.ResponseWriter, r *http.Request) {
	addr := strings.TrimPrefix(r.URL.Path, "/address/")
	if addr == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	data := AddressData{Address: addr}

	if result, err := e.rpcCall("eth_getBalance", []interface{}{addr, "latest"}); err == nil {
		data.Balance = fmt.Sprintf("%v", result)
	} else {
		data.Error = err.Error()
	}

	if result, err := e.rpcCall("eth_getTransactionCount", []interface{}{addr, "latest"}); err == nil {
		data.Nonce = fmt.Sprintf("%v", result)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl, _ := template.New("address").Parse(explorerAddressHTML)
	tmpl.Execute(w, data)
}

func (e *ExplorerServer) handleAPIBlocks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	result, err := e.rpcCall("eth_blockNumber", nil)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"blockNumber": result})
}

func (e *ExplorerServer) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	stats := map[string]interface{}{}

	if result, err := e.rpcCall("eth_blockNumber", nil); err == nil {
		stats["blockHeight"] = result
	}
	if result, err := e.rpcCall("net_peerCount", nil); err == nil {
		stats["peerCount"] = result
	}
	if result, err := e.rpcCall("eth_chainId", nil); err == nil {
		stats["chainId"] = result
	}
	if result, err := e.rpcCall("viri_nodeInfo", nil); err == nil {
		stats["nodeInfo"] = result
	}

	json.NewEncoder(w).Encode(stats)
}

func (e *ExplorerServer) handleCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Write([]byte(explorerCSS))
}

func safeStr(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// RunExplorer starts the block explorer standalone.
func RunExplorer() {
	port := 8080
	if p := os.Getenv("EXPLORER_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	rpcURL := os.Getenv("VIRI_RPC_URL")
	if rpcURL == "" {
		rpcURL = "http://localhost:8545"
	}

	tlsCert := os.Getenv("VIRI_TLS_CERT")
	tlsKey := os.Getenv("VIRI_TLS_KEY")

	fmt.Printf("Viri Block Explorer v%s\n", Version)
	fmt.Printf("RPC: %s | Port: %d\n", rpcURL, port)

	explorer := NewExplorerServer(port, rpcURL, tlsCert, tlsKey)
	if err := explorer.Start(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Explorer error: %v\n", err)
		os.Exit(1)
	}
}

// ============================================================
// Embedded HTML Templates
// ============================================================

const explorerCSS = `
:root {
  --bg-primary: #0a0e17;
  --bg-secondary: #111827;
  --bg-card: #1a2332;
  --bg-card-hover: #1f2b3d;
  --text-primary: #e2e8f0;
  --text-secondary: #94a3b8;
  --accent: #6366f1;
  --accent-light: #818cf8;
  --success: #10b981;
  --warning: #f59e0b;
  --border: #1e293b;
  --font: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}

* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  font-family: var(--font);
  background: var(--bg-primary);
  color: var(--text-primary);
  min-height: 100vh;
  line-height: 1.6;
}

.header {
  background: linear-gradient(135deg, var(--bg-secondary) 0%, #0f172a 100%);
  border-bottom: 1px solid var(--border);
  padding: 1rem 2rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.header h1 {
  font-size: 1.5rem;
  background: linear-gradient(135deg, var(--accent-light) 0%, var(--success) 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  font-weight: 800;
}

.header .network-badge {
  background: rgba(99, 102, 241, 0.15);
  color: var(--accent-light);
  padding: 0.25rem 0.75rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  border: 1px solid rgba(99, 102, 241, 0.3);
}

.container { max-width: 1200px; margin: 0 auto; padding: 2rem; }

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
  margin-bottom: 2rem;
}

.stat-card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 1.25rem;
  transition: all 0.2s;
}

.stat-card:hover {
  border-color: var(--accent);
  transform: translateY(-2px);
  box-shadow: 0 8px 25px rgba(99, 102, 241, 0.1);
}

.stat-card .label {
  color: var(--text-secondary);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  margin-bottom: 0.5rem;
}

.stat-card .value {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--accent-light);
}

.section-title {
  font-size: 1.25rem;
  font-weight: 700;
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 2px solid var(--accent);
  display: inline-block;
}

.table-responsive {
  width: 100%;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}

.blocks-table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 1rem;
}

.blocks-table th {
  text-align: left;
  padding: 0.75rem 1rem;
  background: var(--bg-secondary);
  color: var(--text-secondary);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  border-bottom: 1px solid var(--border);
}

.blocks-table td {
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--border);
  font-size: 0.9rem;
}

.blocks-table tr:hover td { background: var(--bg-card-hover); }

a {
  color: var(--accent-light);
  text-decoration: none;
  transition: color 0.2s;
}
a:hover { color: var(--success); }

.hash {
  font-family: 'Fira Code', 'Cascadia Code', monospace;
  font-size: 0.85rem;
  color: var(--text-secondary);
}

.hash-short {
  font-family: 'Fira Code', 'Cascadia Code', monospace;
  font-size: 0.85rem;
}

.detail-card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
}

.detail-row {
  display: flex;
  padding: 0.75rem 0;
  border-bottom: 1px solid rgba(255,255,255,0.05);
}

.detail-row:last-child { border-bottom: none; }

.detail-label {
  width: 180px;
  color: var(--text-secondary);
  font-size: 0.85rem;
  flex-shrink: 0;
}

.detail-value {
  flex: 1;
  word-break: break-all;
  font-family: 'Fira Code', 'Cascadia Code', monospace;
  font-size: 0.85rem;
}

.search-box {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 2rem;
}

.search-box input {
  flex: 1;
  padding: 0.75rem 1rem;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text-primary);
  font-size: 0.9rem;
  outline: none;
  transition: border-color 0.2s;
}

.search-box input:focus { border-color: var(--accent); }

.search-box button {
  padding: 0.75rem 1.5rem;
  background: var(--accent);
  color: white;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-weight: 600;
  transition: background 0.2s;
}

.search-box button:hover { background: var(--accent-light); }

.badge-tx {
  background: rgba(16, 185, 129, 0.15);
  color: var(--success);
  padding: 0.15rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
}

.error-box {
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: 8px;
  padding: 1rem;
  color: #fca5a5;
  margin-bottom: 1rem;
}

@media (max-width: 768px) {
  .container { padding: 1rem; }
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
  .detail-row { flex-direction: column; }
  .detail-label { margin-bottom: 0.25rem; }
}
`

const explorerIndexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Viri Block Explorer</title>
  <link rel="stylesheet" href="/static/style.css">
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600;700;800&family=Fira+Code:wght@400;500&display=swap" rel="stylesheet">
</head>
<body>
  <div class="header">
    <h1>⛓ Viri Explorer</h1>
    <span class="network-badge">{{.NetworkName}}</span>
  </div>
  <div class="container">
    <form class="search-box" onsubmit="handleSearch(event)">
      <input type="text" id="search" placeholder="Search by block number, tx hash, or address...">
      <button type="submit">Search</button>
    </form>

    <div class="stats-grid">
      <div class="stat-card">
        <div class="label">Block Height</div>
        <div class="value">{{.BlockHeight}}</div>
      </div>
      <div class="stat-card">
        <div class="label">Peers</div>
        <div class="value">{{.PeerCount}}</div>
      </div>
      <div class="stat-card">
        <div class="label">Chain ID</div>
        <div class="value">{{.ChainID}}</div>
      </div>
      <div class="stat-card">
        <div class="label">Network</div>
        <div class="value" style="font-size:1rem">{{.NetworkName}}</div>
      </div>
    </div>

    <h2 class="section-title">Recent Blocks</h2>
    {{if .Blocks}}
    <div class="table-responsive">
      <table class="blocks-table">
        <thead>
          <tr>
            <th>Block</th>
            <th>Hash</th>
            <th>Txs</th>
            <th>Validator</th>
          </tr>
        </thead>
        <tbody>
          {{range .Blocks}}
          <tr>
            <td><a href="/block/{{.Number}}">{{.Number}}</a></td>
            <td class="hash-short"><a href="/block/{{.Number}}">{{if gt (len .Hash) 16}}{{slice .Hash 0 16}}...{{else}}{{.Hash}}{{end}}</a></td>
            <td><span class="badge-tx">{{.TxCount}} txs</span></td>
            <td class="hash-short">{{if gt (len .Miner) 16}}{{slice .Miner 0 16}}...{{else}}{{.Miner}}{{end}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
    </div>
    {{else}}
    <p style="color: var(--text-secondary); padding: 2rem 0;">No blocks found. Is the node producing blocks?</p>
    {{end}}
  </div>

  <script>
  function handleSearch(e) {
    e.preventDefault();
    var q = document.getElementById('search').value.trim();
    if (!q) return;

    if (!q.startsWith('0x') && /^[0-9a-fA-F]+$/.test(q)) {
      if (q.length === 64 || q.length === 40) {
        q = '0x' + q;
      }
    }

    if (q.length === 66 && q.startsWith('0x')) {
      window.location.href = '/tx/' + q;
    } else if (q.startsWith('0x') && q.length === 42) {
      window.location.href = '/address/' + q;
    } else {
      window.location.href = '/block/' + q;
    }
  }

  // Auto-refresh every 5 seconds
  setTimeout(function() { location.reload(); }, 5000);
  </script>
</body>
</html>`

const explorerBlockHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Block {{.Number}} — Viri Explorer</title>
  <link rel="stylesheet" href="/static/style.css">
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600;700;800&family=Fira+Code:wght@400;500&display=swap" rel="stylesheet">
</head>
<body>
  <div class="header">
    <h1><a href="/">⛓ Viri Explorer</a></h1>
    <span class="network-badge">Block Detail</span>
  </div>
  <div class="container">
    {{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
    <h2 class="section-title">Block {{.Number}}</h2>
    <div class="detail-card">
      <div class="detail-row"><div class="detail-label">Block Number</div><div class="detail-value">{{.Number}}</div></div>
      <div class="detail-row"><div class="detail-label">Block Hash</div><div class="detail-value">{{.Hash}}</div></div>
      <div class="detail-row"><div class="detail-label">Parent Hash</div><div class="detail-value">{{.ParentHash}}</div></div>
      <div class="detail-row"><div class="detail-label">Timestamp</div><div class="detail-value">{{.Timestamp}}</div></div>
      <div class="detail-row"><div class="detail-label">Gas Used</div><div class="detail-value">{{.GasUsed}}</div></div>
      <div class="detail-row"><div class="detail-label">Validator</div><div class="detail-value">{{.Miner}}</div></div>
      <div class="detail-row"><div class="detail-label">Transactions</div><div class="detail-value">{{.TxCount}}</div></div>
    </div>

    {{if .Txs}}
    <h2 class="section-title">Transactions</h2>
    <div class="table-responsive">
      <table class="blocks-table">
        <thead><tr><th>Hash</th><th>From</th><th>To</th><th>Value</th></tr></thead>
        <tbody>
          {{range .Txs}}
          <tr>
            <td class="hash-short"><a href="/tx/{{index . "hash"}}">{{index . "hash"}}</a></td>
            <td class="hash-short"><a href="/address/{{index . "from"}}">{{index . "from"}}</a></td>
            <td class="hash-short"><a href="/address/{{index . "to"}}">{{index . "to"}}</a></td>
            <td>{{index . "value"}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
    </div>
    {{end}}

    <p style="margin-top: 2rem;"><a href="/">← Back to overview</a></p>
  </div>
</body>
</html>`

const explorerTxHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Transaction — Viri Explorer</title>
  <link rel="stylesheet" href="/static/style.css">
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600;700;800&family=Fira+Code:wght@400;500&display=swap" rel="stylesheet">
</head>
<body>
  <div class="header">
    <h1><a href="/">⛓ Viri Explorer</a></h1>
    <span class="network-badge">Transaction Detail</span>
  </div>
  <div class="container">
    {{if .error}}<div class="error-box">{{.error}}</div>{{end}}
    <h2 class="section-title">Transaction</h2>
    <div class="detail-card">
      <div class="detail-row"><div class="detail-label">TX Hash</div><div class="detail-value">{{.hash}}</div></div>
      {{if .tx}}
      {{range $k, $v := .tx}}
      <div class="detail-row"><div class="detail-label">{{$k}}</div><div class="detail-value">{{$v}}</div></div>
      {{end}}
      {{end}}
    </div>
    <p style="margin-top: 2rem;"><a href="/">← Back to overview</a></p>
  </div>
</body>
</html>`

const explorerAddressHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Address {{.Address}} — Viri Explorer</title>
  <link rel="stylesheet" href="/static/style.css">
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;600;700;800&family=Fira+Code:wght@400;500&display=swap" rel="stylesheet">
</head>
<body>
  <div class="header">
    <h1><a href="/">⛓ Viri Explorer</a></h1>
    <span class="network-badge">Address Detail</span>
  </div>
  <div class="container">
    {{if .Error}}<div class="error-box">{{.Error}}</div>{{end}}
    <h2 class="section-title">Address</h2>
    <div class="detail-card">
      <div class="detail-row"><div class="detail-label">Address</div><div class="detail-value">{{.Address}}</div></div>
      <div class="detail-row"><div class="detail-label">Balance</div><div class="detail-value">{{.Balance}}</div></div>
      <div class="detail-row"><div class="detail-label">Nonce</div><div class="detail-value">{{.Nonce}}</div></div>
    </div>
    <p style="margin-top: 2rem;"><a href="/">← Back to overview</a></p>
  </div>
</body>
</html>`
