package da

import "testing"

func BenchmarkSubmitBlob(b *testing.B) {
	dal := NewDataAvailabilityLayer()
	data := []byte("benchmark")
	submitter := []byte("s")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dal.SubmitBlob(append(data, byte(i%256)), submitter, uint64(i))
	}
}
