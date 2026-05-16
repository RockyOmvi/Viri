package zk

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

type ShieldedTxType uint8

const (
	ShieldedTxTypeDeposit ShieldedTxType = iota
	ShieldedTxTypeWithdraw
	ShieldedTxTypeTransfer
)

type ShieldedTransaction struct {
	Type        ShieldedTxType
	Commitment  []byte
	Nullifier   []byte
	Proof       *Proof
	PublicData  []byte
	Signature   []byte
	Sender      []byte
	Receiver    []byte
	Amount      uint64
	Timestamp   time.Time
	Nonce       uint64
}

func (p *ShieldedTransaction) Serialize() ([]byte, error) {
	return json.Marshal(p)
}

func (p *ShieldedTransaction) Deserialize(data []byte) error {
	return json.Unmarshal(data, p)
}

func (p *ShieldedTransaction) ComputeHash() []byte {
	h := sha256.New()
	h.Write([]byte{byte(p.Type)})
	h.Write(p.Commitment)
	h.Write(p.Nullifier)
	h.Write(p.PublicData)
	h.Write([]byte(fmt.Sprintf("%d", p.Amount)))
	h.Write([]byte(fmt.Sprintf("%d", p.Timestamp.UnixNano())))
	h.Write([]byte(fmt.Sprintf("%d", p.Nonce)))
	return h.Sum(nil)
}

func (p *ShieldedTransaction) Validate() error {
	if len(p.Commitment) == 0 {
		return fmt.Errorf("missing commitment")
	}
	if len(p.Nullifier) == 0 {
		return fmt.Errorf("missing nullifier")
	}
	if p.Proof == nil {
		return fmt.Errorf("missing proof")
	}
	if p.Amount == 0 && p.Type != ShieldedTxTypeTransfer {
		return fmt.Errorf("amount required for deposit/withdraw")
	}
	return nil
}
