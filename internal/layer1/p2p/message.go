package p2p

import (
	"encoding/binary"
	"errors"
)

type MessageType uint8

const (
	MsgPing       MessageType = 0x00
	MsgPong       MessageType = 0x01
	MsgBlock      MessageType = 0x10
	MsgBlockHeader MessageType = 0x11
	MsgTransaction MessageType = 0x12
	MsgGetBlocks   MessageType = 0x20
	MsgGetHeaders  MessageType = 0x21
	MsgGetPeers    MessageType = 0x22
	MsgPeers       MessageType = 0x23
	MsgAnnounce    MessageType = 0x30
	MsgSync        MessageType = 0x40
	MsgBlockRequest  MessageType = 0x41
	MsgBlockResponse MessageType = 0x42
	MsgProposal    MessageType = 0x50
	MsgVote        MessageType = 0x51
	MsgQC          MessageType = 0x52
	MsgTimeout     MessageType = 0x53
	MsgNewView     MessageType = 0x54
)

var (
	ErrInvalidMessage  = errors.New("invalid message")
	ErrMessageTooLarge = errors.New("message exceeds maximum size")
	ErrUnknownType     = errors.New("unknown message type")
)

const MaxMessageSize = 10 * 1024 * 1024 // 10MB

type Message struct {
	Type    MessageType
	Payload []byte
}

func NewMessage(msgType MessageType, payload []byte) *Message {
	return &Message{
		Type:    msgType,
		Payload: payload,
	}
}

func (m *Message) Encode() ([]byte, error) {
	header := make([]byte, 5)
	header[0] = byte(m.Type)
	binary.BigEndian.PutUint32(header[1:], uint32(len(m.Payload)))

	data := make([]byte, 5+len(m.Payload))
	copy(data[:5], header)
	copy(data[5:], m.Payload)

	return data, nil
}

func DecodeMessage(data []byte) (*Message, error) {
	if len(data) < 5 {
		return nil, ErrInvalidMessage
	}

	msgType := MessageType(data[0])
	payloadLen := binary.BigEndian.Uint32(data[1:5])

	if payloadLen > MaxMessageSize {
		return nil, ErrMessageTooLarge
	}

	if uint32(len(data)-5) != payloadLen {
		return nil, ErrInvalidMessage
	}

	return &Message{
		Type:    msgType,
		Payload: data[5:],
	}, nil
}

func (m *Message) Size() int {
	return 5 + len(m.Payload)
}
