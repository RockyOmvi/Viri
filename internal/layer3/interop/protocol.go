package interop

import (
	"fmt"
	"sync"
	"time"
)

type PacketType uint8

const (
	PacketTypeTransfer PacketType = iota
	PacketTypeIBC
	PacketTypeQuery
	PacketTypeAck
)

type IBCPacket struct {
	Sequence      uint64
	SourcePort    string
	DestPort      string
	SourceChain   string
	DestChain     string
	Type          PacketType
	Data          []byte
	Timeout       uint64
	Status        PacketStatus
	CreatedAt     time.Time
	Proof         []byte
}

type PacketStatus uint8

const (
	PacketStatusPending PacketStatus = iota
	PacketStatusSent
	PacketStatusReceived
	PacketStatusAcknowledged
	PacketStatusTimedOut
	PacketStatusFailed
)

type Channel struct {
	ID          string
	PortA       string
	PortB       string
	ChainA      string
	ChainB      string
	Version     string
	IsActive    bool
	NextSequence uint64
}

type InteropProtocol struct {
	mu       sync.RWMutex
	channels map[string]*Channel
	packets  map[string]*IBCPacket
	handlers map[string]PacketHandler
}

type PacketHandler func(packet *IBCPacket) ([]byte, error)

func NewInteropProtocol() *InteropProtocol {
	return &InteropProtocol{
		channels: make(map[string]*Channel),
		packets:  make(map[string]*IBCPacket),
		handlers: make(map[string]PacketHandler),
	}
}

func (p *InteropProtocol) CreateChannel(portA, portB, chainA, chainB, version string) (*Channel, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := fmt.Sprintf("%s-%s-%s-%s", chainA, portA, chainB, portB)

	if _, exists := p.channels[id]; exists {
		return nil, fmt.Errorf("channel already exists")
	}

	channel := &Channel{
		ID:       id,
		PortA:    portA,
		PortB:    portB,
		ChainA:   chainA,
		ChainB:   chainB,
		Version:  version,
		IsActive: true,
	}

	p.channels[id] = channel
	return channel, nil
}

func (p *InteropProtocol) SendPacket(channelID string, packetType PacketType, data []byte, timeout uint64) (*IBCPacket, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	channel, exists := p.channels[channelID]
	if !exists {
		return nil, fmt.Errorf("channel not found")
	}

	if !channel.IsActive {
		return nil, fmt.Errorf("channel is not active")
	}

	packet := &IBCPacket{
		Sequence:    channel.NextSequence,
		SourcePort:  channel.PortA,
		DestPort:    channel.PortB,
		SourceChain: channel.ChainA,
		DestChain:   channel.ChainB,
		Type:        packetType,
		Data:        data,
		Timeout:     timeout,
		Status:      PacketStatusPending,
		CreatedAt:   time.Now(),
	}

	channel.NextSequence++

	key := fmt.Sprintf("%s-%d", channelID, packet.Sequence)
	p.packets[key] = packet

	return packet, nil
}

func (p *InteropProtocol) ReceivePacket(channelID string, sequence uint64, proof []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := fmt.Sprintf("%s-%d", channelID, sequence)
	packet, exists := p.packets[key]
	if !exists {
		return fmt.Errorf("packet not found")
	}

	if packet.Status != PacketStatusPending {
		return fmt.Errorf("packet not pending")
	}

	packet.Proof = proof
	packet.Status = PacketStatusReceived

	if handler, exists := p.handlers[packet.DestPort]; exists {
		_, err := handler(packet)
		if err != nil {
			packet.Status = PacketStatusFailed
			return err
		}
		packet.Status = PacketStatusAcknowledged
	}

	return nil
}

func (p *InteropProtocol) RegisterHandler(port string, handler PacketHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[port] = handler
}

func (p *InteropProtocol) GetChannel(id string) (*Channel, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	channel, exists := p.channels[id]
	return channel, exists
}

func (p *InteropProtocol) GetActiveChannels() []*Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var active []*Channel
	for _, ch := range p.channels {
		if ch.IsActive {
			active = append(active, ch)
		}
	}

	return active
}

func (p *InteropProtocol) CloseChannel(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	channel, exists := p.channels[id]
	if !exists {
		return fmt.Errorf("channel not found")
	}

	channel.IsActive = false
	return nil
}
