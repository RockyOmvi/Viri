package da

import "testing"

func TestGetBlobMissing(t *testing.T) {
	dal := NewDataAvailabilityLayer()
	if _, ok := dal.GetBlob([]byte("missing")); ok {
		t.Fatalf("unexpected blob")
	}
}

func TestSubmitBlobExceedsMaxSize(t *testing.T) {
	dal := NewDataAvailabilityLayerWithLimit(10, 100)
	_, err := dal.SubmitBlob(make([]byte, 11), []byte("sub"), 1)
	if err == nil {
		t.Fatal("expected error for blob exceeding max size")
	}
}

func TestSubmitBlobExceedsMaxBlobs(t *testing.T) {
	dal := NewDataAvailabilityLayerWithLimit(1000, 2)

	for i := 0; i < 2; i++ {
		data := []byte{byte(i)}
		if _, err := dal.SubmitBlob(data, []byte("sub"), uint64(i)); err != nil {
			t.Fatalf("unexpected error on submit %d: %v", i, err)
		}
	}

	if _, err := dal.SubmitBlob([]byte("overflow"), []byte("sub"), 3); err == nil {
		t.Fatal("expected error when blob storage is full")
	}
}

func TestDeleteBlob(t *testing.T) {
	dal := NewDataAvailabilityLayer()
	blob, err := dal.SubmitBlob([]byte("test"), []byte("sub"), 1)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	if err := dal.DeleteBlob(blob.Hash); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if _, ok := dal.GetBlob(blob.Hash); ok {
		t.Fatal("blob should be deleted")
	}

	if err := dal.DeleteBlob([]byte("nonexistent")); err == nil {
		t.Fatal("expected error deleting nonexistent blob")
	}
}

func TestPrune(t *testing.T) {
	dal := NewDataAvailabilityLayer()
	for i := 0; i < 10; i++ {
		data := []byte{byte(i)}
		if _, err := dal.SubmitBlob(data, []byte("sub"), uint64(i)); err != nil {
			t.Fatalf("submit failed: %v", err)
		}
	}

	removed := dal.Prune(5)
	if removed != 5 {
		t.Fatalf("expected 5 removed, got %d", removed)
	}
	if dal.TotalBlobs() != 5 {
		t.Fatalf("expected 5 blobs after prune, got %d", dal.TotalBlobs())
	}
}
