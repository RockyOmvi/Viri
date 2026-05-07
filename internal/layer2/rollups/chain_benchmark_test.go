package rollups

import "testing"

func BenchmarkSubmitBatch(b *testing.B) {
	rc := NewRollupChain("id", RollupTypeOptimistic, 10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rc.SubmitBatch([]byte("data"), []byte("s"), uint64(i))
	}
}
