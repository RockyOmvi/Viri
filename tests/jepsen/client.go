package jepsen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Client struct {
	endpoint string
	http     *http.Client
}

func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Call(method string, params []interface{}) (*RPCResponse, error) {
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      int(time.Now().UnixNano()),
	}
	body, _ := json.Marshal(req)
	resp, err := c.http.Post(c.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("rpc call %s: %w", method, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var rpcResp RPCResponse
	if err := json.Unmarshal(raw, &rpcResp); err != nil {
		return nil, fmt.Errorf("rpc call %s: decode: %w", method, err)
	}
	return &rpcResp, nil
}

func (c *Client) BlockNumber() (uint64, error) {
	resp, err := c.Call("eth_blockNumber", []interface{}{})
	if err != nil {
		return 0, err
	}
	if resp.Error != nil {
		return 0, fmt.Errorf("rpc error: %s", resp.Error.Message)
	}
	var hex string
	json.Unmarshal(resp.Result, &hex)
	var n uint64
	fmt.Sscanf(hex, "0x%x", &n)
	return n, nil
}

func (c *Client) GetBalance(address string) (string, error) {
	resp, err := c.Call("eth_getBalance", []interface{}{address, "latest"})
	if err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", fmt.Errorf("rpc error: %s", resp.Error.Message)
	}
	var hex string
	json.Unmarshal(resp.Result, &hex)
	return hex, nil
}

func (c *Client) SendRawTransaction(hexTx string) (string, error) {
	resp, err := c.Call("eth_sendRawTransaction", []interface{}{hexTx})
	if err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", fmt.Errorf("rpc error: %s", resp.Error.Message)
	}
	var txHash string
	json.Unmarshal(resp.Result, &txHash)
	return txHash, nil
}

func (c *Client) GetTransactionReceipt(txHash string) (map[string]interface{}, error) {
	resp, err := c.Call("eth_getTransactionReceipt", []interface{}{txHash})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error: %s", resp.Error.Message)
	}
	var receipt map[string]interface{}
	json.Unmarshal(resp.Result, &receipt)
	return receipt, nil
}

func (c *Client) ConsensusState() (map[string]interface{}, error) {
	resp, err := c.Call("viri_getConsensusState", []interface{}{})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error: %s", resp.Error.Message)
	}
	var state map[string]interface{}
	json.Unmarshal(resp.Result, &state)
	return state, nil
}

func (c *Client) GetBlockByNumber(num uint64) (map[string]interface{}, error) {
	hex := fmt.Sprintf("0x%x", num)
	resp, err := c.Call("eth_getBlockByNumber", []interface{}{hex, true})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error: %s", resp.Error.Message)
	}
	var block map[string]interface{}
	json.Unmarshal(resp.Result, &block)
	return block, nil
}

func (c *Client) Health() (map[string]interface{}, error) {
	resp, err := c.http.Get(c.endpoint + "/health")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var h map[string]interface{}
	json.Unmarshal(raw, &h)
	return h, nil
}
