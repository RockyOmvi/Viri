package p2p

import (
	"testing"
)

func TestNewMessage(t *testing.T) {
	payload := []byte("test-payload")
	msg := NewMessage(MsgPing, payload)

	if msg.Type != MsgPing {
		t.Errorf("Expected type MsgPing, got %v", msg.Type)
	}

	if string(msg.Payload) != "test-payload" {
		t.Errorf("Expected payload 'test-payload', got %s", msg.Payload)
	}
}

func TestMessageEncodeDecode(t *testing.T) {
	tests := []struct {
		name    string
		msgType MessageType
		payload []byte
	}{
		{
			name:    "ping message",
			msgType: MsgPing,
			payload: []byte("ping-data"),
		},
		{
			name:    "empty payload",
			msgType: MsgPong,
			payload: []byte{},
		},
		{
			name:    "large payload",
			msgType: MsgBlock,
			payload: make([]byte, 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewMessage(tt.msgType, tt.payload)

			encoded, err := msg.Encode()
			if err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}

			if len(encoded) == 0 {
				t.Fatal("Encoded data is empty")
			}

			decoded, err := DecodeMessage(encoded)
			if err != nil {
				t.Fatalf("Failed to decode: %v", err)
			}

			if decoded.Type != tt.msgType {
				t.Errorf("Type mismatch: expected %v, got %v", tt.msgType, decoded.Type)
			}

			if string(decoded.Payload) != string(tt.payload) {
				t.Errorf("Payload mismatch")
			}
		})
	}
}

func TestDecodeMessageInvalid(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "too short",
			data: []byte{0x00, 0x01},
		},
		{
			name: "payload length mismatch",
			data: []byte{0x10, 0x00, 0x00, 0x00, 0x05, 0x01, 0x02},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeMessage(tt.data)
			if err == nil {
				t.Error("Expected error for invalid message")
			}
		})
	}
}

func TestMessageSize(t *testing.T) {
	payload := []byte("hello")
	msg := NewMessage(MsgPing, payload)

	expected := 5 + len(payload)
	if msg.Size() != expected {
		t.Errorf("Expected size %d, got %d", expected, msg.Size())
	}
}

func TestMessageTypeValues(t *testing.T) {
	if MsgPing != 0x00 {
		t.Errorf("MsgPing should be 0x00, got 0x%02x", MsgPing)
	}
	if MsgBlock != 0x10 {
		t.Errorf("MsgBlock should be 0x10, got 0x%02x", MsgBlock)
	}
	if MsgTransaction != 0x12 {
		t.Errorf("MsgTransaction should be 0x12, got 0x%02x", MsgTransaction)
	}
}
