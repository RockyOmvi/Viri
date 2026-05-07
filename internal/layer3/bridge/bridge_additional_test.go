package bridge

import "testing"

func TestAddSignatureMissingTransfer(t *testing.T) {
	br := NewChainBridge(1)
	if err := br.AddValidatorSignature("missing", "v1"); err == nil {
		t.Fatalf("expected missing transfer error")
	}
}
