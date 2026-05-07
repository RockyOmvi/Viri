package mev

import (
	"testing"
	"time"
)

func BenchmarkMEVBatch(b *testing.B) {
	res := NewMEVResistor(TxOrderingGasPrice, 100, time.Millisecond)
	for i := 0; i < 200; i++ {
		res.AddTx(&PendingTx{GasPrice: uint64(i + 1)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = res.GetBatch()
	}
}
