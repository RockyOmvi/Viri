// Package jepsen implements a Jepsen-style fault injection test suite for
// the Viri blockchain.
//
// What it does:
//   - Starts concurrent client goroutines that poll eth_blockNumber and
//     viri_getConsensusState across all 4 validators
//   - Injects faults via the Docker API: network partitions (disconnect/reconnect),
//     process kills (SIGTERM + restart), pauses (freeze/unfreeze), simulated
//     clock skew (CPU stress)
//   - Records every operation in a linear history with timestamps
//   - After the test, runs safety checkers:
//     - BlockConsistency: all validators report the same block hash per height
//     - Monotonicity: block heights increase monotonically
//     - ChainGrowth: blocks were produced during the test
//
// Usage:
//
//	func TestFaultInjection(t *testing.T) {
//	    result, err := jepsen.RunTest(ctx, jepsen.Config{
//	        Endpoints:    []string{"http://localhost:8545", ...},
//	        ClientCount:  4,
//	        OpsPerClient: 15,
//	        NemesisFreq:  3,
//	        TestDuration: 120 * time.Second,
//	    })
//	}
//
// Prerequisites:
//   - Docker testnet running (4 validators, ports 8545/8550/8555/8560)
//   - Docker Compose project named "viri-testnet" or similar
//
// Fault types (RandomNemesis picks one every NemesisFreq ops):
//   - PartitionNemesis: disconnect validator-0 from the Docker network for 5s
//   - KillNemesis: SIGTERM + restart validator-2
//   - PauseNemesis: freeze validators 1-2 for 8s
//   - ClockSkewNemesis: CPU stress on validator-3 (simulates timing issues)
package jepsen
