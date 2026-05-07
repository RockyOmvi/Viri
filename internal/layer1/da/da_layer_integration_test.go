package da

import "testing"

func TestSamplesCountBounds(t *testing.T) {
	dal := NewDataAvailabilityLayer()
	blob, _ := dal.SubmitBlob([]byte("abc"), []byte("s"), 1)

	samples, err := dal.GetSamples(blob.Hash, 100)
	if err != nil {
		t.Fatalf("samples failed: %v", err)
	}
	if len(samples) != 3 {
		t.Fatalf("expected 3 samples")
	}
}
