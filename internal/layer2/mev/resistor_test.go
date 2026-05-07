package mev

import (
	"testing"
	"time"
)

func TestMEVResistorBatching(t *testing.T) {
	res := NewMEVResistor(TxOrderingFIFO, 2, 10*time.Millisecond)

	if res.PendingCount() != 0 {
		t.Fatalf("expected 0 pending")
	}

	res.AddTx(&PendingTx{GasPrice: 1})
	if res.GetBatch() != nil {
		t.Fatalf("expected nil batch before threshold")
	}

	res.AddTx(&PendingTx{GasPrice: 2})
	batch := res.GetBatch()
	if len(batch) != 2 {
		t.Fatalf("expected batch size 2, got %d", len(batch))
	}
	if res.PendingCount() != 0 {
		t.Fatalf("expected pending cleared")
	}
}

func TestMEVResistorOrderingGasPrice(t *testing.T) {
	res := NewMEVResistor(TxOrderingGasPrice, 3, 0)
	res.AddTx(&PendingTx{GasPrice: 1})
	res.AddTx(&PendingTx{GasPrice: 5})
	res.AddTx(&PendingTx{GasPrice: 3})

	batch := res.GetBatch()
	if batch[0].GasPrice != 5 {
		t.Fatalf("expected highest gas price first")
	}
	if batch[2].GasPrice != 1 {
		t.Fatalf("expected lowest gas price last")
	}
}

func TestMEVResistorOrderingMEVOptimized(t *testing.T) {
	res := NewMEVResistor(TxOrderingMEVOptimized, 2, 0)
	res.AddTx(&PendingTx{GasPrice: 2, Amount: 5})
	res.AddTx(&PendingTx{GasPrice: 3, Amount: 1})

	batch := res.GetBatch()
	if batch[0].GasPrice*batch[0].Amount < batch[1].GasPrice*batch[1].Amount {
		t.Fatalf("expected MEV optimized ordering")
	}
}

func TestMEVResistorClear(t *testing.T) {
	res := NewMEVResistor(TxOrderingFIFO, 1, 0)
	res.AddTx(&PendingTx{})
	res.Clear()
	if res.PendingCount() != 0 {
		t.Fatalf("expected cleared pending")
	}
}
