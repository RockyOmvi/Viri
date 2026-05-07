package sequencer

import "testing"

func TestPendingCountEmpty(t *testing.T) {
	seq := NewSequencer(DefaultSequencerConfig(), newTestChain(t))
	if seq.PendingCount() != 0 {
		t.Fatalf("expected 0 pending")
	}
}
