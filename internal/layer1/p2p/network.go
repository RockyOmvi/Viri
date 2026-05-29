package p2p

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	discutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/viri-chain/viri/internal/layer1/crypto"
	"github.com/viri-chain/viri/internal/layer1/ledger"
	"github.com/viri-chain/viri/internal/layer1/logging"
	"github.com/viri-chain/viri/internal/pkg/metrics"
	"github.com/viri-chain/viri/internal/pkg/security"
)

const (
	ProtocolIDPrefix  protocol.ID = "/viri/1.0.0/"
	BlockTopic        protocol.ID = "/viri/blocks/1.0.0"
	TxTopic           protocol.ID = "/viri/txs/1.0.0"
	HeaderTopic       protocol.ID = "/viri/headers/1.0.0"
	ConsensusTopic    protocol.ID = "/viri/consensus/1.0.0"
	BlockSyncProtocol protocol.ID = ProtocolIDPrefix + "blocksync"
	TxSyncProtocol    protocol.ID = ProtocolIDPrefix + "txsync"
	PeerSyncProtocol  protocol.ID = ProtocolIDPrefix + "peersync"
	ConsensusProtocol protocol.ID = ProtocolIDPrefix + "consensus"
)

type NetworkConfig struct {
	ListenAddr   string
	ExternalAddr string
	Bootstraps   []string
	EnableDHT    bool
	EnableMDNS   bool
	EnablePubSub bool
	ChainID      uint64
	MaxPeers     int
	MinPeers     int
	Rendezvous   string
}

func DefaultNetworkConfig() *NetworkConfig {
	return &NetworkConfig{
		ListenAddr:   "/ip4/0.0.0.0/tcp/30303",
		Bootstraps:   []string{},
		EnableDHT:    true,
		EnableMDNS:   true,
		EnablePubSub: true,
		ChainID:      1337,
		MaxPeers:     50,
		MinPeers:     5,
		Rendezvous:   "viri-chain-dev",
	}
}

type ViriNetwork struct {
	ctx         context.Context
	cancel      context.CancelFunc
	host        host.Host
	dht         *dht.IpfsDHT
	pubsub      *pubsub.PubSub
	blockTopic  *pubsub.Topic
	txTopic     *pubsub.Topic
	headerTopic *pubsub.Topic
	consensusTopic *pubsub.Topic
	peerManager *PeerManager
	connManager *ConnManager
	rateLimiter *RateLimiter
	securityLimiter *security.RateLimiter
	dosProtector   *security.DoSProtector
	propagator  *Propagator
	validator   *MessageValidator
	stats       *NetworkStats
	config      *NetworkConfig
	logger      *logging.Logger
	blockchain  *ledger.PersistentBlockchain
	messageHandler MessageHandler
	consensusHandler func(*Message, peer.ID)
	authenticator   *MessageAuthenticator
	validatorAddress []byte
	validatorPubKey  []byte
	privKey         *crypto.PrivateKey
	mu          sync.Mutex
	running     bool
	onValidatorDiscovered func(pubKey []byte, addr []byte, validatorPubKey []byte)
	metrics     *metrics.MetricsCollector
}

func (n *ViriNetwork) SetMetrics(mc *metrics.MetricsCollector) {
	n.metrics = mc
}

type MessageHandler interface {
	OnBlock(msg *Message, from peer.ID) error
	OnTransaction(msg *Message, from peer.ID) error
	OnGetBlocks(msg *Message, from peer.ID) error
	OnGetHeaders(msg *Message, from peer.ID) error
	OnAnnounce(msg *Message, from peer.ID) error
}

type SimpleMessageHandler struct {
	OnBlockHandler       func(*Message, peer.ID) error
	OnTransactionHandler func(*Message, peer.ID) error
	OnGetBlocksHandler   func(*Message, peer.ID) error
	OnGetHeadersHandler  func(*Message, peer.ID) error
	OnAnnounceHandler    func(*Message, peer.ID) error
}

func (h *SimpleMessageHandler) OnBlock(msg *Message, from peer.ID) error {
	if h.OnBlockHandler != nil {
		return h.OnBlockHandler(msg, from)
	}
	return nil
}

func (h *SimpleMessageHandler) OnTransaction(msg *Message, from peer.ID) error {
	if h.OnTransactionHandler != nil {
		return h.OnTransactionHandler(msg, from)
	}
	return nil
}

func (h *SimpleMessageHandler) OnGetBlocks(msg *Message, from peer.ID) error {
	if h.OnGetBlocksHandler != nil {
		return h.OnGetBlocksHandler(msg, from)
	}
	return nil
}

func (h *SimpleMessageHandler) OnGetHeaders(msg *Message, from peer.ID) error {
	if h.OnGetHeadersHandler != nil {
		return h.OnGetHeadersHandler(msg, from)
	}
	return nil
}

func (h *SimpleMessageHandler) OnAnnounce(msg *Message, from peer.ID) error {
	if h.OnAnnounceHandler != nil {
		return h.OnAnnounceHandler(msg, from)
	}
	return nil
}

func NewViriNetwork(config *NetworkConfig, bc *ledger.PersistentBlockchain, log *logging.Logger, privKey *crypto.PrivateKey) (*ViriNetwork, error) {
	if config == nil {
		config = DefaultNetworkConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	if privKey == nil {
		var err error
		privKey, err = crypto.GenerateKey()
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to generate crypto key: %w", err)
		}
	}

	authenticator := NewMessageAuthenticator(privKey, config.ChainID, DefaultMaxMessageAge)

	n := &ViriNetwork{
		ctx:        ctx,
		cancel:     cancel,
		config:     config,
		blockchain: bc,
		logger:     log,
		privKey:    privKey,
		authenticator: authenticator,
		peerManager: NewPeerManager(&PeerManagerConfig{
			MaxPeers:    config.MaxPeers,
			MinPeers:    config.MinPeers,
			BanDuration: 24 * time.Hour,
		}),
		connManager: NewConnManager(&ConnManagerConfig{
			MaxConnections:    config.MaxPeers,
			MinConnections:    config.MinPeers,
			GracePeriod:       30 * time.Second,
			HealthCheckInterval: 30 * time.Second,
		}),
		rateLimiter: NewRateLimiter(nil),
		securityLimiter: security.NewRateLimiter(1000, 1000),
		dosProtector:   security.NewDoSProtector(nil, func() {
			if log != nil {
				log.Warn("Emergency shutdown triggered: potential DoS attack detected")
			}
		}),
		propagator:  NewPropagator(0, 0),
		validator:   NewMessageValidator(config.ChainID),
		stats:       NewNetworkStats(),
	}

	return n, nil
}

