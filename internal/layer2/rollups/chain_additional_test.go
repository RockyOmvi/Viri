package rollups

import "testing"

func TestChallengeMissingBatch(t *testing.T) {
	rc := NewRollupChain("id", RollupTypeOptimistic, 10)
	if err := rc.ChallengeBatch(1); err == nil {
		t.Fatalf("expected missing batch error")
	}
}
