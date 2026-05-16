package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	out := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		out <- buf.String()
	}()

	fn()
	w.Close()
	os.Stdout = old
	return <-out
}

func TestMainOutputContainsExpected(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})

	checks := []string{
		"Viri Blockchain",
		"Wallet 1:",
		"Wallet 2:",
		"Transferring",
		"Deploying SimpleStorage",
		"set(42)",
		"get()",
		"Stored value: 42",
		"Account Abstraction",
		"SUMMARY",
		"completed successfully",
	}
	for _, c := range checks {
		if !strings.Contains(output, c) {
			t.Errorf("output missing: %s", c)
		}
	}
}

func TestMainTransfersCorrectly(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})

	if !strings.Contains(output, "Wallet 2 balance: 650000") {
		t.Errorf("expected wallet 2 final balance 650000, got output: %s", output)
	}
	if !strings.Contains(output, "Wallet 1 balance:") {
		t.Errorf("expected wallet 1 balance reported")
	}
}

func TestMainContractDeploys(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})

	if !strings.Contains(output, "Contract deployed at:") {
		t.Fatal("contract deployment did not succeed")
	}
	if !strings.Contains(output, "Gas used:") {
		t.Fatal("deployment gas usage not reported")
	}
}

func TestMainAASucceeds(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})

	if !strings.Contains(output, "AA Wallet deployed") {
		t.Fatal("account abstraction deployment did not succeed")
	}
}

func TestMainHandlesAllSections(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})

	sections := []string{
		"Creating wallets",
		"Initializing execution engine",
		"Transferring",
		"Deploying SimpleStorage",
		"Calling set(42)",
		"Calling get()",
		"Transferring 50000",
		"Querying Standard Contracts",
		"Testing Account Abstraction",
		"SUMMARY",
	}
	for _, s := range sections {
		if !strings.Contains(output, s) {
			t.Errorf("missing section: %s", s)
		}
	}
}
