package consensus

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

type Validator struct {
	Address      []byte
	PublicKey    []byte
	Stake        uint64
	IsActive     bool
	IsJailed     bool
	JailedUntil  int64
	UnbondingAt  int64
	CommittedRounds uint64
	MissedVotes     uint64
}

func (v *Validator) Clone() *Validator {
	c := *v
	c.Address = append([]byte(nil), v.Address...)
	c.PublicKey = append([]byte(nil), v.PublicKey...)
	return &c
}

type ValidatorSet struct {
	validators []*Validator
	totalStake uint64
	epoch      uint64
}

func NewValidatorSet(validators []*Validator, epoch uint64) *ValidatorSet {
	vs := &ValidatorSet{
		epoch: epoch,
	}

	for _, v := range validators {
		if v.IsActive && !v.IsJailed {
			vs.validators = append(vs.validators, v.Clone())
			vs.totalStake += v.Stake
		}
	}

	sort.Slice(vs.validators, func(i, j int) bool {
		return bytes.Compare(vs.validators[i].Address, vs.validators[j].Address) < 0
	})

	return vs
}

func (vs *ValidatorSet) GetValidator(addr []byte) (*Validator, bool) {
	for _, v := range vs.validators {
		if bytes.Equal(v.Address, addr) {
			return v, true
		}
	}
	return nil, false
}

func (vs *ValidatorSet) GetValidators() []*Validator {
	result := make([]*Validator, len(vs.validators))
	for i, v := range vs.validators {
		result[i] = v.Clone()
	}
	return result
}

func (vs *ValidatorSet) Size() int {
	return len(vs.validators)
}

func (vs *ValidatorSet) TotalStake() uint64 {
	return vs.totalStake
}

func (vs *ValidatorSet) Epoch() uint64 {
	return vs.epoch
}

func (vs *ValidatorSet) GetProposer(height uint64) (*Validator, error) {
	if len(vs.validators) == 0 {
		return nil, fmt.Errorf("empty validator set")
	}

	index := vs.selectProposer(height, 0)
	return vs.validators[index], nil
}

func (vs *ValidatorSet) GetProposerForView(height uint64, view uint64) (*Validator, error) {
	if len(vs.validators) == 0 {
		return nil, fmt.Errorf("empty validator set")
	}

	index := vs.selectProposer(height, view)
	return vs.validators[index], nil
}

func (vs *ValidatorSet) selectProposer(height uint64, view uint64) int {
	if len(vs.validators) == 0 || vs.totalStake == 0 {
		return 0
	}

	seedBytes := make([]byte, 24)
	binary.BigEndian.PutUint64(seedBytes[0:8], height)
	binary.BigEndian.PutUint64(seedBytes[8:16], view)
	binary.BigEndian.PutUint64(seedBytes[16:24], vs.epoch)
	seedHash := crypto.SHA256(seedBytes)
	seed := binary.BigEndian.Uint64(seedHash[:8])

	weightedIndex := seed % vs.totalStake

	var cumulative uint64
	for i, v := range vs.validators {
		cumulative += v.Stake
		if weightedIndex < cumulative {
			offset := int(view % uint64(len(vs.validators)))
			return (i + offset) % len(vs.validators)
		}
	}

	return len(vs.validators) - 1
}

func (vs *ValidatorSet) GetNextProposer(height uint64) *Validator {
	idx := vs.selectProposer(height + 1, 0)
	return vs.validators[idx]
}

func (vs *ValidatorSet) HasSuperMajority(signatures map[string]bool) bool {
	var signedStake uint64
	for addr, signed := range signatures {
		if !signed {
			continue
		}
		addrBytes, err := hexDecode(addr)
		if err != nil {
			continue
		}
		for _, v := range vs.validators {
			if bytes.Equal(v.Address, addrBytes) {
				signedStake += v.Stake
				break
			}
		}
	}

	return signedStake*3 > vs.totalStake*2
}

func (vs *ValidatorSet) CalculateQuorumStake() uint64 {
	return (vs.totalStake * 2 / 3) + 1
}

func (vs *ValidatorSet) AddValidator(v *Validator) error {
	if _, exists := vs.GetValidator(v.Address); exists {
		return fmt.Errorf("validator already exists")
	}

	vs.validators = append(vs.validators, v.Clone())
	vs.totalStake += v.Stake

	sort.Slice(vs.validators, func(i, j int) bool {
		return bytes.Compare(vs.validators[i].Address, vs.validators[j].Address) < 0
	})

	return nil
}

func (vs *ValidatorSet) RemoveValidator(addr []byte) error {
	idx := -1
	for i, v := range vs.validators {
		if bytes.Equal(v.Address, addr) {
			idx = i
			break
		}
	}

	if idx == -1 {
		return fmt.Errorf("validator not found")
	}

	vs.totalStake -= vs.validators[idx].Stake
	vs.validators = append(vs.validators[:idx], vs.validators[idx+1:]...)

	return nil
}

