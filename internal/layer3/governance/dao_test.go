package governance

import (
	"testing"
	"time"
)

func TestSubmitProposal(t *testing.T) {
	dao := NewGovernanceDAO(10*time.Millisecond, 10, 0.5)

	if _, err := dao.SubmitProposal("t", "d", ProposalTypeText, []byte("p"), 1); err == nil {
		t.Fatalf("expected stake too low")
	}

	p, err := dao.SubmitProposal("t", "d", ProposalTypeText, []byte("p"), 10)
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if p.ID != 0 {
		t.Fatalf("expected id 0")
	}
	if dao.ProposalCount() != 1 {
		t.Fatalf("expected count 1")
	}
}

func TestVoteAndTally(t *testing.T) {
	dao := NewGovernanceDAO(10*time.Millisecond, 1, 0.5)
	p, _ := dao.SubmitProposal("t", "d", ProposalTypeText, []byte("p"), 100)

	if err := dao.Vote(1, []byte("v"), VoteChoiceYes, 10); err == nil {
		t.Fatalf("expected proposal not found")
	}

	if err := dao.Vote(p.ID, []byte("v1"), VoteChoiceYes, 60); err != nil {
		t.Fatalf("vote failed: %v", err)
	}

	if err := dao.Vote(p.ID, []byte("v1"), VoteChoiceNo, 10); err == nil {
		t.Fatalf("expected already voted error")
	}

	if err := dao.Vote(p.ID, []byte("v2"), VoteChoiceNo, 20); err != nil {
		t.Fatalf("vote failed: %v", err)
	}

	if _, err := dao.TallyProposal(p.ID); err == nil {
		t.Fatalf("expected voting period not ended")
	}

	time.Sleep(15 * time.Millisecond)
	result, err := dao.TallyProposal(p.ID)
	if err != nil {
		t.Fatalf("tally failed: %v", err)
	}
	if result.Status != ProposalStatusPassed {
		t.Fatalf("expected passed status")
	}
}

func TestVetoAndQuorum(t *testing.T) {
	dao := NewGovernanceDAO(10*time.Millisecond, 1, 0.8)
	p, _ := dao.SubmitProposal("t", "d", ProposalTypeText, []byte("p"), 100)

	_ = dao.Vote(p.ID, []byte("v1"), VoteChoiceNoWithVeto, 90)
	_ = dao.Vote(p.ID, []byte("v2"), VoteChoiceYes, 5)

	time.Sleep(15 * time.Millisecond)
	result, err := dao.TallyProposal(p.ID)
	if err != nil {
		t.Fatalf("tally failed: %v", err)
	}
	if result.Status != ProposalStatusRejected {
		t.Fatalf("expected rejected status")
	}
}

func TestGetProposalAndActive(t *testing.T) {
	dao := NewGovernanceDAO(10*time.Millisecond, 1, 0.5)
	p, _ := dao.SubmitProposal("t", "d", ProposalTypeText, []byte("p"), 10)

	if _, ok := dao.GetProposal(1); ok {
		t.Fatalf("unexpected proposal")
	}

	loaded, ok := dao.GetProposal(p.ID)
	if !ok || loaded.ID != p.ID {
		t.Fatalf("proposal not found")
	}

	active := dao.GetActiveProposals()
	if len(active) != 1 {
		t.Fatalf("expected 1 active")
	}
}
