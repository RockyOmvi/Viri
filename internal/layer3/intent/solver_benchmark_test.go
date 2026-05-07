package intent

import (
	"testing"
	"time"
)

func BenchmarkSubmitIntent(b *testing.B) {
	solver := NewIntentSolver()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = solver.SubmitIntent([]byte("user12345678"), IntentTypeSwap, []byte("in"), []byte("out"), 0.1, uint64(time.Now().Add(time.Hour).Unix()), 1)
	}
}