func (vs *ValidatorSet) UpdateStake(addr []byte, newStake uint64) error {
	v, exists := vs.GetValidator(addr)
	if !exists {
		return fmt.Errorf("validator not found")
	}

	vs.totalStake -= v.Stake
	v.Stake = newStake
	vs.totalStake += newStake

	return nil
}

func (vs *ValidatorSet) IncrementCommitted(addr []byte) {
	v, exists := vs.GetValidator(addr)
	if exists {
		v.CommittedRounds++
		v.MissedVotes = 0
	}
}

func (vs *ValidatorSet) IncrementMissed(addr []byte) {
	v, exists := vs.GetValidator(addr)
	if exists {
		v.MissedVotes++
	}
}

func hexDecode(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("invalid hex string")
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		var b byte
		_, err := fmt.Sscanf(s[i:i+2], "%02x", &b)
		if err != nil {
			return nil, err
		}
		out[i/2] = b
	}
	return out, nil
}

type QC struct {
	Height    uint64
	View      uint64
	Phase     Phase
	BlockHash []byte
	Signatures map[string]crypto.Signature
	ValidatorAddrs []string
}

func (qc *QC) IsValid(vs *ValidatorSet) bool {
	var signedStake uint64
	for _, addr := range qc.ValidatorAddrs {
		if _, signed := qc.Signatures[addr]; !signed {
			continue
		}
		addrBytes, err := hexDecode(addr)
		if err != nil {
			continue
		}
		for _, v := range vs.validators {
			if bytes.Equal(v.Address, addrBytes) {
				signedStake += v.Stake
				break
			}
		}
	}

	return signedStake*3 > vs.totalStake*2
}

func (qc *QC) Encode() ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	binary.Write(buf, binary.BigEndian, qc.Height)
	binary.Write(buf, binary.BigEndian, qc.View)
	binary.Write(buf, binary.BigEndian, uint8(qc.Phase))
	binary.Write(buf, binary.BigEndian, uint16(len(qc.BlockHash)))
	buf.Write(qc.BlockHash)

	binary.Write(buf, binary.BigEndian, uint32(len(qc.ValidatorAddrs)))
	for _, addr := range qc.ValidatorAddrs {
		binary.Write(buf, binary.BigEndian, uint16(len(addr)))
		buf.WriteString(addr)
		if sig, signed := qc.Signatures[addr]; signed {
			rBytes := sig.R.Bytes()
			sBytes := sig.S.Bytes()
			binary.Write(buf, binary.BigEndian, uint16(len(rBytes)))
			buf.Write(rBytes)
			binary.Write(buf, binary.BigEndian, uint16(len(sBytes)))
			buf.Write(sBytes)
		}
	}

	return buf.Bytes(), nil
}

func DecodeQC(data []byte) (*QC, error) {
	buf := bytes.NewReader(data)
	qc := &QC{
		Signatures: make(map[string]crypto.Signature),
	}

	if err := binary.Read(buf, binary.BigEndian, &qc.Height); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.BigEndian, &qc.View); err != nil {
		return nil, err
	}

	var phase uint8
	if err := binary.Read(buf, binary.BigEndian, &phase); err != nil {
		return nil, err
	}
	qc.Phase = Phase(phase)

	var hashLen uint16
	if err := binary.Read(buf, binary.BigEndian, &hashLen); err != nil {
		return nil, err
	}
	qc.BlockHash = make([]byte, hashLen)
	if _, err := buf.Read(qc.BlockHash); err != nil {
		return nil, err
	}

	var addrCount uint32
	if err := binary.Read(buf, binary.BigEndian, &addrCount); err != nil {
		return nil, err
	}

	for i := uint32(0); i < addrCount; i++ {
		var addrLen uint16
		if err := binary.Read(buf, binary.BigEndian, &addrLen); err != nil {
			return nil, err
		}
		addr := make([]byte, addrLen)
		if _, err := buf.Read(addr); err != nil {
			return nil, err
		}
		addrStr := string(addr)
		qc.ValidatorAddrs = append(qc.ValidatorAddrs, addrStr)

		var rLen, sLen uint16
		if err := binary.Read(buf, binary.BigEndian, &rLen); err != nil {
			return nil, err
		}
		rBytes := make([]byte, rLen)
		if _, err := buf.Read(rBytes); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &sLen); err != nil {
			return nil, err
		}
		sBytes := make([]byte, sLen)
		if _, err := buf.Read(sBytes); err != nil {
			return nil, err
		}

		sig := crypto.Signature{
			R: new(big.Int).SetBytes(rBytes),
			S: new(big.Int).SetBytes(sBytes),
		}
		qc.Signatures[addrStr] = sig
	}

	return qc, nil
}
