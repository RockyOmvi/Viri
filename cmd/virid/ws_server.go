package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
	"github.com/viri-chain/viri/internal/layer1/p2p"
	"github.com/viri-chain/viri/internal/pkg/observability"
	"github.com/viri-chain/viri/internal/pkg/security"
)

func wsCheckOrigin(r *http.Request) bool {
	allowedOrigins := map[string]bool{
		"http://localhost":          true,
		"http://localhost:3000":     true,
		"http://localhost:8080":     true,
		"http://localhost:8545":     true,
		"https://localhost":         true,
		"https://localhost:3000":    true,
		"https://localhost:8080":    true,
		"https://localhost:8545":    true,
		"http://127.0.0.1":          true,
		"http://127.0.0.1:3000":     true,
		"http://127.0.0.1:8080":     true,
		"http://127.0.0.1:8545":     true,
		"https://127.0.0.1":         true,
		"https://127.0.0.1:3000":    true,
		"https://127.0.0.1:8080":    true,
		"https://127.0.0.1:8545":    true,
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	return allowedOrigins[origin]
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     wsCheckOrigin,
}

type WSServer struct {
	port       int
	blockchain *ledger.PersistentBlockchain
	network    *p2p.ViriNetwork
	logger     *logging.Logger
	server     *http.Server
	clients    map[*WSClient]bool
	mu         sync.RWMutex
	tlsCert    string
	tlsKey     string
	apiKeyHash string
	drainer    *security.ConnectionDrainer
}

type WSClient struct {
	conn         *websocket.Conn
	send         chan []byte
	subscriptions map[string]bool
	mu           sync.RWMutex
}

func NewWSServer(port int, bc *ledger.PersistentBlockchain, net *p2p.ViriNetwork, log *logging.Logger, tlsCert, tlsKey, apiKeyHash string) *WSServer {
	return &WSServer{
		port:       port,
		blockchain: bc,
		network:    net,
		logger:     log,
		clients:    make(map[*WSClient]bool),
		tlsCert:    tlsCert,
		tlsKey:     tlsKey,
		apiKeyHash: apiKeyHash,
		drainer:    security.NewConnectionDrainer(30 * time.Second),
	}
}

func (s *WSServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/health", s.handleHealth)

	if s.apiKeyHash == "" {
		s.logger.Warn("WebSocket API key auth is disabled")
	}

	getClientID := func(r *http.Request) string {
		return r.RemoteAddr
	}

	rateLimiter := security.NewRateLimiter(5.0, 10)
	connLimiter := security.NewConnectionLimiter(100, 10)

	handler := security.ConnectionLimitMiddleware(connLimiter, getClientID)(
		security.RateLimitMiddleware(rateLimiter, getClientID)(
			mux,
		),
	)

	handler = security.DrainMiddleware(handler, s.drainer)

	s.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		s.logger.WithField("port", s.port).Info("WebSocket server started")
		var err error
		if s.tlsCert != "" && s.tlsKey != "" {
			err = s.server.ListenAndServeTLS(s.tlsCert, s.tlsKey)
		} else {
			err = s.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			s.logger.Error(fmt.Sprintf("WebSocket server error: %v", err))
		}
	}()

	return nil
}

func (s *WSServer) Stop() error {
	if s.server != nil {
		s.drainer.StartDrain()

		if err := s.drainer.Wait(); err != nil {
			s.logger.WithField("error", err.Error()).Warn("Timeout waiting for WebSocket connections to drain")
		}

		return s.server.Close()
	}
	return nil
}

func (s *WSServer) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.WithField("error", err.Error()).Warn("WebSocket upgrade failed")
		return
	}

	client := &WSClient{
		conn:          conn,
		send:          make(chan []byte, 256),
		subscriptions: make(map[string]bool),
	}

	s.mu.Lock()
	s.clients[client] = true
	s.mu.Unlock()

	connID := r.RemoteAddr
	s.drainer.Track(connID)
	defer s.drainer.Release(connID)

	s.logger.WithField("remote", r.RemoteAddr).Info("WebSocket client connected")

	go s.writePump(client)
	go s.readPump(client)
}

