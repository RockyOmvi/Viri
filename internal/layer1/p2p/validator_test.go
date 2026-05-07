package p2p

import (
	"testing"
)

func TestValidatorAcceptValidBlock(t *testing.T) {
	mv := NewMessageValidator(1337)

	msg := NewMessage(MsgBlock, make([]byte, 1000))

	result, err := mv.Validate(msg)
	if err != nil {
		t.Fatalf("Validation error: %v", err)
	}

	if result != ValidationAccept {
		t.Errorf("Expected ValidationAccept, got %v", result)
	}
}

func TestValidatorRejectEmptyBlock(t *testing.T) {
	mv := NewMessageValidator(1337)

	msg := NewMessage(MsgBlock, []byte{})

	result, err := mv.Validate(msg)
	if err == nil {
		t.Error("Expected error for empty block")
	}

	if result != ValidationReject {
		t.Errorf("Expected ValidationReject, got %v", result)
	}
}

func TestValidatorRejectLargeBlock(t *testing.T) {
	mv := NewMessageValidator(1337)
	mv.maxBlockPayloadSize = 100

	msg := NewMessage(MsgBlock, make([]byte, 200))

	result, err := mv.Validate(msg)
	if err == nil {
		t.Error("Expected error for large block")
	}

	if result != ValidationReject {
		t.Errorf("Expected ValidationReject, got %v", result)
	}
}

func TestValidatorAcceptValidTx(t *testing.T) {
	mv := NewMessageValidator(1337)

	msg := NewMessage(MsgTransaction, make([]byte, 100))

	result, err := mv.Validate(msg)
	if err != nil {
		t.Fatalf("Validation error: %v", err)
	}

	if result != ValidationAccept {
		t.Errorf("Expected ValidationAccept, got %v", result)
	}
}

func TestValidatorRejectUnknownType(t *testing.T) {
	mv := NewMessageValidator(1337)

	msg := NewMessage(0xFF, []byte("data"))

	result, err := mv.Validate(msg)
	if err == nil {
		t.Error("Expected error for unknown message type")
	}

	if result != ValidationReject {
		t.Errorf("Expected ValidationReject, got %v", result)
	}
}

func TestValidatorPingPayload(t *testing.T) {
	mv := NewMessageValidator(1337)

	msg := NewMessage(MsgPing, make([]byte, 100))

	result, err := mv.Validate(msg)
	if err != nil {
		t.Fatalf("Validation error: %v", err)
	}

	if result != ValidationAccept {
		t.Errorf("Expected ValidationAccept, got %v", result)
	}
}

func TestValidatorRejectLargePing(t *testing.T) {
	mv := NewMessageValidator(1337)

	msg := NewMessage(MsgPing, make([]byte, 300))

	result, err := mv.Validate(msg)
	if err == nil {
		t.Error("Expected error for large ping")
	}

	if result != ValidationReject {
		t.Errorf("Expected ValidationReject, got %v", result)
	}
}

func TestValidatorHeaderTooSmall(t *testing.T) {
	mv := NewMessageValidator(1337)

	msg := NewMessage(MsgBlockHeader, make([]byte, 10))

	result, err := mv.Validate(msg)
	if err == nil {
		t.Error("Expected error for small header")
	}

	if result != ValidationReject {
		t.Errorf("Expected ValidationReject, got %v", result)
	}
}

func TestValidatorAcceptEmptyQuery(t *testing.T) {
	mv := NewMessageValidator(1337)

	msg := NewMessage(MsgGetBlocks, []byte{})

	result, err := mv.Validate(msg)
	if err != nil {
		t.Fatalf("Validation error: %v", err)
	}

	if result != ValidationAccept {
		t.Errorf("Expected ValidationAccept, got %v", result)
	}
}

func TestValidatorRejectLargeQuery(t *testing.T) {
	mv := NewMessageValidator(1337)

	msg := NewMessage(MsgGetBlocks, make([]byte, 2000))

	result, err := mv.Validate(msg)
	if err == nil {
		t.Error("Expected error for large query")
	}

	if result != ValidationReject {
		t.Errorf("Expected ValidationReject, got %v", result)
	}
}

func TestValidatorValidateHandshake(t *testing.T) {
	mv := NewMessageValidator(1337)

	hs := NewHandshakeMessage(1337, 100, 42, newTestPubKey())
	hs.Agent = "viri/0.1.0"

	data, _ := hs.Encode()

	decoded, result, err := mv.ValidateHandshake(data)
	if err != nil {
		t.Fatalf("Handshake validation error: %v", err)
	}

	if result != ValidationAccept {
		t.Errorf("Expected ValidationAccept, got %v", result)
	}

	if decoded.ChainID != 1337 {
		t.Errorf("Expected chainID 1337, got %d", decoded.ChainID)
	}
}

func TestValidatorRejectInvalidHandshake(t *testing.T) {
	mv := NewMessageValidator(1337)

	_, result, err := mv.ValidateHandshake([]byte{0x01, 0x02})
	if err == nil {
		t.Error("Expected error for invalid handshake")
	}

	if result != ValidationReject {
		t.Errorf("Expected ValidationReject, got %v", result)
	}
}

func TestHashFromPayload(t *testing.T) {
	hash := HashFromPayload([]byte("test-data"))
	if hash == "" {
		t.Error("Hash should not be empty")
	}

	emptyHash := HashFromPayload([]byte{})
	if emptyHash != "" {
		t.Error("Hash of empty payload should be empty string")
	}
}