func (n *ViriNetwork) Start() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.logger.Info("Initializing libp2p host")

	var (
		privKey libp2pcrypto.PrivKey
		err     error
	)
	if n.privKey != nil {
		rawKey := n.privKey.PrivateBytes()
		privKey, err = libp2pcrypto.UnmarshalSecp256k1PrivateKey(rawKey)
		if err != nil {
			return fmt.Errorf("failed to create libp2p key from node key: %w", err)
		}
		n.logger.Info("Using persistent node key for libp2p identity")
	} else {
		privKey, _, err = libp2pcrypto.GenerateSecp256k1Key(rand.Reader)
		if err != nil {
			return fmt.Errorf("failed to generate libp2p key: %w", err)
		}
	}

	opts := []libp2p.Option{
		libp2p.Identity(privKey),
		libp2p.ListenAddrStrings(n.config.ListenAddr),
		libp2p.UserAgent("viri/0.1.0"),
		libp2p.Ping(true),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return fmt.Errorf("failed to create libp2p host: %w", err)
	}

	n.host = h

	n.host.Network().Notify(n.connManager.Notifee())
	n.connManager.SetHost(n.host.Network())
	n.connManager.Start()

	n.logger.Info("libp2p host created")
	n.logger.Info(fmt.Sprintf("libp2p host details peer_id=%s addrs=%v", h.ID().String(), h.Addrs()))
	n.logger.Info(fmt.Sprintf("Message auth peer_id=%s", n.authenticator.GetPeerID()))

	n.peerManager.Subscribe(func(event PeerEvent) {
		n.logger.Debug(fmt.Sprintf("Peer event event=%s peer=%s reason=%s", string(event.Type), event.Peer.ID.String(), event.Reason))
	})

	if n.config.EnableDHT {
		if err := n.setupDHT(); err != nil {
			return fmt.Errorf("failed to setup DHT: %w", err)
		}
	}

	if n.config.EnablePubSub {
		if err := n.setupPubSub(); err != nil {
			return fmt.Errorf("failed to setup pubsub: %w", err)
		}
	}

	if n.config.EnableMDNS {
		n.setupMDNS()
	}

	n.setupStreamHandlers()

	if n.dht != nil && n.config.Rendezvous != "" {
		routingDiscovery := routing.NewRoutingDiscovery(n.dht)
		discutil.Advertise(n.ctx, routingDiscovery, n.config.Rendezvous)
		n.logger.Info(fmt.Sprintf("Advertising at rendezvous point: %s", n.config.Rendezvous))

		peers, err := routingDiscovery.FindPeers(n.ctx, n.config.Rendezvous)
		if err != nil {
			n.logger.Warn(fmt.Sprintf("Rendezvous discovery error: %v", err))
	} else {
		go func() {
			for pi := range peers {
				if pi.ID == n.host.ID() || len(pi.Addrs) == 0 {
					continue
				}
				ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
				if err := n.host.Connect(ctx, pi); err == nil {
					n.peerManager.AddPeer(pi.ID, pi.Addrs[0], network.DirOutbound)
					n.logger.Info(fmt.Sprintf("Connected via rendezvous peer=%s", pi.ID.String()[:12]+"..."))
				} else {
					n.logger.Debug(fmt.Sprintf("Rendezvous connect failed peer=%s error=%v", pi.ID.String()[:12]+"...", err))
				}
				cancel()
			}
		}()
	}
	}

	if err := n.connectToBootstrap(); err != nil {
		n.logger.Warn(fmt.Sprintf("Failed to connect to bootstrap peers error=%v", err))
	}

	n.startPeerDiscoveryLoop()
	n.startHandshakeLoop()

	n.running = true
	n.logger.Info(fmt.Sprintf("Viri network started peer_id=%s auth_id=%s", n.ShortPeerID(), n.authenticator.GetPeerID()))

	return nil
}

func (n *ViriNetwork) setupDHT() error {
	n.logger.Info("Setting up DHT")

	kdht, err := dht.New(n.ctx, n.host,
		dht.Mode(dht.ModeAuto),
		dht.ProtocolPrefix("/viri"),
	)
	if err != nil {
		return err
	}

	n.dht = kdht

	if err := n.dht.Bootstrap(n.ctx); err != nil {
		return err
	}

	return nil
}

func (n *ViriNetwork) setupPubSub() error {
	n.logger.Info("Setting up gossipsub")

	params := pubsub.DefaultGossipSubParams()
	params.HeartbeatInterval = 200 * time.Millisecond
	params.FanoutTTL = 2 * time.Second
	params.D = 8
	params.Dlo = 6
	params.Dhi = 12
	params.Dout = 3

	ps, err := pubsub.NewGossipSub(n.ctx, n.host,
		pubsub.WithPeerScore(
			&pubsub.PeerScoreParams{
				Topics:        make(map[string]*pubsub.TopicScoreParams),
				DecayInterval: time.Second,
				DecayToZero:   0.01,
				AppSpecificScore: func(p peer.ID) float64 {
					return 0
				},
				AppSpecificWeight: 0,
			},
			&pubsub.PeerScoreThresholds{
				GossipThreshold:             -100,
				PublishThreshold:            -500,
				GraylistThreshold:           -1000,
				AcceptPXThreshold:           100,
			},
		),
		pubsub.WithFloodPublish(true),
		pubsub.WithDirectPeers(nil),
		pubsub.WithDirectConnectTicks(2),
		pubsub.WithGossipSubParams(params),
	)
	if err != nil {
		return err
	}

	n.pubsub = ps

	blockTopic, err := ps.Join(string(BlockTopic))
	if err != nil {
		return err
	}
	n.blockTopic = blockTopic

	txTopic, err := ps.Join(string(TxTopic))
	if err != nil {
		return err
	}
	n.txTopic = txTopic

	headerTopic, err := ps.Join(string(HeaderTopic))
	if err != nil {
		return err
	}
	n.headerTopic = headerTopic

	consensusTopic, err := ps.Join(string(ConsensusTopic))
	if err != nil {
		return err
	}
	n.consensusTopic = consensusTopic

	return nil
}

func (n *ViriNetwork) setupMDNS() {
	n.logger.Info("Setting up mDNS discovery")

	service := mdns.NewMdnsService(n.host, "viri-chain", &mdnsListener{network: n})
	if err := service.Start(); err != nil {
		n.logger.Warn(fmt.Sprintf("Failed to start mDNS error=%v", err))
	}
}

type mdnsListener struct {
	network *ViriNetwork
}

func (l *mdnsListener) HandlePeerFound(pi peer.AddrInfo) {
	l.network.logger.Debug(fmt.Sprintf("mDNS peer discovered peer=%s", pi.ID.String()))
	ctx, cancel := context.WithTimeout(l.network.ctx, 10*time.Second)
	defer cancel()

	if err := l.network.host.Connect(ctx, pi); err != nil {
		l.network.logger.Debug(fmt.Sprintf("Failed to connect to mDNS peer error=%v", err))
	}
}

func (n *ViriNetwork) setupStreamHandlers() {
	n.host.SetStreamHandler(BlockSyncProtocol, n.handleBlockSync)
	n.host.SetStreamHandler(TxSyncProtocol, n.handleTxSync)
	n.host.SetStreamHandler(PeerSyncProtocol, n.handlePeerSync)
	n.host.SetStreamHandler(protocol.ID("/viri/ping/1.0.0"), n.handlePing)
	n.host.SetStreamHandler(protocol.ID("/viri/handshake/1.0.0"), n.handleHandshake)
	n.host.SetStreamHandler(ConsensusProtocol, n.handleConsensusStream)
}

func (n *ViriNetwork) connectToBootstrap() error {
	if len(n.config.Bootstraps) == 0 {
		return nil
	}

	n.logger.Info(fmt.Sprintf("Connecting to bootstrap peers count=%d", len(n.config.Bootstraps)))

	for _, addr := range n.config.Bootstraps {
		ai, err := peer.AddrInfoFromString(addr)
		if err != nil {
			n.logger.Warn(fmt.Sprintf("Invalid bootstrap address addr=%s error=%v", addr, err))
			continue
		}

		ctx, cancel := context.WithTimeout(n.ctx, 30*time.Second)
		err = n.host.Connect(ctx, *ai)
		cancel()

		if err != nil {
			n.logger.Warn(fmt.Sprintf("Failed to connect to bootstrap peer peer=%s error=%v", ai.ID.String(), err))
			continue
		}

		if len(ai.Addrs) > 0 {
			n.peerManager.AddPeer(ai.ID, ai.Addrs[0], network.DirOutbound)
		}
		n.logger.Info(fmt.Sprintf("Connected to bootstrap peer peer=%s", ai.ID.String()))
	}

	return nil
}

func (n *ViriNetwork) PublishBlock(blockData []byte) error {
	if n.blockTopic == nil {
		return fmt.Errorf("block topic not initialized")
	}

	msg := NewMessage(MsgBlock, blockData)
	signedMsg, err := n.authenticator.Sign(msg)
	if err != nil {
		return fmt.Errorf("failed to sign block message: %w", err)
	}

	encoded, err := signedMsg.Encode()
	if err != nil {
		return err
	}

	hash := HashFromPayload(blockData)
	connectedPeers := n.peerManager.GetConnectedPeers()
	peerIDs := make([]peer.ID, 0, len(connectedPeers))
	for _, p := range connectedPeers {
		peerIDs = append(peerIDs, p.ID)
	}
	n.propagator.AddPendingBlock(hash, peerIDs)

	n.stats.RecordMessageOut(len(encoded))
	n.stats.RecordBlockOut()

	if n.metrics != nil {
		n.metrics.IncP2PMessagesOut()
		n.metrics.AddP2PBytesOut(len(encoded))
	}

	return n.blockTopic.Publish(n.ctx, encoded)
}

