package p2p

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

type HandshakeStatus uint8

const (
	HandshakePending HandshakeStatus = iota
	HandshakeComplete
	HandshakeFailed
)

const (
	ProtocolVersion uint32 = 1
	MagicNumber     uint32 = 0x56495249
)

type HandshakeMessage struct {
	Version         uint32
	Magic           uint32
	ChainID         uint64
	Timestamp       int64
	Nonce           uint64
	Agent           string
	Height          uint64
	Status          uint32
	PubKey          []byte
	ValidatorAddr   []byte
	ValidatorPubKey []byte
}

func NewHandshakeMessage(chainID uint64, height uint64, nonce uint64, pubKey *crypto.PublicKey) *HandshakeMessage {
	pubKeyBytes := pubKey.Compressed()
	return &HandshakeMessage{
		Version:   ProtocolVersion,
		Magic:     MagicNumber,
		ChainID:   chainID,
		Timestamp: time.Now().Unix(),
		Nonce:     nonce,
		Agent:     "viri/0.1.0",
		Height:    height,
		Status:    0,
		PubKey:    pubKeyBytes,
	}
}

func (h *HandshakeMessage) Encode() ([]byte, error) {
	buf := make([]byte, 52+len(h.Agent)+len(h.PubKey)+len(h.ValidatorAddr)+len(h.ValidatorPubKey))
	offset := 0

	binary.BigEndian.PutUint32(buf[offset:], h.Version)
	offset += 4
	binary.BigEndian.PutUint32(buf[offset:], h.Magic)
	offset += 4
	binary.BigEndian.PutUint64(buf[offset:], h.ChainID)
	offset += 8
	binary.BigEndian.PutUint64(buf[offset:], uint64(h.Timestamp))
	offset += 8
	binary.BigEndian.PutUint64(buf[offset:], h.Nonce)
	offset += 8
	binary.BigEndian.PutUint16(buf[offset:], uint16(len(h.Agent)))
	offset += 2
	copy(buf[offset:], h.Agent)
	offset += len(h.Agent)
	binary.BigEndian.PutUint64(buf[offset:], h.Height)
	offset += 8
	binary.BigEndian.PutUint32(buf[offset:], h.Status)
	offset += 4

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(h.PubKey)))
	offset += 2
	copy(buf[offset:], h.PubKey)
	offset += len(h.PubKey)

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(h.ValidatorAddr)))
	offset += 2
	copy(buf[offset:], h.ValidatorAddr)
	offset += len(h.ValidatorAddr)

	binary.BigEndian.PutUint16(buf[offset:], uint16(len(h.ValidatorPubKey)))
	offset += 2
	copy(buf[offset:], h.ValidatorPubKey)

	return buf, nil
}

func DecodeHandshakeMessage(data []byte) (*HandshakeMessage, error) {
	if len(data) < 39 {
		return nil, fmt.Errorf("handshake data too short: %d bytes", len(data))
	}

	offset := 0
	h := &HandshakeMessage{}

	h.Version = binary.BigEndian.Uint32(data[offset:])
	offset += 4
	h.Magic = binary.BigEndian.Uint32(data[offset:])
	offset += 4
	h.ChainID = binary.BigEndian.Uint64(data[offset:])
	offset += 8
	h.Timestamp = int64(binary.BigEndian.Uint64(data[offset:]))
	offset += 8
	h.Nonce = binary.BigEndian.Uint64(data[offset:])
	offset += 8

	if offset+2 > len(data) {
		return nil, fmt.Errorf("handshake data truncated at agent length")
	}

	agentLen := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2

	if offset+agentLen+12 > len(data) {
		return nil, fmt.Errorf("handshake data truncated at agent payload")
	}

	h.Agent = string(data[offset : offset+agentLen])
	offset += agentLen

	h.Height = binary.BigEndian.Uint64(data[offset:])
	offset += 8
	h.Status = binary.BigEndian.Uint32(data[offset:])
	offset += 4

	if offset+2 > len(data) {
		return nil, fmt.Errorf("handshake data truncated at public key length")
	}
	pubKeyLen := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2

	if offset+pubKeyLen > len(data) {
		return nil, fmt.Errorf("handshake data truncated at public key")
	}
	h.PubKey = make([]byte, pubKeyLen)
	copy(h.PubKey, data[offset:offset+pubKeyLen])
	offset += pubKeyLen

	if offset+2 <= len(data) {
		addrLen := int(binary.BigEndian.Uint16(data[offset:]))
		offset += 2
		if offset+addrLen <= len(data) && addrLen > 0 {
			h.ValidatorAddr = make([]byte, addrLen)
			copy(h.ValidatorAddr, data[offset:offset+addrLen])
			offset += addrLen
		}
	}

	if offset+2 <= len(data) {
		pubKeyLen := int(binary.BigEndian.Uint16(data[offset:]))
		offset += 2
		if offset+pubKeyLen <= len(data) && pubKeyLen > 0 {
			h.ValidatorPubKey = make([]byte, pubKeyLen)
			copy(h.ValidatorPubKey, data[offset:offset+pubKeyLen])
		}
	}

	return h, nil
}

func (h *HandshakeMessage) Validate(chainID uint64) error {
	if h.Magic != MagicNumber {
		return fmt.Errorf("invalid magic number: 0x%08x", h.Magic)
	}

	if h.Version != ProtocolVersion {
		return fmt.Errorf("unsupported protocol version: %d", h.Version)
	}

	if h.ChainID != chainID {
		return fmt.Errorf("chain ID mismatch: expected %d, got %d", chainID, h.ChainID)
	}

	if time.Now().Unix()-h.Timestamp > 60 {
		return fmt.Errorf("handshake timestamp too old: %d", h.Timestamp)
	}

	if len(h.Agent) == 0 || len(h.Agent) > 100 {
		return fmt.Errorf("invalid agent string length: %d", len(h.Agent))
	}

	return nil
}

type HandshakeResult struct {
	PeerID         string
	PeerAgent      string
	PeerHeight     uint64
	PeerChainID    uint64
	Latency        time.Duration
	Established    time.Time
	PubKey         []byte
	ValidatorAddr  []byte
	ValidatorPubKey []byte
}
