package da

import "testing"

func TestGetBlobMissing(t *testing.T) {
	dal := NewDataAvailabilityLayer()
	if _, ok := dal.GetBlob([]byte("missing")); ok {
		t.Fatalf("unexpected blob")
	}
}
