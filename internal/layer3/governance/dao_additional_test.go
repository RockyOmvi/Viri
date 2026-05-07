package governance

import "testing"

func TestGetActiveProposalsEmpty(t *testing.T) {
	dao := NewGovernanceDAO(0, 1, 0.5)
	if len(dao.GetActiveProposals()) != 0 {
		t.Fatalf("expected empty proposals")
	}
}