func (s *WSServer) writePump(client *WSClient) {
	defer func() {
		client.conn.Close()
		s.mu.Lock()
		delete(s.clients, client)
		s.mu.Unlock()
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-client.send:
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(client.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *WSServer) readPump(client *WSClient) {
	defer func() {
		client.conn.Close()
		s.mu.Lock()
		delete(s.clients, client)
		s.mu.Unlock()
	}()

	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				s.logger.WithField("error", err.Error()).Warn("WebSocket read error")
			}
			break
		}

		var req struct {
			Action string   `json:"action"`
			Topics []string `json:"topics"`
		}

		if err := json.Unmarshal(message, &req); err != nil {
			s.sendError(client, "invalid request format")
			continue
		}

		switch req.Action {
		case "subscribe":
			client.mu.Lock()
			for _, topic := range req.Topics {
				client.subscriptions[topic] = true
			}
			client.mu.Unlock()
			s.sendResponse(client, map[string]interface{}{
				"type":     "subscription_confirmed",
				"topics":   req.Topics,
				"timestamp": time.Now().Unix(),
			})
		case "unsubscribe":
			client.mu.Lock()
			for _, topic := range req.Topics {
				delete(client.subscriptions, topic)
			}
			client.mu.Unlock()
			s.sendResponse(client, map[string]interface{}{
				"type":     "unsubscribed",
				"topics":   req.Topics,
				"timestamp": time.Now().Unix(),
			})
		default:
			s.sendError(client, "unknown action")
		}
	}
}

func (s *WSServer) BroadcastBlock(block *ledger.Block) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type":   "new_block",
		"height": block.Header.Height,
		"hash":   fmt.Sprintf("0x%x", block.Hash()),
		"txs":    len(block.Transactions),
		"block":  formatBlock(block),
	})

	s.mu.RLock()
	for client := range s.clients {
		client.mu.RLock()
		if client.subscriptions["new_blocks"] || client.subscriptions["*"] {
			select {
			case client.send <- msg:
			default:
			}
		}
		client.mu.RUnlock()
	}
	s.mu.RUnlock()
}

func (s *WSServer) BroadcastTransaction(tx *ledger.Transaction) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type": "new_transaction",
		"hash": fmt.Sprintf("0x%x", tx.Hash),
		"from": fmt.Sprintf("0x%x", tx.SenderAddress()),
		"to":   fmt.Sprintf("0x%x", tx.To),
		"value": tx.Value,
	})

	s.mu.RLock()
	for client := range s.clients {
		client.mu.RLock()
		if client.subscriptions["new_transactions"] || client.subscriptions["*"] {
			select {
			case client.send <- msg:
			default:
			}
		}
		client.mu.RUnlock()
	}
	s.mu.RUnlock()
}

func (s *WSServer) BroadcastPeerChange(count int) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type":  "peer_change",
		"peers": count,
	})

	s.mu.RLock()
	for client := range s.clients {
		client.mu.RLock()
		if client.subscriptions["peers"] || client.subscriptions["*"] {
			select {
			case client.send <- msg:
			default:
			}
		}
		client.mu.RUnlock()
	}
	s.mu.RUnlock()
}

func (s *WSServer) sendResponse(client *WSClient, data interface{}) {
	msg, _ := json.Marshal(data)
	select {
	case client.send <- msg:
	default:
	}
}

func (s *WSServer) sendError(client *WSClient, errMsg string) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type":  "error",
		"error": errMsg,
	})
	select {
	case client.send <- msg:
	default:
	}
}

func (s *WSServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	observability.SetReady("ws", true)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"clients": len(s.clients),
	})
}
