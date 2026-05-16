package sequencer

import "testing"

func TestPendingCountEmpty(t *testing.T) {
	cfg, _ := newTestSequencerConfig()
	seq := NewSequencer(cfg, newTestChain(t))
	if seq.PendingCount() != 0 {
		t.Fatalf("expected 0 pending")
	}
}
