package bridge

import "testing"

func BenchmarkInitiateTransfer(b *testing.B) {
	br := NewChainBridge(1)
	br.RegisterChain("a", "A", "a")
	br.RegisterChain("b", "B", "b")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = br.InitiateTransfer("a", "b", []byte("s"), []byte("r"), 1, []byte("T"))
	}
}
