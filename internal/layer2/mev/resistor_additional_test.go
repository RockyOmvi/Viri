package mev

import (
	"testing"
	"time"
)

func TestBatchDelay(t *testing.T) {
	res := NewMEVResistor(TxOrderingFIFO, 10, 10*time.Millisecond)
	res.AddTx(&PendingTx{})
	if res.GetBatch() != nil {
		t.Fatalf("expected nil before delay")
	}
	time.Sleep(15 * time.Millisecond)
	if res.GetBatch() == nil {
		t.Fatalf("expected batch after delay")
	}
}
