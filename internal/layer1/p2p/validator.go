package p2p

import (
	"encoding/hex"
	"fmt"
)

type ValidationResult int

const (
	ValidationAccept ValidationResult = iota
	ValidationReject
	ValidationIgnore
)

type MessageValidator struct {
	chainID              uint64
	maxBlockPayloadSize  int
	maxTxPayloadSize     int
	allowedMessageTypes  map[MessageType]bool
}

func NewMessageValidator(chainID uint64) *MessageValidator {
	return &MessageValidator{
		chainID:             chainID,
		maxBlockPayloadSize: 10 * 1024 * 1024,
		maxTxPayloadSize:    1 * 1024 * 1024,
		allowedMessageTypes: map[MessageType]bool{
			MsgPing:        true,
			MsgPong:        true,
			MsgBlock:       true,
			MsgBlockHeader: true,
			MsgTransaction: true,
			MsgGetBlocks:   true,
			MsgGetHeaders:  true,
			MsgGetPeers:    true,
			MsgPeers:       true,
			MsgAnnounce:    true,
			MsgSync:        true,
			MsgBlockRequest:  true,
			MsgBlockResponse: true,
			MsgProposal:    true,
			MsgVote:        true,
			MsgQC:          true,
			MsgTimeout:     true,
			MsgNewView:     true,
		},
	}
}

func (mv *MessageValidator) Validate(msg *Message) (ValidationResult, error) {
	if !mv.allowedMessageTypes[msg.Type] {
		return ValidationReject, fmt.Errorf("unknown message type: %d", msg.Type)
	}

	switch msg.Type {
	case MsgBlock:
		return mv.validateBlock(msg)
	case MsgTransaction:
		return mv.validateTransaction(msg)
	case MsgBlockHeader:
		return mv.validateHeader(msg)
	case MsgGetBlocks, MsgGetHeaders:
		return mv.validateQuery(msg)
	case MsgPing, MsgPong:
		return mv.validatePing(msg)
	default:
		return ValidationAccept, nil
	}
}

func (mv *MessageValidator) validateBlock(msg *Message) (ValidationResult, error) {
	if len(msg.Payload) == 0 {
		return ValidationReject, fmt.Errorf("empty block payload")
	}

	if len(msg.Payload) > mv.maxBlockPayloadSize {
		return ValidationReject, fmt.Errorf("block payload too large: %d bytes", len(msg.Payload))
	}

	return ValidationAccept, nil
}

func (mv *MessageValidator) validateTransaction(msg *Message) (ValidationResult, error) {
	if len(msg.Payload) == 0 {
		return ValidationReject, fmt.Errorf("empty transaction payload")
	}

	if len(msg.Payload) > mv.maxTxPayloadSize {
		return ValidationReject, fmt.Errorf("transaction payload too large: %d bytes", len(msg.Payload))
	}

	return ValidationAccept, nil
}

func (mv *MessageValidator) validateHeader(msg *Message) (ValidationResult, error) {
	if len(msg.Payload) < 64 {
		return ValidationReject, fmt.Errorf("header payload too small: %d bytes", len(msg.Payload))
	}

	return ValidationAccept, nil
}

func (mv *MessageValidator) validateQuery(msg *Message) (ValidationResult, error) {
	if len(msg.Payload) == 0 {
		return ValidationAccept, nil
	}

	if len(msg.Payload) > 1024 {
		return ValidationReject, fmt.Errorf("query payload too large: %d bytes", len(msg.Payload))
	}

	return ValidationAccept, nil
}

func (mv *MessageValidator) validatePing(msg *Message) (ValidationResult, error) {
	if len(msg.Payload) > 256 {
		return ValidationReject, fmt.Errorf("ping payload too large: %d bytes", len(msg.Payload))
	}

	return ValidationAccept, nil
}

func (mv *MessageValidator) ValidateHandshake(data []byte) (*HandshakeMessage, ValidationResult, error) {
	hs, err := DecodeHandshakeMessage(data)
	if err != nil {
		return nil, ValidationReject, fmt.Errorf("invalid handshake: %w", err)
	}

	if err := hs.Validate(mv.chainID); err != nil {
		return nil, ValidationReject, fmt.Errorf("handshake validation failed: %w", err)
	}

	return hs, ValidationAccept, nil
}

func HashFromPayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}

	hash := make([]byte, min(len(payload), 32))
	copy(hash, payload[:len(hash)])
	return hex.EncodeToString(hash)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
