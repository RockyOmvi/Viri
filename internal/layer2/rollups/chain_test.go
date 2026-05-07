package rollups

import "testing"

func TestSubmitAndGetBatch(t *testing.T) {
	rc := NewRollupChain("id", RollupTypeOptimistic, 10)
	batch, err := rc.SubmitBatch([]byte("data"), []byte("submitter"), 1)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if batch.SequenceNumber != 0 {
		t.Fatalf("expected seq 0")
	}

	loaded, err := rc.GetBatch(0)
	if err != nil {
		t.Fatalf("get batch failed: %v", err)
	}
	if string(loaded.Data) != "data" {
		t.Fatalf("data mismatch")
	}

	if _, err := rc.GetBatch(1); err == nil {
		t.Fatalf("expected missing batch error")
	}
}

func TestChallengeAndConfirm(t *testing.T) {
	rc := NewRollupChain("id", RollupTypeZK, 10)
	_, _ = rc.SubmitBatch([]byte("data"), []byte("s"), 1)

	if err := rc.ChallengeBatch(0); err != nil {
		t.Fatalf("challenge failed: %v", err)
	}
	if err := rc.ConfirmBatch(0); err == nil {
		t.Fatalf("expected confirm error for challenged batch")
	}
	if err := rc.ChallengeBatch(1); err == nil {
		t.Fatalf("expected missing batch error")
	}
}

func TestConfirmBatch(t *testing.T) {
	rc := NewRollupChain("id", RollupTypeValidium, 10)
	_, _ = rc.SubmitBatch([]byte("data"), []byte("s"), 1)

	if err := rc.ConfirmBatch(0); err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
}

func TestPendingBatches(t *testing.T) {
	rc := NewRollupChain("id", RollupTypeOptimistic, 10)
	_, _ = rc.SubmitBatch([]byte("a"), []byte("s"), 1)
	_, _ = rc.SubmitBatch([]byte("b"), []byte("s"), 2)

	pending := rc.GetPendingBatches()
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending")
	}
}

func TestMetadataAccessors(t *testing.T) {
	rc := NewRollupChain("id", RollupTypeZK, 10)
	if rc.ID() != "id" {
		t.Fatalf("id mismatch")
	}
	if rc.Type() != RollupTypeZK {
		t.Fatalf("type mismatch")
	}
	if rc.BatchCount() != 0 {
		t.Fatalf("expected 0 batches")
	}
}
