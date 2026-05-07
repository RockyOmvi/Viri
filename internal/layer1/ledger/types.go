package ledger

import (
	"time"
)

type BlockVersion uint8

const (
	Version1 BlockVersion = 1
)

type Header struct {
	Version       BlockVersion
	Height        uint64
	PrevHash      []byte
	TxsHash       []byte
	StateRoot     []byte
	ReceiptsRoot  []byte
	Timestamp     time.Time
	Proposer      []byte
	Signature     []byte
	Nonce         uint64
}

type Block struct {
	Header       *Header
	Transactions []*Transaction
	ConsensusHash []byte
}

type Transaction struct {
	Hash      []byte
	Nonce     uint64
	From      []byte
	To        []byte
	Value     uint64
	GasLimit  uint64
	GasPrice  uint64
	Data      []byte
	Signature *TxSignature
}

type TxSignature struct {
	R []byte
	S []byte
	V byte
}

type Receipt struct {
	TxHash      []byte
	BlockHeight uint64
	GasUsed     uint64
	Status      uint8
	Logs        []*Log
}

type Log struct {
	Address []byte
	Topics  [][]byte
	Data    []byte
}

type GenesisConfig struct {
	ChainID          uint64           `json:"chain_id"`
	Network          string           `json:"network_name,omitempty"`
	GenesisTime      string           `json:"genesis_time,omitempty"`
	InitialValidators []*ValidatorInfo `json:"validators"`
	InitialSupply    uint64           `json:"total_stake,omitempty"`
	BlockTime        time.Duration    `json:"block_time,omitempty"`
	MaxBlockSize     uint64           `json:"max_block_size,omitempty"`
	MaxGasPerBlock   uint64           `json:"max_gas_per_block,omitempty"`
}

type ValidatorInfo struct {
	Address   []byte `json:"address"`
	PublicKey []byte `json:"public_key"`
	Stake     uint64 `json:"stake"`
	Name      string `json:"name,omitempty"`
}
