package jepsen_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/viri-chain/viri/tests/jepsen"
)

func TestJepsenFaultInjection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Jepsen fault injection test in short mode")
	}

	endpoints := []string{
		"http://localhost:8545",
		"http://localhost:8550",
		"http://localhost:8555",
		"http://localhost:8560",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	cfg := jepsen.Config{
		Endpoints:    endpoints,
		ClientCount:  4,
		OpsPerClient: 999999,
		NemesisFreq:  3,
		TestDuration: 60 * time.Second,
	}

	t.Logf("Starting Jepsen test: %d clients, %ds duration",
		cfg.ClientCount, int(cfg.TestDuration.Seconds()))

	result, err := jepsen.RunTest(ctx, cfg)
	if err != nil {
		t.Fatalf("Jepsen test failed: %v", err)
	}

	t.Logf("Test completed in %v", result.Duration)
	t.Logf("Total operations: %d", len(result.History))
	t.Logf("Faults injected: %d", len(result.Faults))

	for _, f := range result.Faults {
		t.Logf("  [fault] %s", f)
	}

	allPassed := true
	for _, cr := range result.CheckResults {
		if cr.Valid {
			t.Logf("  [PASS] %s: %s", cr.Name, cr.Message)
		} else {
			t.Errorf("  [FAIL] %s: %s", cr.Name, cr.Message)
			allPassed = false
		}
		for _, d := range cr.Details {
			t.Logf("         %s", d)
		}
	}

	if !allPassed {
		t.Fatal("Jepsen safety checks failed")
	}

	ops := result.History
	okCount := 0
	failCount := 0
	for _, op := range ops {
		switch op.Status {
		case jepsen.OpOk:
			okCount++
		case jepsen.OpFail:
			failCount++
		}
	}
	t.Logf("Operation summary: %d ok, %d failed, %d total", okCount, failCount, len(ops))

	fmt.Println("\n=== JEPSEN TEST SUMMARY ===")
	fmt.Printf("Test duration: %v\n", result.Duration)
	fmt.Printf("Operations: %d\n", len(result.History))
	fmt.Printf("Faults injected: %d\n", len(result.Faults))
	for _, cr := range result.CheckResults {
		status := "PASS"
		if !cr.Valid {
			status = "FAIL"
		}
		fmt.Printf("[%s] %s: %s\n", status, cr.Name, cr.Message)
	}
	fmt.Printf("Operations: %d ok, %d failed\n", okCount, failCount)
	fmt.Println("===========================")
}
