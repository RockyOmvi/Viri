package consensus

import (
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

type Phase uint8

const (
	PhaseIdle Phase = iota
	PhasePrepare
	PhasePreCommit
	PhaseCommit
	PhaseDecide
)

type MessageType uint8

const (
	MsgProposal MessageType = iota
	MsgVotePrepare
	MsgVotePreCommit
	MsgVoteCommit
	MsgTimeout
	MsgNewView
	MsgBlockRequest
	MsgBlockResponse
)

func (p Phase) String() string {
	switch p {
	case PhaseIdle:
		return "IDLE"
	case PhasePrepare:
		return "PREPARE"
	case PhasePreCommit:
		return "PRECOMMIT"
	case PhaseCommit:
		return "COMMIT"
	case PhaseDecide:
		return "DECIDE"
	default:
		return "UNKNOWN"
	}
}

type ConsensusMessage struct {
	Type       MessageType
	Height     uint64
	View       uint64
	BlockHash  []byte
	Validator  []byte
	Signature  *crypto.Signature
	JustifyQC  *QC
	Payload    []byte
	Timestamp  time.Time
}

type consensusMessageJSON struct {
	Type       MessageType `json:"type"`
	Height     uint64      `json:"height"`
	View       uint64      `json:"view"`
	BlockHash  string      `json:"block_hash"`
	Validator  string      `json:"validator"`
	Signature  []byte      `json:"signature,omitempty"`
	JustifyQC  []byte      `json:"justify_qc,omitempty"`
	Payload    string      `json:"payload"`
	Timestamp  time.Time   `json:"timestamp"`
}

func (m *ConsensusMessage) MarshalJSON() ([]byte, error) {
	sigBytes := []byte{}
	if m.Signature != nil {
		sigBytes = m.Signature.Bytes()
	}
	qcBytes := []byte{}
	if m.JustifyQC != nil {
		var err error
		qcBytes, err = m.JustifyQC.Encode()
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(consensusMessageJSON{
		Type:      m.Type,
		Height:    m.Height,
		View:      m.View,
		BlockHash: hexEncode(m.BlockHash),
		Validator: hexEncode(m.Validator),
		Signature: sigBytes,
		JustifyQC: qcBytes,
		Payload:   hexEncode(m.Payload),
		Timestamp: m.Timestamp,
	})
}

func (m *ConsensusMessage) UnmarshalJSON(data []byte) error {
	var aux consensusMessageJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	m.Type = aux.Type
	m.Height = aux.Height
	m.View = aux.View
	var err error
	m.BlockHash, err = hexDecodeString(aux.BlockHash)
	if err != nil {
		return err
	}
	m.Validator, err = hexDecodeString(aux.Validator)
	if err != nil {
		return err
	}
	if len(aux.Signature) > 0 {
		sig, err := crypto.SignatureFromBytes(aux.Signature)
		if err == nil {
			m.Signature = sig
		}
	} else {
		m.Signature = nil
	}
	if len(aux.JustifyQC) > 0 {
		m.JustifyQC = &QC{}
		if err := json.Unmarshal(aux.JustifyQC, m.JustifyQC); err == nil {
		} else {
			m.JustifyQC = nil
		}
	} else {
		m.JustifyQC = nil
	}
	m.Payload, err = hexDecodeString(aux.Payload)
	if err != nil {
		return err
	}
	m.Timestamp = aux.Timestamp
	return nil
}

func hexEncode(b []byte) string {
	return hex.EncodeToString(b)
}

func hexDecodeString(s string) ([]byte, error) {
	if s == "" {
		return []byte{}, nil
	}
	return hex.DecodeString(s)
}

type Proposal struct {
	Height    uint64
	View      uint64
	BlockHash []byte
	Proposer  []byte
	JustifyQC *QC
	Payload   []byte
}

type Vote struct {
	Height    uint64
	View      uint64
	Phase     Phase
	BlockHash []byte
	Validator []byte
	Signature *crypto.Signature
}

type TimeoutCert struct {
	Height    uint64
	View      uint64
	Timeouts  map[string]bool
	Signatures map[string]crypto.Signature
	TotalStake uint64
	HighQC    *QC
}

func (tc *TimeoutCert) HasQuorum(threshold int) bool {
	return len(tc.Timeouts) >= threshold
}

type ConsensusState struct {
	Height      uint64
	View        uint64
	Phase       Phase
	LockedQC    *QC
	PreparedQC  *QC
	DecidedHash []byte
	StartTime   time.Time
	ProtocolVersion uint64
}

type ConsensusConfig struct {
	BlockTime         time.Duration
	ViewTimeout       time.Duration
	MaxViewTimeout    time.Duration
	TimeoutIncrease   time.Duration
	MinValidators     int
	EpochLength       uint64
	UnbondingPeriod   time.Duration
	SlashingFraction  float64
	DowntimeThreshold uint64
	ProtocolVersion  uint64
	MessageRateLimit  int
	MessageRateWindow time.Duration
}

func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		BlockTime:         3 * time.Second,
		ViewTimeout:       5 * time.Second,
		MaxViewTimeout:    30 * time.Second,
		TimeoutIncrease:   2 * time.Second,
		MinValidators:     4,
		EpochLength:       1000,
		UnbondingPeriod:   21 * 24 * time.Hour,
		SlashingFraction:  0.01,
		DowntimeThreshold: 10,
		ProtocolVersion:  1,
		MessageRateLimit:  1000,
		MessageRateWindow: time.Second,
	}
}
