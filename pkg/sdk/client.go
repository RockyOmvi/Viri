package sdk

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	RPCEndpoint   string
	APIEndpoint   string
	HTTPClient    *http.Client
	Timeout       time.Duration
}

func NewClient(rpcEndpoint string) *Client {
	return &Client{
		RPCEndpoint: rpcEndpoint,
		APIEndpoint: rpcEndpoint[:len(rpcEndpoint)-4] + "8546",
		HTTPClient:  &http.Client{Timeout: 10 * time.Second},
		Timeout:     10 * time.Second,
	}
}

func (c *Client) GetBlockNumber() (uint64, error) {
	result, err := c.RPCCall("eth_blockNumber", nil)
	if err != nil {
		return 0, err
	}

	if res, exists := result["result"]; exists {
		if hexStr, ok := res.(string); ok {
			var num uint64
			if _, err := fmt.Sscanf(hexStr, "0x%x", &num); err != nil {
				num = 0
			}
			return num, nil
		}
	}

	return 0, fmt.Errorf("unexpected response")
}

func (c *Client) GetBlockByNumber(height uint64) (map[string]interface{}, error) {
	params := []interface{}{fmt.Sprintf("0x%x", height), true}
	result, err := c.RPCCall("eth_getBlockByNumber", params)
	if err != nil {
		return nil, err
	}

	if res, exists := result["result"]; exists {
		if m, ok := res.(map[string]interface{}); ok {
			return m, nil
		}
	}

	return nil, fmt.Errorf("block not found")
}

func (c *Client) GetBalance(address string) (uint64, error) {
	params := []interface{}{address, "latest"}
	result, err := c.RPCCall("eth_getBalance", params)
	if err != nil {
		return 0, err
	}

	if res, exists := result["result"]; exists {
		if hexStr, ok := res.(string); ok {
			var balance uint64
			fmt.Sscanf(hexStr, "0x%x", &balance)
			return balance, nil
		}
	}

	return 0, fmt.Errorf("unexpected response")
}

func (c *Client) GetPeers() ([]map[string]interface{}, error) {
	result, err := c.RPCCall("viri_getPeers", nil)
	if err != nil {
		return nil, err
	}

	if res, exists := result["result"]; exists {
		if peers, ok := res.([]interface{}); ok {
			result := make([]map[string]interface{}, 0, len(peers))
			for _, p := range peers {
				if m, ok := p.(map[string]interface{}); ok {
					result = append(result, m)
				}
			}
			return result, nil
		}
	}

	return nil, fmt.Errorf("unexpected response")
}

func (c *Client) GetNodeInfo() (map[string]interface{}, error) {
	result, err := c.RPCCall("viri_nodeInfo", nil)
	if err != nil {
		return nil, err
	}

	if res, exists := result["result"]; exists {
		if m, ok := res.(map[string]interface{}); ok {
			return m, nil
		}
	}

	return nil, fmt.Errorf("unexpected response")
}

func (c *Client) GetStatus() (map[string]interface{}, error) {
	resp, err := c.HTTPClient.Get(c.APIEndpoint + "/api/v1/status")
	if err != nil {
		return nil, err
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

	return result, nil
}

func (c *Client) HealthCheck() (bool, error) {
	resp, err := c.HTTPClient.Get(c.APIEndpoint + "/api/v1/health")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}

func (c *Client) GetBlocks(from, to uint64, limit int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/v1/blocks?from=%d&to=%d&limit=%d", c.APIEndpoint, from, to, limit)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, err
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

	if blocks, exists := result["blocks"]; exists {
		if arr, ok := blocks.([]interface{}); ok {
			result := make([]map[string]interface{}, 0, len(arr))
			for _, b := range arr {
				if m, ok := b.(map[string]interface{}); ok {
					result = append(result, m)
				}
			}
			return result, nil
		}
	}

	return nil, fmt.Errorf("unexpected response")
}

type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	Error   interface{} `json:"error,omitempty"`
	ID      int         `json:"id"`
}

func (c *Client) RPCCall(method string, params []interface{}) (map[string]interface{}, error) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Post(c.RPCEndpoint, "application/json", bytes.NewReader(reqData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, err
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %v", rpcResp.Error)
	}

	return map[string]interface{}{
		"result": rpcResp.Result,
	}, nil
}

func HexToBytes(s string) ([]byte, error) {
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	return hex.DecodeString(s)
}

func BytesToHex(b []byte) string {
	return "0x" + hex.EncodeToString(b)
}
