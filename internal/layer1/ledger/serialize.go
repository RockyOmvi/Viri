package ledger

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
)

func init() {
	gob.Register(&Header{})
	gob.Register(&Block{})
	gob.Register(&Transaction{})
	gob.Register(&TxSignature{})
	gob.Register(&Receipt{})
	gob.Register(&Log{})
}

func SerializeBlock(block *Block) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(block); err != nil {
		return nil, fmt.Errorf("serialize block: %w", err)
	}
	return buf.Bytes(), nil
}

func DeserializeBlock(data []byte) (*Block, error) {
	var block Block
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&block); err != nil {
		return nil, fmt.Errorf("deserialize block: %w", err)
	}
	return &block, nil
}

func SerializeTransaction(tx *Transaction) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(tx); err != nil {
		return nil, fmt.Errorf("serialize tx: %w", err)
	}
	return buf.Bytes(), nil
}

func DeserializeTransaction(data []byte) (*Transaction, error) {
	var tx Transaction
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&tx); err != nil {
		return nil, fmt.Errorf("deserialize tx: %w", err)
	}
	return &tx, nil
}

func SerializeReceipt(receipt *Receipt) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(receipt); err != nil {
		return nil, fmt.Errorf("serialize receipt: %w", err)
	}
	return buf.Bytes(), nil
}

func DeserializeReceipt(data []byte) (*Receipt, error) {
	var receipt Receipt
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&receipt); err != nil {
		return nil, fmt.Errorf("deserialize receipt: %w", err)
	}
	return &receipt, nil
}

func SerializeHeader(header *Header) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(header); err != nil {
		return nil, fmt.Errorf("serialize header: %w", err)
	}
	return buf.Bytes(), nil
}

func DeserializeHeader(data []byte) (*Header, error) {
	var header Header
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&header); err != nil {
		return nil, fmt.Errorf("deserialize header: %w", err)
	}
	return &header, nil
}

func BlockToJSON(block *Block) ([]byte, error) {
	type BlockJSON struct {
		Header     *Header        `json:"header"`
		TxCount    int            `json:"tx_count"`
		Hash       string         `json:"hash"`
		TxHashes   []string       `json:"tx_hashes"`
	}

	txHashes := make([]string, len(block.Transactions))
	for i, tx := range block.Transactions {
		txHashes[i] = fmt.Sprintf("%x", tx.Hash)
	}

	jsonBlock := BlockJSON{
		Header:   block.Header,
		TxCount:  len(block.Transactions),
		Hash:     fmt.Sprintf("%x", block.Hash()),
		TxHashes: txHashes,
	}

	return json.Marshal(jsonBlock)
}

func TransactionToJSON(tx *Transaction) ([]byte, error) {
	type TxJSON struct {
		Hash     string `json:"hash"`
		Nonce    uint64 `json:"nonce"`
		From     string `json:"from"`
		To       string `json:"to"`
		Value    uint64 `json:"value"`
		GasLimit uint64 `json:"gas_limit"`
		GasPrice uint64 `json:"gas_price"`
		Data     string `json:"data"`
	}

	return json.Marshal(TxJSON{
		Hash:     fmt.Sprintf("%x", tx.Hash),
		Nonce:    tx.Nonce,
		From:     fmt.Sprintf("%x", tx.From),
		To:       fmt.Sprintf("%x", tx.To),
		Value:    tx.Value,
		GasLimit: tx.GasLimit,
		GasPrice: tx.GasPrice,
		Data:     fmt.Sprintf("%x", tx.Data),
	})
}

func BlocksToJSON(blocks []*Block) ([]byte, error) {
	type BlockSummary struct {
		Height    uint64  `json:"height"`
		Hash      string  `json:"hash"`
		PrevHash  string  `json:"prev_hash"`
		TxCount   int     `json:"tx_count"`
		Timestamp int64   `json:"timestamp"`
		Proposer  string  `json:"proposer"`
	}

	summaries := make([]BlockSummary, len(blocks))
	for i, block := range blocks {
		summaries[i] = BlockSummary{
			Height:    block.Header.Height,
			Hash:      fmt.Sprintf("%x", block.Hash()),
			PrevHash:  fmt.Sprintf("%x", block.Header.PrevHash),
			TxCount:   len(block.Transactions),
			Timestamp: block.Header.Timestamp.Unix(),
			Proposer:  fmt.Sprintf("%x", block.Header.Proposer),
		}
	}

	return json.Marshal(map[string]interface{}{
		"count":  len(blocks),
		"blocks": summaries,
	})
}

func WriteBlockToFile(block *Block, w io.Writer) error {
	enc := gob.NewEncoder(w)
	return enc.Encode(block)
}

func ReadBlockFromFile(r io.Reader) (*Block, error) {
	var block Block
	dec := gob.NewDecoder(r)
	if err := dec.Decode(&block); err != nil {
		return nil, err
	}
	return &block, nil
}
