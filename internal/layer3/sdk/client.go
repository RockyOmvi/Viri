package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type L3Client struct {
	Endpoint   string
	HTTPClient *http.Client
}

func NewL3Client(endpoint string) *L3Client {
	return &L3Client{
		Endpoint:   endpoint,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *L3Client) SubmitProposal(title, description string, proposalType uint8, proposer string, stake uint64) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"title":       title,
		"description": description,
		"type":        proposalType,
		"proposer":    proposer,
		"stake":       stake,
	}

	return c.postJSON("/api/v3/governance/proposals", body)
}

func (c *L3Client) VoteOnProposal(proposalID uint64, voter string, choice uint8, stake uint64) error {
	body := map[string]interface{}{
		"proposal_id": proposalID,
		"voter":       voter,
		"choice":      choice,
		"stake":       stake,
	}

	_, err := c.postJSON("/api/v3/governance/vote", body)
	return err
}

func (c *L3Client) GetProposals() ([]map[string]interface{}, error) {
	resp, err := c.HTTPClient.Get(c.Endpoint + "/api/v3/governance/proposals")
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

	if proposals, exists := result["proposals"]; exists {
		if arr, ok := proposals.([]interface{}); ok {
			result := make([]map[string]interface{}, 0, len(arr))
			for _, p := range arr {
				if m, ok := p.(map[string]interface{}); ok {
					result = append(result, m)
				}
			}
			return result, nil
		}
	}

	return nil, fmt.Errorf("unexpected response")
}

func (c *L3Client) InitiateTransfer(sourceChain, destChain, sender, receiver string, amount uint64, token string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"source_chain": sourceChain,
		"dest_chain":   destChain,
		"sender":       sender,
		"receiver":     receiver,
		"amount":       amount,
		"token":        token,
	}

	return c.postJSON("/api/v3/bridge/transfers", body)
}

func (c *L3Client) ValidateTransfer(transferID, validatorID string) error {
	body := map[string]interface{}{
		"transfer_id":  transferID,
		"validator_id": validatorID,
	}

	_, err := c.postJSON("/api/v3/bridge/validate", body)
	return err
}

func (c *L3Client) GetPendingTransfers() ([]map[string]interface{}, error) {
	resp, err := c.HTTPClient.Get(c.Endpoint + "/api/v3/bridge/transfers")
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

	if transfers, exists := result["transfers"]; exists {
		if arr, ok := transfers.([]interface{}); ok {
			result := make([]map[string]interface{}, 0, len(arr))
			for _, t := range arr {
				if m, ok := t.(map[string]interface{}); ok {
					result = append(result, m)
				}
			}
			return result, nil
		}
	}

	return nil, fmt.Errorf("unexpected response")
}

func (c *L3Client) CreateChannel(portA, portB, chainA, chainB, version string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"port_a":  portA,
		"port_b":  portB,
		"chain_a": chainA,
		"chain_b": chainB,
		"version": version,
	}

	return c.postJSON("/api/v3/interop/channels", body)
}

func (c *L3Client) GetActiveChannels() ([]map[string]interface{}, error) {
	resp, err := c.HTTPClient.Get(c.Endpoint + "/api/v3/interop/channels")
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

	if channels, exists := result["channels"]; exists {
		if arr, ok := channels.([]interface{}); ok {
			result := make([]map[string]interface{}, 0, len(arr))
			for _, ch := range arr {
				if m, ok := ch.(map[string]interface{}); ok {
					result = append(result, m)
				}
			}
			return result, nil
		}
	}

	return nil, fmt.Errorf("unexpected response")
}

func (c *L3Client) SubmitIntent(user string, intentType uint8, input, output string, maxSlippage float64, deadline, fee uint64) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"user":          user,
		"type":          intentType,
		"input":         input,
		"output":        output,
		"max_slippage":  maxSlippage,
		"deadline":      deadline,
		"fee":           fee,
	}

	return c.postJSON("/api/v3/intents", body)
}

func (c *L3Client) SolveIntent(intentID, solverID string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"intent_id": intentID,
		"solver_id": solverID,
	}

	return c.postJSON("/api/v3/intents/solve", body)
}

func (c *L3Client) GetOpenIntents() ([]map[string]interface{}, error) {
	resp, err := c.HTTPClient.Get(c.Endpoint + "/api/v3/intents")
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

	if intents, exists := result["intents"]; exists {
		if arr, ok := intents.([]interface{}); ok {
			result := make([]map[string]interface{}, 0, len(arr))
			for _, i := range arr {
				if m, ok := i.(map[string]interface{}); ok {
					result = append(result, m)
				}
			}
			return result, nil
		}
	}

	return nil, fmt.Errorf("unexpected response")
}

func (c *L3Client) HealthCheck() (bool, error) {
	resp, err := c.HTTPClient.Get(c.Endpoint + "/api/v3/health")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}

func (c *L3Client) postJSON(path string, data map[string]interface{}) (map[string]interface{}, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Post(c.Endpoint+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	if errData, exists := result["error"]; exists && errData != nil {
		return nil, fmt.Errorf("API error: %v", errData)
	}

	return result, nil
}
