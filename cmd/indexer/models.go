package main

import "time"

type StoredBlock struct {
	Hash       string    `bson:"_id"`
	Number     uint64    `bson:"number"`
	ParentHash string    `bson:"parent_hash"`
	Timestamp  uint64    `bson:"timestamp"`
	Proposer   string    `bson:"proposer"`
	TxCount    int       `bson:"tx_count"`
	GasUsed    uint64    `bson:"gas_used"`
	GasLimit   uint64    `bson:"gas_limit"`
	StateRoot  string    `bson:"state_root"`
	Size       uint64    `bson:"size"`
	ImportedAt time.Time `bson:"imported_at"`
}

type StoredTx struct {
	Hash        string    `bson:"_id"`
	BlockNumber uint64    `bson:"block_number"`
	BlockHash   string    `bson:"block_hash"`
	From        string    `bson:"from"`
	To          string    `bson:"to"`
	Value       string    `bson:"value"`
	GasPrice    string    `bson:"gas_price"`
	GasLimit    uint64    `bson:"gas_limit"`
	GasUsed     uint64    `bson:"gas_used"`
	Nonce       uint64    `bson:"nonce"`
	Status      string    `bson:"status"`
	Input       string    `bson:"input"`
	Index       int       `bson:"index"`
	ImportedAt  time.Time `bson:"imported_at"`
}

type StoredReceipt struct {
	TxHash          string    `bson:"_id"`
	BlockNumber     uint64    `bson:"block_number"`
	GasUsed         uint64    `bson:"gas_used"`
	Status          string    `bson:"status"`
	ContractAddress string    `bson:"contract_address,omitempty"`
	Logs            []StoredLog `bson:"logs"`
	ImportedAt      time.Time `bson:"imported_at"`
}

type StoredLog struct {
	Address string   `bson:"address"`
	Topics  []string `bson:"topics"`
	Data    string   `bson:"data"`
}

type SyncState struct {
	ID        string    `bson:"_id"`
	LastBlock uint64    `bson:"last_block"`
	UpdatedAt time.Time `bson:"updated_at"`
}
