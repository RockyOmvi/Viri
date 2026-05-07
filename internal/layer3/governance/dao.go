package governance

import (
	"fmt"
	"sync"
	"time"
)

type ProposalType uint8

const (
	ProposalTypeParameterChange ProposalType = iota
	ProposalTypeSoftwareUpgrade
	ProposalTypeText
	ProposalTypeTreasurySpend
)

type VoteChoice uint8

const (
	VoteChoiceYes VoteChoice = iota
	VoteChoiceNo
	VoteChoiceAbstain
	VoteChoiceNoWithVeto
)

type ProposalStatus uint8

const (
	ProposalStatusActive ProposalStatus = iota
	ProposalStatusPassed
	ProposalStatusRejected
	ProposalStatusExecuted
)

type Proposal struct {
	ID          uint64
	Title       string
	Description string
	Type        ProposalType
	Proposer    []byte
	Status      ProposalStatus
	StartTime   time.Time
	EndTime     time.Time
	YesVotes    uint64
	NoVotes     uint64
	Abstain     uint64
	VetoVotes   uint64
	TotalStake  uint64
	Threshold   float64
	VetoPercent float64
}

type GovernanceDAO struct {
	mu          sync.RWMutex
	proposals   map[uint64]*Proposal
	votes       map[uint64]map[string]VoteChoice
	nextID      uint64
	votingPeriod time.Duration
	minStake    uint64
	quorum      float64
}

func NewGovernanceDAO(votingPeriod time.Duration, minStake uint64, quorum float64) *GovernanceDAO {
	return &GovernanceDAO{
		proposals:   make(map[uint64]*Proposal),
		votes:       make(map[uint64]map[string]VoteChoice),
		votingPeriod: votingPeriod,
		minStake:    minStake,
		quorum:      quorum,
	}
}

func (dao *GovernanceDAO) SubmitProposal(title, description string, proposalType ProposalType, proposer []byte, stake uint64) (*Proposal, error) {
	dao.mu.Lock()
	defer dao.mu.Unlock()

	if stake < dao.minStake {
		return nil, fmt.Errorf("stake too low: required %d, got %d", dao.minStake, stake)
	}

	now := time.Now()
	proposal := &Proposal{
		ID:          dao.nextID,
		Title:       title,
		Description: description,
		Type:        proposalType,
		Proposer:    proposer,
		Status:      ProposalStatusActive,
		StartTime:   now,
		EndTime:     now.Add(dao.votingPeriod),
		Threshold:   0.5,
		VetoPercent: 0.33,
		TotalStake:  stake,
	}

	dao.proposals[dao.nextID] = proposal
	dao.votes[dao.nextID] = make(map[string]VoteChoice)
	dao.nextID++

	return proposal, nil
}

func (dao *GovernanceDAO) Vote(proposalID uint64, voter []byte, choice VoteChoice, stake uint64) error {
	dao.mu.Lock()
	defer dao.mu.Unlock()

	proposal, exists := dao.proposals[proposalID]
	if !exists {
		return fmt.Errorf("proposal not found")
	}

	if time.Now().After(proposal.EndTime) {
		return fmt.Errorf("voting period ended")
	}

	voterKey := string(voter)
	if _, voted := dao.votes[proposalID][voterKey]; voted {
		return fmt.Errorf("already voted")
	}

	dao.votes[proposalID][voterKey] = choice

	switch choice {
	case VoteChoiceYes:
		proposal.YesVotes += stake
	case VoteChoiceNo:
		proposal.NoVotes += stake
	case VoteChoiceAbstain:
		proposal.Abstain += stake
	case VoteChoiceNoWithVeto:
		proposal.VetoVotes += stake
	}

	return nil
}

func (dao *GovernanceDAO) TallyProposal(proposalID uint64) (*Proposal, error) {
	dao.mu.Lock()
	defer dao.mu.Unlock()

	proposal, exists := dao.proposals[proposalID]
	if !exists {
		return nil, fmt.Errorf("proposal not found")
	}

	if time.Now().Before(proposal.EndTime) {
		return nil, fmt.Errorf("voting period not ended")
	}

	totalVotes := proposal.YesVotes + proposal.NoVotes + proposal.Abstain + proposal.VetoVotes
	if totalVotes == 0 {
		proposal.Status = ProposalStatusRejected
		return proposal, nil
	}

	vetoRatio := float64(proposal.VetoVotes) / float64(totalVotes)
	if vetoRatio > proposal.VetoPercent {
		proposal.Status = ProposalStatusRejected
		return proposal, nil
	}

	quorumMet := float64(totalVotes) / float64(proposal.TotalStake) >= dao.quorum
	if !quorumMet {
		proposal.Status = ProposalStatusRejected
		return proposal, nil
	}

	yesRatio := float64(proposal.YesVotes) / float64(proposal.YesVotes+proposal.NoVotes)
	if yesRatio >= proposal.Threshold {
		proposal.Status = ProposalStatusPassed
	} else {
		proposal.Status = ProposalStatusRejected
	}

	return proposal, nil
}

func (dao *GovernanceDAO) GetProposal(id uint64) (*Proposal, bool) {
	dao.mu.RLock()
	defer dao.mu.RUnlock()

	p, exists := dao.proposals[id]
	if !exists {
		return nil, false
	}

	return p, true
}

func (dao *GovernanceDAO) GetActiveProposals() []*Proposal {
	dao.mu.RLock()
	defer dao.mu.RUnlock()

	var active []*Proposal
	for _, p := range dao.proposals {
		if p.Status == ProposalStatusActive {
			active = append(active, p)
		}
	}

	return active
}

func (dao *GovernanceDAO) ProposalCount() int {
	dao.mu.RLock()
	defer dao.mu.RUnlock()
	return len(dao.proposals)
}
