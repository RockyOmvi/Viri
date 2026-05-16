package da

import (
	"crypto/sha256"
	"testing"
)

func TestSubmitAndGetBlob(t *testing.T) {
	dal := NewDataAvailabilityLayer()
	data := []byte("blob-data")
	submitter := []byte("submitter")

	blob, err := dal.SubmitBlob(data, submitter, 1)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	if blob.Size != uint64(len(data)) {
		t.Fatalf("unexpected size: %d", blob.Size)
	}

	computed := sha256.Sum256(data)
	if string(blob.Hash) != string(computed[:]) {
		t.Fatalf("hash mismatch")
	}

	loaded, ok := dal.GetBlob(blob.Hash)
	if !ok {
		t.Fatalf("blob not found")
	}

	if string(loaded.Data) != string(data) {
		t.Fatalf("data mismatch")
	}
}

func TestSubmitDuplicateBlob(t *testing.T) {
	dal := NewDataAvailabilityLayer()
	data := []byte("dup-blob")

	if _, err := dal.SubmitBlob(data, []byte("a"), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := dal.SubmitBlob(data, []byte("a"), 2); err == nil {
		t.Fatalf("expected duplicate error")
	}
}

func TestVerifyAvailability(t *testing.T) {
	dal := NewDataAvailabilityLayer()
	data := []byte("avail")

	blob, err := dal.SubmitBlob(data, []byte("a"), 1)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	if !dal.VerifyAvailability(blob.Hash) {
		t.Fatalf("expected availability")
	}

	if dal.VerifyAvailability([]byte("missing")) {
		t.Fatalf("unexpected availability")
	}
}

func TestGetSamples(t *testing.T) {
	dal := NewDataAvailabilityLayer()
	data := []byte("sample")
	blob, err := dal.SubmitBlob(data, []byte("a"), 1)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	samples, err := dal.GetSamples(blob.Hash, 2)
	if err != nil {
		t.Fatalf("samples failed: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(samples))
	}

	all, err := dal.GetSamples(blob.Hash, 0)
	if err != nil {
		t.Fatalf("samples failed: %v", err)
	}
	if len(all) != len(data) {
		t.Fatalf("expected %d samples, got %d", len(data), len(all))
	}

	if _, err := dal.GetSamples([]byte("missing"), 1); err == nil {
		t.Fatalf("expected error for missing blob")
	}
}

func TestGetSubmittersBlobs(t *testing.T) {
	dal := NewDataAvailabilityLayer()
	submitter := []byte("user")

	_, _ = dal.SubmitBlob([]byte("a"), submitter, 1)
	_, _ = dal.SubmitBlob([]byte("b"), submitter, 2)

	blobs := dal.GetSubmittersBlobs(submitter)
	if len(blobs) != 2 {
		t.Fatalf("expected 2 blobs, got %d", len(blobs))
	}

	if dal.GetSubmittersBlobs([]byte("missing")) != nil {
		t.Fatalf("expected nil for missing submitter")
	}
}

func TestTotals(t *testing.T) {
	dal := NewDataAvailabilityLayer()
	if dal.TotalBlobs() != 0 {
		t.Fatalf("expected 0 blobs")
	}
	if dal.TotalSize() != 0 {
		t.Fatalf("expected 0 size")
	}

	_, _ = dal.SubmitBlob([]byte("abc"), []byte("a"), 1)
	_, _ = dal.SubmitBlob([]byte("de"), []byte("a"), 2)

	if dal.TotalBlobs() != 2 {
		t.Fatalf("expected 2 blobs")
	}
	if dal.TotalSize() != 5 {
		t.Fatalf("expected total size 5, got %d", dal.TotalSize())
	}
}
