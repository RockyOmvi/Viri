package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	sdk "github.com/viri-chain/viri/pkg/sdk"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	rpcEndpoint := flag.String("rpc", "http://localhost:8545", "RPC endpoint")
	apiAddr := flag.String("api", ":8547", "API listen address")
	mongoURI := flag.String("mongo", "mongodb://localhost:27017", "MongoDB URI")
	mongoDB := flag.String("db", "viri_indexer", "MongoDB database name")
	syncInterval := flag.Duration("interval", 2*time.Second, "Sync interval")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mongoCtx, mongoCancel := context.WithTimeout(ctx, 10*time.Second)
	mongoClient, err := mongo.Connect(mongoCtx, options.Client().ApplyURI(*mongoURI))
	mongoCancel()
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(ctx)

	if err := mongoClient.Ping(ctx, nil); err != nil {
		log.Fatalf("MongoDB ping failed: %v", err)
	}
	log.Printf("Connected to MongoDB at %s", *mongoURI)

	db := mongoClient.Database(*mongoDB)
	sdkClient := sdk.NewClient(*rpcEndpoint)
	syncer := NewSyncer(sdkClient, db, *syncInterval)
	apiServer := NewAPIServer(*apiAddr, db)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go syncer.Start(ctx)

	go func() {
		log.Printf("Indexer syncer running (interval: %v)", *syncInterval)
	}()

	go func() {
		if err := apiServer.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API server error: %v", err)
		}
	}()

	log.Printf("Indexer service started. RPC: %s, API: %s, MongoDB: %s/%s",
		*rpcEndpoint, *apiAddr, *mongoURI, *mongoDB)

	<-sigCh
	log.Println("Shutting down...")

	syncer.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := apiServer.Stop(shutdownCtx); err != nil {
		log.Printf("API server shutdown error: %v", err)
	}

	cancel()
	mongoClient.Disconnect(context.Background())
	log.Println("Indexer stopped")
}
