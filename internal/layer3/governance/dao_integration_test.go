package governance

import (
	"testing"
	"time"
)

func TestTallyRejectedOnNoVotes(t *testing.T) {
	dao := NewGovernanceDAO(10*time.Millisecond, 1, 0.5)
	p, _ := dao.SubmitProposal("t", "d", ProposalTypeText, []byte("p"), 10)
	if _, err := dao.TallyProposal(p.ID); err == nil {
		t.Fatalf("expected voting period error")
	}

	time.Sleep(15 * time.Millisecond)
	result, err := dao.TallyProposal(p.ID)
	if err != nil {
		t.Fatalf("tally failed: %v", err)
	}
	if result.Status != ProposalStatusRejected {
		t.Fatalf("expected rejected")
	}
}