func (n *ViriNetwork) PublishTransaction(txData []byte) error {
	if n.txTopic == nil {
		return fmt.Errorf("tx topic not initialized")
	}

	msg := NewMessage(MsgTransaction, txData)
	signedMsg, err := n.authenticator.Sign(msg)
	if err != nil {
		return fmt.Errorf("failed to sign transaction message: %w", err)
	}

	encoded, err := signedMsg.Encode()
	if err != nil {
		return err
	}

	hash := HashFromPayload(txData)
	connectedPeers := n.peerManager.GetConnectedPeers()
	peerIDs := make([]peer.ID, 0, len(connectedPeers))
	for _, p := range connectedPeers {
		peerIDs = append(peerIDs, p.ID)
	}
	n.propagator.AddPendingTx(hash, peerIDs)

	n.stats.RecordMessageOut(len(encoded))
	n.stats.RecordTxOut()

	if n.metrics != nil {
		n.metrics.IncP2PMessagesOut()
		n.metrics.AddP2PBytesOut(len(encoded))
	}

	return n.txTopic.Publish(n.ctx, encoded)
}

func (n *ViriNetwork) PublishHeader(headerData []byte) error {
	if n.headerTopic == nil {
		return fmt.Errorf("header topic not initialized")
	}

	msg := NewMessage(MsgBlockHeader, headerData)
	signedMsg, err := n.authenticator.Sign(msg)
	if err != nil {
		return fmt.Errorf("failed to sign header message: %w", err)
	}

	encoded, err := signedMsg.Encode()
	if err != nil {
		return err
	}

	n.stats.RecordMessageOut(len(encoded))

	return n.headerTopic.Publish(n.ctx, encoded)
}

