package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	sdk "github.com/viri-chain/viri/pkg/sdk"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Syncer struct {
	client   *sdk.Client
	db       *mongo.Database
	interval time.Duration
	stopCh   chan struct{}
}

func NewSyncer(client *sdk.Client, db *mongo.Database, interval time.Duration) *Syncer {
	return &Syncer{
		client:   client,
		db:       db,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (s *Syncer) Start(ctx context.Context) {
	lastBlock := s.loadLastBlock(ctx)
	log.Printf("Syncer starting from block %d", lastBlock)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.syncOnce(ctx, &lastBlock)
		}
	}
}

func (s *Syncer) Stop() {
	close(s.stopCh)
}

func (s *Syncer) syncOnce(ctx context.Context, lastBlock *uint64) {
	current, err := s.client.GetBlockNumber()
	if err != nil {
		log.Printf("Failed to get block number: %v", err)
		return
	}

	if current <= *lastBlock {
		return
	}

	for height := *lastBlock + 1; height <= current; height++ {
		block, err := s.client.GetBlockByNumber(height)
		if err != nil {
			log.Printf("Failed to get block %d: %v", height, err)
			return
		}
		if block == nil {
			continue
		}

		if err := s.storeBlock(ctx, block); err != nil {
			log.Printf("Failed to store block %d: %v", height, err)
			return
		}

		*lastBlock = height
	}

	s.saveLastBlock(ctx, *lastBlock)
}

func (s *Syncer) storeBlock(ctx context.Context, raw map[string]interface{}) error {
	hash := getString(raw, "hash")
	numberHex := getString(raw, "number")
	number := parseHexUint64(numberHex)

	stored := StoredBlock{
		Hash:       hash,
		Number:     number,
		ParentHash: getString(raw, "parentHash"),
		Timestamp:  parseHexUint64(getString(raw, "timestamp")),
		Proposer:   getString(raw, "miner"),
		GasUsed:    parseHexUint64(getString(raw, "gasUsed")),
		GasLimit:   parseHexUint64(getString(raw, "gasLimit")),
		StateRoot:  getString(raw, "stateRoot"),
		Size:       parseHexUint64(getString(raw, "size")),
		ImportedAt: time.Now(),
	}

	rawTxs, _ := raw["transactions"].([]interface{})
	stored.TxCount = len(rawTxs)

	opts := options.Replace().SetUpsert(true)
	_, err := s.db.Collection("blocks").ReplaceOne(ctx, bson.M{"_id": hash}, stored, opts)
	if err != nil {
		return fmt.Errorf("store block: %w", err)
	}

	for i, rawTx := range rawTxs {
		tx, ok := rawTx.(map[string]interface{})
		if !ok {
			continue
		}
		if err := s.storeTx(ctx, tx, number, hash, i); err != nil {
			return fmt.Errorf("store tx %d: %w", i, err)
		}
	}

	return nil
}

func (s *Syncer) storeTx(ctx context.Context, tx map[string]interface{}, blockNumber uint64, blockHash string, index int) error {
	txHash := getString(tx, "hash")
	stored := StoredTx{
		Hash:        txHash,
		BlockNumber: blockNumber,
		BlockHash:   blockHash,
		From:        getString(tx, "from"),
		To:          getString(tx, "to"),
		Value:       getString(tx, "value"),
		GasPrice:    getString(tx, "gasPrice"),
		GasLimit:    parseHexUint64(getString(tx, "gas")),
		Nonce:       parseHexUint64(getString(tx, "nonce")),
		Input:       getString(tx, "input"),
		Index:       index,
		ImportedAt:  time.Now(),
	}

	opts := options.Replace().SetUpsert(true)
	if _, err := s.db.Collection("transactions").ReplaceOne(ctx, bson.M{"_id": txHash}, stored, opts); err != nil {
		return err
	}

	receipt, err := s.client.RPCCall("eth_getTransactionReceipt", []interface{}{txHash})
	if err == nil && receipt != nil {
		r, _ := receipt["result"].(map[string]interface{})
		if r != nil {
			s.storeReceipt(ctx, txHash, blockNumber, r)
		}
	}

	return nil
}

func (s *Syncer) storeReceipt(ctx context.Context, txHash string, blockNumber uint64, r map[string]interface{}) {
	status := "pending"
	if s := getString(r, "status"); s == "0x1" {
		status = "success"
	} else if s == "0x0" {
		status = "failed"
	}

	stored := StoredReceipt{
		TxHash:          txHash,
		BlockNumber:     blockNumber,
		GasUsed:         parseHexUint64(getString(r, "gasUsed")),
		Status:          status,
		ContractAddress: getString(r, "contractAddress"),
		ImportedAt:      time.Now(),
	}

	rawLogs, _ := r["logs"].([]interface{})
	for _, rawL := range rawLogs {
		l, ok := rawL.(map[string]interface{})
		if !ok {
			continue
		}
		topicsRaw, _ := l["topics"].([]interface{})
		topics := make([]string, 0, len(topicsRaw))
		for _, t := range topicsRaw {
			topics = append(topics, fmt.Sprintf("%v", t))
		}
		stored.Logs = append(stored.Logs, StoredLog{
			Address: getString(l, "address"),
			Topics:  topics,
			Data:    getString(l, "data"),
		})
	}

	opts := options.Replace().SetUpsert(true)
	if _, err := s.db.Collection("receipts").ReplaceOne(ctx, bson.M{"_id": txHash}, stored, opts); err != nil {
		log.Printf("Failed to store receipt %s: %v", txHash, err)
	}

	s.db.Collection("transactions").UpdateOne(ctx, bson.M{"_id": txHash}, bson.M{"$set": bson.M{"status": status}})
}

func (s *Syncer) loadLastBlock(ctx context.Context) uint64 {
	var state SyncState
	err := s.db.Collection("sync_state").FindOne(ctx, bson.M{"_id": "main"}).Decode(&state)
	if err != nil {
		return 0
	}
	return state.LastBlock
}

func (s *Syncer) saveLastBlock(ctx context.Context, block uint64) {
	opts := options.Replace().SetUpsert(true)
	s.db.Collection("sync_state").ReplaceOne(ctx, bson.M{"_id": "main"}, SyncState{
		ID:        "main",
		LastBlock: block,
		UpdatedAt: time.Now(),
	}, opts)
}

func getString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}

func parseHexUint64(s string) uint64 {
	if len(s) < 2 {
		return 0
	}
	if s[:2] == "0x" || s[:2] == "0X" {
		s = s[2:]
	}
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return 0
	}
	return v
}
