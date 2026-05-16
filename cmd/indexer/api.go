package main

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type APIServer struct {
	db      *mongo.Database
	httpSrv *http.Server
}

func NewAPIServer(addr string, db *mongo.Database) *APIServer {
	s := &APIServer{db: db}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/address/", s.handleAddress)
	mux.HandleFunc("/api/v1/blocks", s.handleBlocks)
	mux.HandleFunc("/api/v1/health", s.handleHealth)

	s.httpSrv = &http.Server{
		Addr:    addr,
		Handler: withCORS(mux),
	}
	return s
}

func (s *APIServer) Start() error {
	log.Printf("Indexer API server listening on %s", s.httpSrv.Addr)
	return s.httpSrv.ListenAndServe()
}

func (s *APIServer) Stop(ctx context.Context) error {
	return s.httpSrv.Shutdown(ctx)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	var state SyncState
	err := s.db.Collection("sync_state").FindOne(r.Context(), bson.M{"_id": "main"}).Decode(&state)
	lastBlock := uint64(0)
	if err == nil {
		lastBlock = state.LastBlock
	}

	totalBlocks, _ := s.db.Collection("blocks").EstimatedDocumentCount(r.Context())
	totalTxs, _ := s.db.Collection("transactions").EstimatedDocumentCount(r.Context())

	writeJSON(w, map[string]interface{}{
		"synced_block": lastBlock,
		"total_blocks": totalBlocks,
		"total_txs":    totalTxs,
		"status":       "running",
		"updated_at":   time.Now(),
	})
}

func (s *APIServer) handleAddress(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Path[len("/api/v1/address/"):]
	if addr == "" {
		writeError(w, "address required", http.StatusBadRequest)
		return
	}

	page, limit := parsePageParams(r, 1, 20)
	skip := (page - 1) * limit

	ctx := r.Context()

	total, err := s.db.Collection("transactions").CountDocuments(ctx, bson.M{
		"$or": []bson.M{{"from": addr}, {"to": addr}},
	})
	if err != nil {
		writeError(w, "query failed", http.StatusInternalServerError)
		return
	}

	findOpts := options.Find().
		SetSort(bson.M{"block_number": -1, "index": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := s.db.Collection("transactions").Find(ctx, bson.M{
		"$or": []bson.M{{"from": addr}, {"to": addr}},
	}, findOpts)
	if err != nil {
		writeError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var txs []StoredTx
	if err := cursor.All(ctx, &txs); err != nil {
		writeError(w, "decode failed", http.StatusInternalServerError)
		return
	}

	if txs == nil {
		txs = []StoredTx{}
	}

	writeJSON(w, map[string]interface{}{
		"transactions": txs,
		"total":        total,
		"page":         page,
		"limit":        limit,
		"pages":        int(math.Ceil(float64(total) / float64(limit))),
	})
}

func (s *APIServer) handleBlocks(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePageParams(r, 1, 20)
	skip := (page - 1) * limit

	ctx := r.Context()

	total, err := s.db.Collection("blocks").EstimatedDocumentCount(ctx)
	if err != nil {
		writeError(w, "query failed", http.StatusInternalServerError)
		return
	}

	findOpts := options.Find().
		SetSort(bson.M{"number": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := s.db.Collection("blocks").Find(ctx, bson.M{}, findOpts)
	if err != nil {
		writeError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var blocks []StoredBlock
	if err := cursor.All(ctx, &blocks); err != nil {
		writeError(w, "decode failed", http.StatusInternalServerError)
		return
	}

	if blocks == nil {
		blocks = []StoredBlock{}
	}

	writeJSON(w, map[string]interface{}{
		"blocks": blocks,
		"total":  total,
		"page":   page,
		"limit":  limit,
		"pages":  int(math.Ceil(float64(total) / float64(limit))),
	})
}

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.db.Client().Ping(ctx, nil); err != nil {
		writeError(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]string{"status": "healthy"})
}

func parsePageParams(r *http.Request, defaultPage, defaultLimit int) (int, int) {
	page := defaultPage
	limit := defaultLimit

	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	return page, limit
}
