package consensus

import (
	"bytes"
	"testing"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

func TestValidatorSetGetProposer(t *testing.T) {
	validators := []*Validator{
		{Address: []byte("validator1"), PublicKey: []byte("pk1"), Stake: 100, IsActive: true},
		{Address: []byte("validator2"), PublicKey: []byte("pk2"), Stake: 200, IsActive: true},
		{Address: []byte("validator3"), PublicKey: []byte("pk3"), Stake: 300, IsActive: true},
	}

	vs := NewValidatorSet(validators, 1)

	proposer, err := vs.GetProposer(1)
	if err != nil {
		t.Fatalf("GetProposer failed: %v", err)
	}

	if proposer == nil {
		t.Fatal("Proposer is nil")
	}
}

func TestValidatorSetWeightedSelection(t *testing.T) {
	validators := []*Validator{
		{Address: []byte("v1"), PublicKey: []byte("pk1"), Stake: 10, IsActive: true},
		{Address: []byte("v2"), PublicKey: []byte("pk2"), Stake: 90, IsActive: true},
	}

	vs := NewValidatorSet(validators, 1)

	v2Count := 0
	iterations := 2000
	for i := uint64(0); i < uint64(iterations); i++ {
		p, _ := vs.GetProposer(i)
		if bytes.Equal(p.Address, []byte("v2")) {
			v2Count++
		}
	}

	if v2Count < iterations/2 {
		t.Errorf("Expected validator2 to be selected frequently, got %d/%d", v2Count, iterations)
	}
}

func TestValidatorSetSizeAndTotalStake(t *testing.T) {
	validators := []*Validator{
		{Address: []byte("v1"), PublicKey: []byte("pk1"), Stake: 100, IsActive: true},
		{Address: []byte("v2"), PublicKey: []byte("pk2"), Stake: 200, IsActive: true},
		{Address: []byte("v3"), PublicKey: []byte("pk3"), Stake: 0, IsActive: true},
		{Address: []byte("v4"), PublicKey: []byte("pk4"), Stake: 50, IsActive: false},
	}

	vs := NewValidatorSet(validators, 1)

	if vs.Size() != 3 {
		t.Errorf("Expected 3 active validators, got %d", vs.Size())
	}

	if vs.TotalStake() != 300 {
		t.Errorf("Expected total stake 300, got %d", vs.TotalStake())
	}
}

func TestValidatorSetAddRemove(t *testing.T) {
	vs := NewValidatorSet([]*Validator{}, 1)

	v := &Validator{Address: []byte("new"), PublicKey: []byte("pk"), Stake: 100, IsActive: true}
	err := vs.AddValidator(v)
	if err != nil {
		t.Fatalf("AddValidator failed: %v", err)
	}

	if vs.Size() != 1 {
		t.Errorf("Expected 1 validator after add, got %d", vs.Size())
	}

	err = vs.RemoveValidator([]byte("new"))
	if err != nil {
		t.Fatalf("RemoveValidator failed: %v", err)
	}

	if vs.Size() != 0 {
		t.Errorf("Expected 0 validators after remove, got %d", vs.Size())
	}
}

func TestValidatorSetUpdateStake(t *testing.T) {
	vs := NewValidatorSet([]*Validator{
		{Address: []byte("v1"), PublicKey: []byte("pk1"), Stake: 100, IsActive: true},
	}, 1)

	err := vs.UpdateStake([]byte("v1"), 200)
	if err != nil {
		t.Fatalf("UpdateStake failed: %v", err)
	}

	if vs.TotalStake() != 200 {
		t.Errorf("Expected total stake 200, got %d", vs.TotalStake())
	}
}

func TestValidatorSetCalculateQuorumStake(t *testing.T) {
	vs := NewValidatorSet([]*Validator{
		{Address: []byte("v1"), PublicKey: []byte("pk1"), Stake: 100, IsActive: true},
		{Address: []byte("v2"), PublicKey: []byte("pk2"), Stake: 200, IsActive: true},
		{Address: []byte("v3"), PublicKey: []byte("pk3"), Stake: 300, IsActive: true},
	}, 1)

	quorum := vs.CalculateQuorumStake()

	if quorum <= 400 || quorum > 600 {
		t.Errorf("Quorum should be >2/3 of 600, got %d", quorum)
	}
}

func TestQCIsValid(t *testing.T) {
	validators := []*Validator{
		{Address: []byte{0x01}, PublicKey: []byte("pk1"), Stake: 100, IsActive: true},
		{Address: []byte{0x02}, PublicKey: []byte("pk2"), Stake: 200, IsActive: true},
		{Address: []byte{0x03}, PublicKey: []byte("pk3"), Stake: 300, IsActive: true},
	}

	vs := NewValidatorSet(validators, 1)

	key1, _ := crypto.GenerateKey()
	sig1, _ := key1.Sign([]byte("hash"))
	key2, _ := crypto.GenerateKey()
	sig2, _ := key2.Sign([]byte("hash"))
	key3, _ := crypto.GenerateKey()
	sig3, _ := key3.Sign([]byte("hash"))

	qc := &QC{
		Height:    1,
		View:      0,
		Phase:     PhasePrepare,
		BlockHash: []byte("hash"),
		Signatures: map[string]crypto.Signature{
			"01": *sig1,
			"02": *sig2,
			"03": *sig3,
		},
		ValidatorAddrs: []string{"01", "02", "03"},
	}

	if !qc.IsValid(vs) {
		t.Error("QC with all validators should be valid")
	}
}
