package privacy

import "testing"

func BenchmarkCreateNote(b *testing.B) {
	pool := NewShieldedPool()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pool.CreateNote(1, []byte("owner"), []byte("rand1234567890123"))
	}
}