func (n *ViriNetwork) SubscribeToBlocks(handler func(*Message, peer.ID)) error {
	if n.blockTopic == nil {
		return fmt.Errorf("block topic not initialized")
	}

	sub, err := n.blockTopic.Subscribe()
	if err != nil {
		return err
	}

	go func() {
		defer sub.Cancel()
		for {
			msg, err := sub.Next(n.ctx)
			if err != nil {
				return
			}

			if msg.ReceivedFrom == n.host.ID() {
				continue
			}

			peerID := msg.ReceivedFrom

	if n.rateLimiter.IsBlocked(peerID) {
			n.stats.RecordRejected()
			continue
		}

		if n.securityLimiter.IsBlocked(peerID) {
			n.stats.RecordRejected()
			continue
		}

		allowed, reason := n.securityLimiter.AllowPeer(peerID, len(msg.Data), security.MsgTypeBlockRequest, msg.Data)
		if !allowed {
			n.logger.Debug(fmt.Sprintf("Security rate limit: peer=%s reason=%s", peerID.String(), reason))
			n.stats.RecordRejected()
			continue
		}

		if !n.rateLimiter.Allow(peerID, len(msg.Data)) {
			n.stats.RecordRejected()
			continue
		}

		signedMsg, err := DecodeSignedMessage(msg.Data)
		if err != nil {
			n.stats.RecordDropped()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		if err := n.authenticator.Verify(signedMsg); err != nil {
			n.logger.Debug(fmt.Sprintf("Block message verification failed peer=%s error=%v", peerID.String(), err))
			n.stats.RecordRejected()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		message, err := DecodeMessage(signedMsg.MessageBytes)
		if err != nil {
			n.stats.RecordDropped()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		result, err := n.validator.Validate(message)
		if err != nil || result == ValidationReject {
			n.stats.RecordRejected()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		n.securityLimiter.ReportValidMsg(peerID)

		hash := HashFromPayload(message.Payload)
		if n.propagator.IsBlockSeen(hash) {
			continue
		}
		n.propagator.MarkBlockSeen(hash, peerID)

		n.stats.RecordMessageIn(len(msg.Data))
		n.stats.RecordBlockIn()

		if n.metrics != nil {
			n.metrics.IncP2PMessagesIn()
			n.metrics.AddP2PBytesIn(len(msg.Data))
		}

		if n.messageHandler != nil {
			n.messageHandler.OnBlock(message, peerID)
		} else if handler != nil {
			handler(message, peerID)
		}
		}
	}()

	return nil
}

func (n *ViriNetwork) SubscribeToTransactions(handler func(*Message, peer.ID)) error {
	if n.txTopic == nil {
		return fmt.Errorf("tx topic not initialized")
	}

	sub, err := n.txTopic.Subscribe()
	if err != nil {
		return err
	}

	go func() {
		defer sub.Cancel()
		for {
			msg, err := sub.Next(n.ctx)
			if err != nil {
				return
			}

			if msg.ReceivedFrom == n.host.ID() {
				continue
			}

			peerID := msg.ReceivedFrom

	if n.rateLimiter.IsBlocked(peerID) {
			n.stats.RecordRejected()
			continue
		}

		if n.securityLimiter.IsBlocked(peerID) {
			n.stats.RecordRejected()
			continue
		}

		allowed, reason := n.securityLimiter.AllowPeer(peerID, len(msg.Data), security.MsgTypeGeneral, msg.Data)
		if !allowed {
			n.logger.Debug(fmt.Sprintf("Security rate limit: peer=%s reason=%s", peerID.String(), reason))
			n.stats.RecordRejected()
			continue
		}

		if !n.rateLimiter.Allow(peerID, len(msg.Data)) {
			n.stats.RecordRejected()
			continue
		}

		signedMsg, err := DecodeSignedMessage(msg.Data)
		if err != nil {
			n.stats.RecordDropped()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		if err := n.authenticator.Verify(signedMsg); err != nil {
			n.logger.Debug(fmt.Sprintf("Transaction message verification failed peer=%s error=%v", peerID.String(), err))
			n.stats.RecordRejected()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		message, err := DecodeMessage(signedMsg.MessageBytes)
		if err != nil {
			n.stats.RecordDropped()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		result, err := n.validator.Validate(message)
		if err != nil || result == ValidationReject {
			n.stats.RecordRejected()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		n.securityLimiter.ReportValidMsg(peerID)

		hash := HashFromPayload(message.Payload)
		if n.propagator.IsTxSeen(hash) {
			continue
		}
		n.propagator.MarkTxSeen(hash, peerID)

		n.stats.RecordMessageIn(len(msg.Data))
		n.stats.RecordTxIn()

		if n.metrics != nil {
			n.metrics.IncP2PMessagesIn()
			n.metrics.AddP2PBytesIn(len(msg.Data))
		}

		if n.messageHandler != nil {
			n.messageHandler.OnTransaction(message, peerID)
		} else if handler != nil {
			handler(message, peerID)
		}
		}
	}()

	return nil
}

func (n *ViriNetwork) SubscribeToHeaders(handler func(*Message, peer.ID)) error {
	if n.headerTopic == nil {
		return fmt.Errorf("header topic not initialized")
	}

	sub, err := n.headerTopic.Subscribe()
	if err != nil {
		return err
	}

	go func() {
		defer sub.Cancel()
		for {
			msg, err := sub.Next(n.ctx)
			if err != nil {
				return
			}

			if msg.ReceivedFrom == n.host.ID() {
				continue
			}

		peerID := msg.ReceivedFrom

		if n.rateLimiter.IsBlocked(peerID) {
			n.stats.RecordRejected()
			continue
		}

		if n.securityLimiter.IsBlocked(peerID) {
			n.stats.RecordRejected()
			continue
		}

		allowed, reason := n.securityLimiter.AllowPeer(peerID, len(msg.Data), security.MsgTypeGeneral, msg.Data)
		if !allowed {
			n.logger.Debug(fmt.Sprintf("Security rate limit: peer=%s reason=%s", peerID.String(), reason))
			n.stats.RecordRejected()
			continue
		}

		if !n.rateLimiter.Allow(peerID, len(msg.Data)) {
			n.stats.RecordRejected()
			continue
		}

		signedMsg, err := DecodeSignedMessage(msg.Data)
		if err != nil {
			n.stats.RecordDropped()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		if err := n.authenticator.Verify(signedMsg); err != nil {
			n.logger.Debug(fmt.Sprintf("Header message verification failed peer=%s error=%v", peerID.String(), err))
			n.stats.RecordRejected()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		message, err := DecodeMessage(signedMsg.MessageBytes)
		if err != nil {
			n.logger.Info(fmt.Sprintf("Consensus message decode failed peer=%s error=%v", peerID.String(), err))
			n.stats.RecordDropped()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		result, err := n.validator.Validate(message)
		if err != nil || result == ValidationReject {
			n.logger.Info(fmt.Sprintf("Consensus message validation failed peer=%s type=%d error=%v", peerID.String(), message.Type, err))
			n.stats.RecordRejected()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		n.securityLimiter.ReportValidMsg(peerID)

		n.stats.RecordMessageIn(len(msg.Data))

		if n.metrics != nil {
			n.metrics.IncP2PMessagesIn()
			n.metrics.AddP2PBytesIn(len(msg.Data))
		}

		if handler != nil {
			handler(message, peerID)
		}
		}
	}()

	return nil
}

func (n *ViriNetwork) PublishConsensus(data []byte) error {
	if n.consensusTopic == nil {
		return fmt.Errorf("consensus topic not initialized")
	}

	msg := NewMessage(MsgProposal, data)
	signedMsg, err := n.authenticator.Sign(msg)
	if err != nil {
		return fmt.Errorf("failed to sign consensus message: %w", err)
	}

	encoded, err := signedMsg.Encode()
	if err != nil {
		return err
	}

	n.stats.RecordMessageOut(len(encoded))

	if n.metrics != nil {
		n.metrics.IncP2PMessagesOut()
		n.metrics.AddP2PBytesOut(len(encoded))
	}

	return n.consensusTopic.Publish(n.ctx, encoded)
}

func (n *ViriNetwork) sendConsensusDirect(msg *Message, signedMsg *SignedMessage) {
	peers := n.host.Network().Peers()
	for _, p := range peers {
		if p == n.host.ID() {
			continue
		}
		encoded, err := signedMsg.Encode()
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(n.ctx, 2*time.Second)
		stream, err := n.host.NewStream(ctx, p, ConsensusProtocol)
		cancel()
		if err != nil {
			continue
		}
		stream.SetDeadline(time.Now().Add(2 * time.Second))
		stream.Write(encoded)
		stream.Close()
	}
}

func (n *ViriNetwork) SubscribeToConsensus(handler func(*Message, peer.ID)) error {
	if n.consensusTopic == nil {
		return fmt.Errorf("consensus topic not initialized")
	}

	n.consensusHandler = handler

	sub, err := n.consensusTopic.Subscribe()
	if err != nil {
		return err
	}

	go func() {
		defer sub.Cancel()
		for {
			msg, err := sub.Next(n.ctx)
			if err != nil {
				return
			}

			if msg.ReceivedFrom == n.host.ID() {
				continue
			}

		peerID := msg.ReceivedFrom

		n.logger.Debug(fmt.Sprintf("Consensus msg received from=%s size=%d", peerID.String()[:16], len(msg.Data)))

		if n.rateLimiter.IsBlocked(peerID) {
			n.logger.Debug(fmt.Sprintf("Consensus msg REJECTED: rate limiter blocked peer=%s", peerID.String()[:16]))
			n.stats.RecordRejected()
			continue
		}

		if n.securityLimiter.IsBlocked(peerID) {
			n.logger.Debug(fmt.Sprintf("Consensus msg REJECTED: security blocked peer=%s", peerID.String()[:16]))
			n.stats.RecordRejected()
			continue
		}

		allowed, reason := n.securityLimiter.AllowPeer(peerID, len(msg.Data), security.MsgTypeConsensus, msg.Data)
		if !allowed {
			n.logger.Debug(fmt.Sprintf("Consensus msg REJECTED: security limiter peer=%s reason=%s", peerID.String()[:16], reason))
			n.stats.RecordRejected()
			continue
		}

		if !n.rateLimiter.Allow(peerID, len(msg.Data)) {
			n.logger.Debug(fmt.Sprintf("Consensus msg REJECTED: rate limit peer=%s", peerID.String()[:16]))
			n.stats.RecordRejected()
			continue
		}

		signedMsg, err := DecodeSignedMessage(msg.Data)
		if err != nil {
			n.logger.Debug(fmt.Sprintf("Consensus msg DROPPED: decode error peer=%s err=%v", peerID.String()[:16], err))
			n.stats.RecordDropped()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		if err := n.authenticator.Verify(signedMsg); err != nil {
			n.logger.Info(fmt.Sprintf("Consensus message verification failed peer=%s error=%v", peerID.String(), err))
			n.stats.RecordRejected()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		message, err := DecodeMessage(signedMsg.MessageBytes)
		if err != nil {
			n.logger.Debug(fmt.Sprintf("Consensus msg DROPPED: message decode error peer=%s err=%v", peerID.String()[:16], err))
			n.stats.RecordDropped()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		result, err := n.validator.Validate(message)
		if err != nil || result == ValidationReject {
			n.logger.Debug(fmt.Sprintf("Consensus msg REJECTED: validation failed peer=%s err=%v", peerID.String()[:16], err))
			n.stats.RecordRejected()
			n.securityLimiter.ReportInvalidMsg(peerID)
			continue
		}

		n.logger.Debug(fmt.Sprintf("Consensus msg PASSED all checks type=%d payload_size=%d from=%s", message.Type, len(message.Payload), peerID.String()[:16]))

		n.securityLimiter.ReportValidMsg(peerID)

		n.stats.RecordMessageIn(len(msg.Data))

		if n.metrics != nil {
			n.metrics.IncP2PMessagesIn()
			n.metrics.AddP2PBytesIn(len(msg.Data))
		}

		if handler != nil {
			handler(message, peerID)
		}
		}
	}()

	return nil
}

func (n *ViriNetwork) handleConsensusStream(stream network.Stream) {
	defer stream.Close()

	remotePeer := stream.Conn().RemotePeer()

	if n.rateLimiter.IsBlocked(remotePeer) {
		n.stats.RecordRejected()
		stream.Reset()
		return
	}

	if n.securityLimiter.IsBlocked(remotePeer) {
		n.stats.RecordRejected()
		stream.Reset()
		return
	}

	buf := make([]byte, MaxMessageSize)
	bytesRead, err := stream.Read(buf)
	if err != nil {
		return
	}

	allowed, reason := n.securityLimiter.AllowPeer(remotePeer, bytesRead, security.MsgTypeConsensus, buf[:bytesRead])
	if !allowed {
		n.logger.Debug(fmt.Sprintf("Security rate limit: peer=%s reason=%s", remotePeer.String(), reason))
		n.stats.RecordRejected()
		return
	}

	if !n.rateLimiter.Allow(remotePeer, bytesRead) {
		n.stats.RecordRejected()
		return
	}

	n.stats.RecordMessageIn(bytesRead)

	signedMsg, err := DecodeSignedMessage(buf[:bytesRead])
	if err != nil {
		n.peerManager.UpdateScore(remotePeer, -5)
		n.securityLimiter.ReportInvalidMsg(remotePeer)
		return
	}

	if err := n.authenticator.Verify(signedMsg); err != nil {
		n.logger.Info(fmt.Sprintf("Consensus stream verification failed peer=%s error=%v", remotePeer.String(), err))
		n.stats.RecordRejected()
		n.securityLimiter.ReportInvalidMsg(remotePeer)
		return
	}

	msg, err := DecodeMessage(signedMsg.MessageBytes)
	if err != nil {
		n.peerManager.UpdateScore(remotePeer, -5)
		n.stats.RecordDropped()
		n.securityLimiter.ReportInvalidMsg(remotePeer)
		return
	}

	result, err := n.validator.Validate(msg)
	if err != nil || result == ValidationReject {
		n.stats.RecordRejected()
		n.peerManager.UpdateScore(remotePeer, -5)
		n.securityLimiter.ReportInvalidMsg(remotePeer)
		return
	}

	n.securityLimiter.ReportValidMsg(remotePeer)

	if n.consensusHandler != nil {
		n.consensusHandler(msg, remotePeer)
	}
}

func (n *ViriNetwork) FindPeers(ctx context.Context, count int) ([]peer.AddrInfo, error) {
	if n.dht == nil {
		return nil, fmt.Errorf("DHT not initialized")
	}

	var peers []peer.AddrInfo
	for _, p := range n.dht.RoutingTable().ListPeers() {
		if p == n.host.ID() {
			continue
		}
		addrInfo := peer.AddrInfo{ID: p}
		peers = append(peers, addrInfo)
		if len(peers) >= count {
			break
		}
	}

	return peers, nil
}

func (n *ViriNetwork) SendToPeer(ctx context.Context, peerID peer.ID, msg *Message) error {
	var proto protocol.ID
	switch msg.Type {
	case MsgBlock, MsgGetBlocks, MsgBlockHeader, MsgGetHeaders, MsgAnnounce, MsgBlockRequest, MsgBlockResponse:
		proto = BlockSyncProtocol
	case MsgTransaction:
		proto = TxSyncProtocol
	default:
		proto = BlockSyncProtocol
	}

	stream, err := n.host.NewStream(ctx, peerID, proto)
	if err != nil {
		return err
	}
	defer stream.Close()

	data, err := msg.Encode()
	if err != nil {
		return err
	}

	n.stats.RecordMessageOut(len(data))

	_, err = stream.Write(data)
	return err
}

func (n *ViriNetwork) SelectProtocol(msgType MessageType) protocol.ID {
	switch msgType {
	case MsgBlock, MsgGetBlocks, MsgBlockHeader, MsgGetHeaders, MsgAnnounce, MsgBlockRequest, MsgBlockResponse:
		return BlockSyncProtocol
	case MsgTransaction:
		return TxSyncProtocol
	case MsgGetPeers, MsgPeers:
		return PeerSyncProtocol
	default:
		return BlockSyncProtocol
	}
}

func (n *ViriNetwork) Peers() []*PeerInfo {
	if n.metrics != nil {
		n.metrics.SetP2PPeersConnected(len(n.peerManager.GetConnectedPeers()))
	}
	return n.peerManager.GetConnectedPeers()
}

func (n *ViriNetwork) PeerCount() int {
	return n.peerManager.PeerCount()
}

func (n *ViriNetwork) PeerID() peer.ID {
	if n.host == nil {
		return ""
	}
	return n.host.ID()
}

func (n *ViriNetwork) ShortPeerID() string {
	pid := n.PeerID()
	if pid == "" {
		return "unknown"
	}
	s := pid.String()
	if len(s) > 12 {
		return s[:12] + "..."
	}
	return s
}

func (n *ViriNetwork) Addresses() []multiaddr.Multiaddr {
	if n.host == nil {
		return nil
	}
	return n.host.Addrs()
}

func (n *ViriNetwork) SetValidatorAddress(addr []byte) {
	n.validatorAddress = append([]byte(nil), addr...)
}

func (n *ViriNetwork) SetValidatorPubKey(pubKey []byte) {
	n.validatorPubKey = append([]byte(nil), pubKey...)
}

func (n *ViriNetwork) WritePeerInfo(path string) error {
	info := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", n.config.listenPort(), n.PeerID())
	return os.WriteFile(path, []byte(info), 0644)
}

func (n *ViriNetwork) ReadAndConnectPeerInfo(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	addr := strings.TrimSpace(string(data))
	if addr == "" {
		return fmt.Errorf("empty peer info file")
	}

	ai, err := peer.AddrInfoFromString(addr)
	if err != nil {
		return fmt.Errorf("invalid peer info: %w", err)
	}

	ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
	defer cancel()

	if err := n.host.Connect(ctx, *ai); err != nil {
		return err
	}

	n.peerManager.AddPeer(ai.ID, ai.Addrs[0], network.DirOutbound)
	n.logger.Info(fmt.Sprintf("Connected to peer from file peer=%s", ai.ID.String()[:12]+"..."))
	return nil
}

func (n *ViriNetwork) ConnectPeer(maddr string) error {
	ai, err := peer.AddrInfoFromString(maddr)
	if err != nil {
		return fmt.Errorf("invalid multiaddress: %w", err)
	}

	// Filter out loopback addresses to avoid "dial to self" errors
	filteredAddrs := make([]multiaddr.Multiaddr, 0, len(ai.Addrs))
	for _, addr := range ai.Addrs {
		addrStr := addr.String()
		if strings.Contains(addrStr, "127.0.0.1") || strings.Contains(addrStr, "0.0.0.0") || strings.Contains(addrStr, "localhost") {
			continue
		}
		filteredAddrs = append(filteredAddrs, addr)
	}

	if len(filteredAddrs) == 0 {
		return fmt.Errorf("no valid external addresses for peer")
	}

	ai.Addrs = filteredAddrs

	ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
	defer cancel()

	if err := n.host.Connect(ctx, *ai); err != nil {
		return err
	}

	if len(ai.Addrs) > 0 {
		n.peerManager.AddPeer(ai.ID, ai.Addrs[0], network.DirOutbound)
	}
	n.stats.SetPeerCount(len(n.host.Network().Peers()))
	n.logger.Info(fmt.Sprintf("Connected to peer peer=%s addr=%s", ai.ID.String()[:12]+"...", maddr))
	return nil
}

func (n *ViriNetwork) FullMultiaddr() string {
	if n.host == nil {
		return ""
	}
	pid := n.PeerID()
	addrs := n.host.Addrs()
	if len(addrs) == 0 || pid == "" {
		return ""
	}

	for _, addr := range addrs {
		if strings.Contains(addr.String(), "0.0.0.0") || strings.Contains(addr.String(), "127.0.0.1") {
			continue
		}
		return fmt.Sprintf("%s/p2p/%s", addr.String(), pid.String())
	}

	return fmt.Sprintf("%s/p2p/%s", addrs[0].String(), pid.String())
}

func (nc *NetworkConfig) listenPort() int {
	parts := strings.Split(nc.ListenAddr, "/")
	for i, m := range parts {
		if m == "tcp" && i+1 < len(parts) {
			if port, err := strconv.Atoi(parts[i+1]); err == nil {
				return port
			}
		}
	}
	return 0
}

func (n *ViriNetwork) PeerManager() *PeerManager {
	return n.peerManager
}

func (n *ViriNetwork) ConnManager() *ConnManager {
	return n.connManager
}

func (n *ViriNetwork) RateLimiter() *RateLimiter {
	return n.rateLimiter
}

func (n *ViriNetwork) SecurityLimiter() *security.RateLimiter {
	return n.securityLimiter
}

func (n *ViriNetwork) DoSProtector() *security.DoSProtector {
	return n.dosProtector
}

func (n *ViriNetwork) GetDoSProtector() *security.DoSProtector {
	return n.dosProtector
}

func (n *ViriNetwork) SetSecurityLimiter(sl *security.RateLimiter) {
	n.securityLimiter = sl
}

func (n *ViriNetwork) SetDoSProtector(dos *security.DoSProtector) {
	n.dosProtector = dos
}

func (n *ViriNetwork) CheckDoSConnection(ip string) bool {
	if n.dosProtector != nil {
		return n.dosProtector.AllowConnection(ip)
	}
	return true
}

func (n *ViriNetwork) ReportDoSConnection(peerID peer.ID, ip string) {
	if n.dosProtector != nil {
		n.dosProtector.RecordConnection(peerID, ip)
	}
}

func (n *ViriNetwork) CleanupSecurity() {
	if n.securityLimiter != nil {
	}
	if n.dosProtector != nil {
		n.dosProtector.Cleanup()
	}
}

func (n *ViriNetwork) IsUnderAttack() bool {
	if n.dosProtector != nil {
		return n.dosProtector.IsUnderAttack()
	}
	return false
}

func (n *ViriNetwork) SecurityMemoryUsed() int64 {
	if n.dosProtector != nil {
		return n.dosProtector.MemoryUsed()
	}
	return 0
}

func (n *ViriNetwork) Propagator() *Propagator {
	return n.propagator
}

func (n *ViriNetwork) Stats() *NetworkStats {
	return n.stats
}

func (n *ViriNetwork) SetMessageHandler(handler MessageHandler) {
	n.messageHandler = handler
}

func (n *ViriNetwork) OnValidatorDiscovered(fn func(pubKey []byte, addr []byte, validatorPubKey []byte)) {
	n.onValidatorDiscovered = fn
}

func (n *ViriNetwork) Host() host.Host {
	return n.host
}

func (n *ViriNetwork) Drain(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.running {
		return nil
	}

	n.logger.Info("Starting graceful P2P connection drain...")

	// Close all tracked connections
	n.connManager.Close()

	// Wait for context or give time for in-flight operations to complete
	select {
	case <-ctx.Done():
		n.logger.Warn("P2P drain timed out")
		return ctx.Err()
	case <-time.After(500 * time.Millisecond):
	}

	n.logger.Info("P2P connections drained")
	return nil
}

func (n *ViriNetwork) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.running {
		return nil
	}

	n.running = false
	n.cancel()
	n.peerManager.Close()
	n.connManager.Stop()

	if n.host != nil {
		return n.host.Close()
	}

	return nil
}

func (n *ViriNetwork) handleBlockSync(stream network.Stream) {
	defer stream.Close()

	remotePeer := stream.Conn().RemotePeer()

	if n.rateLimiter.IsBlocked(remotePeer) {
		n.stats.RecordRejected()
		stream.Reset()
		return
	}

	if n.securityLimiter.IsBlocked(remotePeer) {
		n.stats.RecordRejected()
		stream.Reset()
		return
	}

	n.peerManager.UpdateScore(remotePeer, 1)

	buf := make([]byte, MaxMessageSize)
	bytesRead, err := stream.Read(buf)
	if err != nil {
		return
	}

	allowed, reason := n.securityLimiter.AllowPeer(remotePeer, bytesRead, security.MsgTypeGeneral, buf[:bytesRead])
	if !allowed {
		n.logger.Debug(fmt.Sprintf("Security rate limit: peer=%s reason=%s", remotePeer.String(), reason))
		n.stats.RecordRejected()
		return
	}

	if !n.rateLimiter.Allow(remotePeer, bytesRead) {
		n.stats.RecordRejected()
		return
	}

	n.stats.RecordMessageIn(bytesRead)

	msg, err := DecodeMessage(buf[:bytesRead])
	if err != nil {
		n.peerManager.UpdateScore(remotePeer, -5)
		n.stats.RecordDropped()
		n.securityLimiter.ReportInvalidMsg(remotePeer)
		return
	}

	result, err := n.validator.Validate(msg)
	if err != nil || result == ValidationReject {
		n.stats.RecordRejected()
		n.peerManager.UpdateScore(remotePeer, -5)
		n.securityLimiter.ReportInvalidMsg(remotePeer)
		return
	}

	n.securityLimiter.ReportValidMsg(remotePeer)

	if msg.Type == MsgBlock {
		n.stats.RecordBlockIn()
	}

	switch msg.Type {
	case MsgGetBlocks:
		n.handleGetBlocksRequest(stream, msg, remotePeer)
	case MsgGetHeaders:
		n.handleGetHeadersRequest(stream, msg, remotePeer)
	case MsgBlock:
		if n.messageHandler != nil {
			n.messageHandler.OnBlock(msg, remotePeer)
		}
	case MsgAnnounce:
		if n.messageHandler != nil {
			n.messageHandler.OnAnnounce(msg, remotePeer)
		}
	case MsgSync:
		n.handleSyncRequest(stream, msg, remotePeer)
	case MsgBlockRequest:
		n.handleBlockRequest(stream, msg, remotePeer)
	case MsgBlockResponse:
		if n.messageHandler != nil {
			n.messageHandler.OnBlock(msg, remotePeer)
		}
	}
}

func (n *ViriNetwork) handleGetBlocksRequest(stream network.Stream, msg *Message, remotePeer peer.ID) {
	if n.blockchain == nil {
		return
	}

	var startHeight uint64 = 0
	var count uint64 = 10

	if len(msg.Payload) >= 16 {
		startHeight = binary.BigEndian.Uint64(msg.Payload[:8])
		count = binary.BigEndian.Uint64(msg.Payload[8:16])
	}

	if count > 100 {
		count = 100
	}

	chainHeight := n.blockchain.Height()
	if startHeight >= chainHeight {
		emptyResp := NewMessage(MsgBlock, []byte{})
		respData, _ := emptyResp.Encode()
		stream.Write(respData)
		return
	}

	if startHeight+count > chainHeight {
		count = chainHeight - startHeight
	}

	for i := uint64(0); i < count; i++ {
		block, err := n.blockchain.GetBlock(startHeight + i)
		if err != nil {
			continue
		}
		data, err := ledger.SerializeBlock(block)
		if err != nil {
			continue
		}
		resp := NewMessage(MsgBlock, data)
		respData, _ := resp.Encode()
		if _, err := stream.Write(respData); err != nil {
			return
		}
		n.stats.RecordMessageOut(len(respData))
		n.stats.RecordBlockOut()
	}
}

func (n *ViriNetwork) handleGetHeadersRequest(stream network.Stream, msg *Message, remotePeer peer.ID) {
	if n.blockchain == nil {
		return
	}

	var startHeight uint64 = 0
	var count uint64 = 10

	if len(msg.Payload) >= 16 {
		startHeight = binary.BigEndian.Uint64(msg.Payload[:8])
		count = binary.BigEndian.Uint64(msg.Payload[8:16])
	}

	if count > 100 {
		count = 100
	}

	chainHeight := n.blockchain.Height()
	if startHeight >= chainHeight {
		emptyResp := NewMessage(MsgBlockHeader, []byte{})
		respData, _ := emptyResp.Encode()
		stream.Write(respData)
		return
	}

	if startHeight+count > chainHeight {
		count = chainHeight - startHeight
	}

	for i := uint64(0); i < count; i++ {
		block, err := n.blockchain.GetBlock(startHeight + i)
		if err != nil {
			continue
		}
		headerData, err := ledger.SerializeHeader(block.Header)
		if err != nil {
			continue
		}
		resp := NewMessage(MsgBlockHeader, headerData)
		respData, _ := resp.Encode()
		if _, err := stream.Write(respData); err != nil {
			return
		}
		n.stats.RecordMessageOut(len(respData))
	}
}

func (n *ViriNetwork) handleTxSync(stream network.Stream) {
	defer stream.Close()

	remotePeer := stream.Conn().RemotePeer()

	if n.rateLimiter.IsBlocked(remotePeer) {
		n.stats.RecordRejected()
		stream.Reset()
		return
	}

	if n.securityLimiter.IsBlocked(remotePeer) {
		n.stats.RecordRejected()
		stream.Reset()
		return
	}

	n.peerManager.UpdateScore(remotePeer, 1)

	buf := make([]byte, MaxMessageSize)
	bytesRead, err := stream.Read(buf)
	if err != nil {
		return
	}

	allowed, reason := n.securityLimiter.AllowPeer(remotePeer, bytesRead, security.MsgTypeGeneral, buf[:bytesRead])
	if !allowed {
		n.logger.Debug(fmt.Sprintf("Security rate limit: peer=%s reason=%s", remotePeer.String(), reason))
		n.stats.RecordRejected()
		return
	}

	if !n.rateLimiter.Allow(remotePeer, bytesRead) {
		n.stats.RecordRejected()
		return
	}

	n.stats.RecordMessageIn(bytesRead)

	msg, err := DecodeMessage(buf[:bytesRead])
	if err != nil {
		n.peerManager.UpdateScore(remotePeer, -5)
		n.stats.RecordDropped()
		n.securityLimiter.ReportInvalidMsg(remotePeer)
		return
	}

	result, err := n.validator.Validate(msg)
	if err != nil || result == ValidationReject {
		n.stats.RecordRejected()
		n.peerManager.UpdateScore(remotePeer, -5)
		n.securityLimiter.ReportInvalidMsg(remotePeer)
		return
	}

	n.securityLimiter.ReportValidMsg(remotePeer)

	n.stats.RecordTxIn()

	if n.messageHandler != nil {
		switch msg.Type {
		case MsgTransaction:
			n.messageHandler.OnTransaction(msg, remotePeer)
		}
	}
}

func (n *ViriNetwork) handlePeerSync(stream network.Stream) {
	defer stream.Close()

	remotePeer := stream.Conn().RemotePeer()
	remoteAddr := stream.Conn().RemoteMultiaddr()

	if n.rateLimiter.IsBlocked(remotePeer) {
		n.stats.RecordRejected()
		stream.Reset()
		return
	}

	if n.securityLimiter.IsBlocked(remotePeer) {
		n.stats.RecordRejected()
		stream.Reset()
		return
	}

	n.peerManager.AddPeer(remotePeer, remoteAddr, stream.Conn().Stat().Direction)
	n.stats.SetPeerCount(len(n.host.Network().Peers()))

	buf := make([]byte, MaxMessageSize)
	bytesRead, err := stream.Read(buf)
	if err != nil {
		return
	}

	allowed, reason := n.securityLimiter.AllowPeer(remotePeer, bytesRead, security.MsgTypeGeneral, buf[:bytesRead])
	if !allowed {
		n.logger.Debug(fmt.Sprintf("Security rate limit: peer=%s reason=%s", remotePeer.String(), reason))
		n.stats.RecordRejected()
		return
	}

	if !n.rateLimiter.Allow(remotePeer, bytesRead) {
		n.stats.RecordRejected()
		return
	}

	n.stats.RecordMessageIn(bytesRead)

	msg, err := DecodeMessage(buf[:bytesRead])
	if err != nil {
		n.peerManager.UpdateScore(remotePeer, -5)
		n.stats.RecordDropped()
		n.securityLimiter.ReportInvalidMsg(remotePeer)
		return
	}

	result, err := n.validator.Validate(msg)
	if err != nil || result == ValidationReject {
		n.stats.RecordRejected()
		n.peerManager.UpdateScore(remotePeer, -5)
		n.securityLimiter.ReportInvalidMsg(remotePeer)
		return
	}

	n.securityLimiter.ReportValidMsg(remotePeer)

	switch msg.Type {
	case MsgGetPeers:
		peers := n.peerManager.GetConnectedPeers()
		peerAddrs := make([]string, 0, len(peers))
		for _, p := range peers {
			if p.ID != remotePeer {
				peerAddrs = append(peerAddrs, p.Address.String())
			}
		}
		payload, _ := encodePeerList(peerAddrs)
		resp := NewMessage(MsgPeers, payload)
		respData, _ := resp.Encode()
		stream.Write(respData)
		n.stats.RecordMessageOut(len(respData))
	case MsgPeers:
		peerAddrs, err := decodePeerList(msg.Payload)
		if err == nil {
			for _, addr := range peerAddrs {
				ai, err := peer.AddrInfoFromString(addr)
				if err != nil {
					continue
				}
				if ai.ID == n.host.ID() {
					continue
				}
				n.peerManager.AddPeer(ai.ID, ai.Addrs[0], network.DirOutbound)
			}
		}
	}
}

func (n *ViriNetwork) handleSyncRequest(stream network.Stream, msg *Message, remotePeer peer.ID) {
	if n.blockchain == nil {
		return
	}

	height := n.blockchain.Height()
	tipHash := n.blockchain.TipHash()

	syncData := make([]byte, 16+len(tipHash))
	binary.BigEndian.PutUint64(syncData[:8], height)
	copy(syncData[8:8+len(tipHash)], tipHash)

	resp := NewMessage(MsgSync, syncData)
	respData, _ := resp.Encode()
	stream.Write(respData)
	n.stats.RecordMessageOut(len(respData))
}

func (n *ViriNetwork) handleBlockRequest(stream network.Stream, msg *Message, remotePeer peer.ID) {
	if n.blockchain == nil {
		return
	}

	if len(msg.Payload) < 16 {
		return
	}

	fromHeight := binary.BigEndian.Uint64(msg.Payload[:8])
	toHeight := binary.BigEndian.Uint64(msg.Payload[8:16])

	if toHeight < fromHeight {
		return
	}

	if toHeight-fromHeight > 100 {
		toHeight = fromHeight + 100
	}

	chainHeight := n.blockchain.Height()
	if fromHeight > chainHeight {
		return
	}

	if toHeight > chainHeight {
		toHeight = chainHeight
	}

	for h := fromHeight; h <= toHeight; h++ {
		block, err := n.blockchain.GetBlock(h)
		if err != nil {
			continue
		}

		blockData, err := ledger.SerializeBlock(block)
		if err != nil {
			continue
		}

		resp := NewMessage(MsgBlockResponse, blockData)
		respData, err := resp.Encode()
		if err != nil {
			continue
		}

		if _, err := stream.Write(respData); err != nil {
			return
		}
		n.stats.RecordMessageOut(len(respData))
		n.stats.RecordBlockOut()
	}
}

func (n *ViriNetwork) RequestBlocksFromPeer(ctx context.Context, peerID peer.ID, fromHeight, toHeight uint64) error {
	payload := make([]byte, 16)
	binary.BigEndian.PutUint64(payload[:8], fromHeight)
	binary.BigEndian.PutUint64(payload[8:16], toHeight)

	msg := NewMessage(MsgBlockRequest, payload)
	return n.SendToPeer(ctx, peerID, msg)
}

func (n *ViriNetwork) RequestBlocks(fromHeight, toHeight uint64) error {
	peers := n.peerManager.GetConnectedPeers()
	if len(peers) == 0 {
		return fmt.Errorf("no connected peers")
	}

	payload := make([]byte, 16)
	binary.BigEndian.PutUint64(payload[:8], fromHeight)
	binary.BigEndian.PutUint64(payload[8:16], toHeight)

	msg := NewMessage(MsgBlockRequest, payload)

	for _, p := range peers {
		if p.ID == n.host.ID() {
			continue
		}
		go func(pid peer.ID) {
			ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
			defer cancel()
			if err := n.SendToPeer(ctx, pid, msg); err != nil {
				n.logger.WithField("peer", pid.String()).WithField("error", err.Error()).Warn("RequestBlocks send failed")
			}
		}(p.ID)
	}

	return nil
}

func (n *ViriNetwork) startPeerDiscoveryLoop() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		// Initial connection attempt to bootstraps is already done in Start(),
		// but we retry them here periodically if we need more peers.

		for {
			select {
			case <-ticker.C:
				// 1. Retry bootstraps if we are low on peers
				if n.peerManager.NeedsPeers() && len(n.config.Bootstraps) > 0 {
					for _, addrStr := range n.config.Bootstraps {
						addr, err := multiaddr.NewMultiaddr(addrStr)
						if err != nil {
							continue
						}
						peerinfo, err := peer.AddrInfoFromP2pAddr(addr)
						if err != nil {
							continue
						}
						if n.host.Network().Connectedness(peerinfo.ID) == network.Connected {
							continue
						}

						ctx, cancel := context.WithTimeout(n.ctx, 5*time.Second)
						if err := n.host.Connect(ctx, *peerinfo); err != nil {
							n.logger.Debug(fmt.Sprintf("Failed to reconnect to bootstrap peer %s: %v", addrStr, err))
						} else {
							n.logger.Info(fmt.Sprintf("Reconnected to bootstrap peer %s", addrStr))
						}
						cancel()
					}
				}

				// 2. DHT discovery
				if n.peerManager.NeedsPeers() && n.dht != nil && n.config.Rendezvous != "" {
					routingDiscovery := routing.NewRoutingDiscovery(n.dht)
					peers, err := routingDiscovery.FindPeers(n.ctx, n.config.Rendezvous)
					if err != nil {
						n.logger.Debug(fmt.Sprintf("Rendezvous discovery error: %v", err))
					} else {
						for pi := range peers {
							if pi.ID == n.host.ID() || len(pi.Addrs) == 0 {
								continue
							}
							ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
							if err := n.host.Connect(ctx, pi); err == nil {
								n.logger.Info(fmt.Sprintf("DHT reconnected peer=%s", pi.ID.String()[:12]+"..."))
							}
							cancel()
						}
					}
				}

				if len(n.peerManager.GetConnectedPeers()) > 0 {
					n.BroadcastGetPeers()
				}

				n.peerManager.UpdateScore(n.host.ID(), 0)
			case <-n.ctx.Done():
				return
			}
		}
	}()
}

func (n *ViriNetwork) startHandshakeLoop() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				peers := n.peerManager.GetConnectedPeers()
				for _, p := range peers {
					if p.Version != "" {
						continue
					}
					go func(pid peer.ID) {
						ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
						defer cancel()
						result, err := n.SendHandshake(ctx, pid)
						if err != nil {
							n.logger.Debug(fmt.Sprintf("Handshake failed peer=%s error=%v", pid.String()[:12]+"...", err))
							return
						}
						n.logger.Info(fmt.Sprintf("Handshake complete peer=%s agent=%s height=%d latency=%v",
							result.PeerID[:12]+"...", result.PeerAgent, result.PeerHeight, result.Latency))
					}(p.ID)
				}
			case <-n.ctx.Done():
				return
			}
		}
	}()
}

func (n *ViriNetwork) handlePing(stream network.Stream) {
	defer stream.Close()

	remotePeer := stream.Conn().RemotePeer()

	buf := make([]byte, MaxMessageSize)
	bytesRead, err := stream.Read(buf)
	if err != nil {
		return
	}

	if n.rateLimiter.IsBlocked(remotePeer) {
		n.stats.RecordRejected()
		return
	}

	if n.securityLimiter.IsBlocked(remotePeer) {
		n.stats.RecordRejected()
		return
	}

	allowed, reason := n.securityLimiter.AllowPeer(remotePeer, bytesRead, security.MsgTypeGeneral, buf[:bytesRead])
	if !allowed {
		n.logger.Debug(fmt.Sprintf("Security rate limit: peer=%s reason=%s", remotePeer.String(), reason))
		n.stats.RecordRejected()
		return
	}

	if !n.rateLimiter.Allow(remotePeer, bytesRead) {
		n.stats.RecordRejected()
		return
	}

	n.stats.RecordMessageIn(bytesRead)

	msg, err := DecodeMessage(buf[:bytesRead])
	if err != nil {
		stream.Write([]byte("pong:invalid"))
		return
	}

	switch msg.Type {
	case MsgPing:
		nonce := hex.EncodeToString(msg.Payload[:8])
		resp := NewMessage(MsgPong, []byte(nonce))
		respData, _ := resp.Encode()
		stream.Write(respData)
		n.stats.RecordMessageOut(len(respData))
	case MsgPong:
		n.peerManager.UpdateScore(remotePeer, 1)
		n.securityLimiter.ReportValidMsg(remotePeer)
	default:
		stream.Write([]byte("pong:unknown"))
	}
}

func GenerateNodeKey() (libp2pcrypto.PrivKey, error) {
	privKey, _, err := libp2pcrypto.GenerateSecp256k1Key(rand.Reader)
	return privKey, err
}

func KeyToHex(privKey libp2pcrypto.PrivKey) string {
	bytes, _ := privKey.Raw()
	return hex.EncodeToString(bytes)
}

func MultiAddrString(host string, port string, peerID string) string {
	return fmt.Sprintf("/ip4/%s/tcp/%s/p2p/%s", host, port, peerID)
}

func (n *ViriNetwork) handleHandshake(stream network.Stream) {
	defer stream.Close()

	remotePeer := stream.Conn().RemotePeer()

	if n.rateLimiter.IsBlocked(remotePeer) {
		n.stats.RecordRejected()
		stream.Reset()
		return
	}

	if n.securityLimiter.IsBlocked(remotePeer) {
		n.stats.RecordRejected()
		stream.Reset()
		return
	}

	buf := make([]byte, MaxMessageSize)
	bytesRead, err := stream.Read(buf)
	if err != nil {
		return
	}

	allowed, reason := n.securityLimiter.AllowPeer(remotePeer, bytesRead, security.MsgTypeGeneral, buf[:bytesRead])
	if !allowed {
		n.logger.Debug(fmt.Sprintf("Security rate limit: peer=%s reason=%s", remotePeer.String(), reason))
		n.stats.RecordRejected()
		return
	}

	if !n.rateLimiter.Allow(remotePeer, bytesRead) {
		n.stats.RecordRejected()
		return
	}

	remoteHS, err := DecodeHandshakeMessage(buf[:bytesRead])
	if err != nil {
		n.peerManager.UpdateScore(remotePeer, -10)
		n.stats.RecordDropped()
		n.securityLimiter.ReportInvalidMsg(remotePeer)
		return
	}

	if err := remoteHS.Validate(n.config.ChainID); err != nil {
		n.logger.Info(fmt.Sprintf("Handshake rejected peer=%s error=%v", remotePeer.String(), err))
		n.peerManager.UpdateScore(remotePeer, -10)
		n.stats.RecordRejected()
		n.securityLimiter.ReportInvalidMsg(remotePeer)
		rejectHS := &HandshakeMessage{Status: 1}
		rejectData, _ := rejectHS.Encode()
		stream.Write(rejectData)
		return
	}

	n.securityLimiter.ReportValidMsg(remotePeer)

	localHeight := uint64(0)
	if n.blockchain != nil {
		localHeight = n.blockchain.Height()
	}

	localHS := NewHandshakeMessage(n.config.ChainID, localHeight, 0, n.authenticator.PublicKey())
	localHS.ValidatorAddr = n.validatorAddress
	localHS.ValidatorPubKey = n.validatorPubKey
	localData, _ := localHS.Encode()
	stream.Write(localData)
	n.stats.RecordMessageOut(len(localData))

	if peerInfo, exists := n.peerManager.GetPeer(remotePeer); exists {
		peerInfo.Version = fmt.Sprintf("%d", remoteHS.Version)
		peerInfo.Height = remoteHS.Height
		peerInfo.Agent = remoteHS.Agent
		peerInfo.PubKey = remoteHS.PubKey
	} else {
		n.peerManager.AddPeerWithPubKey(remotePeer, stream.Conn().RemoteMultiaddr(), stream.Conn().Stat().Direction, remoteHS.PubKey)
	}

	n.stats.RecordMessageIn(bytesRead)
	n.peerManager.UpdateScore(remotePeer, 5)
	n.peerManager.UpdateHeight(remotePeer, remoteHS.Height)

	if n.onValidatorDiscovered != nil && len(remoteHS.PubKey) > 0 {
		n.onValidatorDiscovered(remoteHS.PubKey, remoteHS.ValidatorAddr, remoteHS.ValidatorPubKey)
	}
}

func (n *ViriNetwork) SendHandshake(ctx context.Context, peerID peer.ID) (*HandshakeResult, error) {
	start := time.Now()

	stream, err := n.host.NewStream(ctx, peerID, protocol.ID("/viri/handshake/1.0.0"))
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	localHeight := uint64(0)
	if n.blockchain != nil {
		localHeight = n.blockchain.Height()
	}

	hs := NewHandshakeMessage(n.config.ChainID, localHeight, 0, n.authenticator.PublicKey())
	hs.ValidatorAddr = n.validatorAddress
	hs.ValidatorPubKey = n.validatorPubKey
	data, err := hs.Encode()
	if err != nil {
		return nil, err
	}

	if _, err := stream.Write(data); err != nil {
		return nil, err
	}
	n.stats.RecordMessageOut(len(data))

	buf := make([]byte, MaxMessageSize)
	bytesRead, err := stream.Read(buf)
	if err != nil {
		return nil, err
	}

	n.stats.RecordMessageIn(bytesRead)

	remoteHS, err := DecodeHandshakeMessage(buf[:bytesRead])
	if err != nil {
		return nil, err
	}

	if err := remoteHS.Validate(n.config.ChainID); err != nil {
		return nil, err
	}

	latency := time.Since(start)

	if peerInfo, exists := n.peerManager.GetPeer(peerID); exists {
		peerInfo.Version = fmt.Sprintf("%d", remoteHS.Version)
		peerInfo.Height = remoteHS.Height
		peerInfo.Agent = remoteHS.Agent
		peerInfo.PubKey = remoteHS.PubKey
	} else {
		n.peerManager.AddPeerWithPubKey(peerID, nil, network.DirOutbound, remoteHS.PubKey)
	}
	n.peerManager.UpdateHeight(peerID, remoteHS.Height)

	if n.onValidatorDiscovered != nil && len(remoteHS.PubKey) > 0 {
		n.onValidatorDiscovered(remoteHS.PubKey, remoteHS.ValidatorAddr, remoteHS.ValidatorPubKey)
	}

	return &HandshakeResult{
		PeerID:          peerID.String(),
		PeerAgent:       remoteHS.Agent,
		PeerHeight:      remoteHS.Height,
		PeerChainID:     remoteHS.ChainID,
		Latency:         latency,
		Established:     time.Now(),
		PubKey:          remoteHS.PubKey,
		ValidatorAddr:   remoteHS.ValidatorAddr,
		ValidatorPubKey: remoteHS.ValidatorPubKey,
	}, nil
}

func (n *ViriNetwork) BroadcastGetPeers() {
	peers := n.peerManager.GetConnectedPeers()
	for _, p := range peers {
		if p.ID == n.host.ID() {
			continue
		}
		req := NewMessage(MsgGetPeers, []byte{})
		go func(pid peer.ID) {
			ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
			defer cancel()
			n.SendToPeer(ctx, pid, req)
		}(p.ID)
	}
}

func encodePeerList(addrs []string) ([]byte, error) {
	return json.Marshal(addrs)
}

func decodePeerList(data []byte) ([]string, error) {
	var addrs []string
	if err := json.Unmarshal(data, &addrs); err != nil {
		return nil, err
	}
	return addrs, nil
}
