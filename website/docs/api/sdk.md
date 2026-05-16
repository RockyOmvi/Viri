# SDK

Viri provides a Go SDK at `pkg/sdk/client.go` for interacting with the network.

## Installation

```go
import "github.com/viri-chain/viri/pkg/sdk"
```

## Usage

```go
// Create a client
client := sdk.NewClient("http://localhost:8545")

// Get block number
height, err := client.GetBlockNumber()

// Get block by number
block, err := client.GetBlockByHeight(height)

// Get balance
balance, err := client.GetBalance("0x...")

// Get peers
peers, err := client.GetPeers()

// Get node info
info, err := client.GetNodeInfo()

// Get status
status, err := client.GetStatus()

// Health check
healthy, err := client.HealthCheck()
```

## Error Handling

All methods return `(result, error)` following Go conventions. Check errors for network issues, RPC errors, and not-found cases.
