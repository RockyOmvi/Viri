package p2p

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"time"

	"github.com/viri-chain/viri/internal/layer1/crypto"
)

var (
	ErrInvalidSignature       = errors.New("invalid signature")
	ErrSignatureExpired       = errors.New("message signature expired")
	ErrChainIDMismatch        = errors.New("chain ID mismatch")
	ErrInvalidPublicKey       = errors.New("invalid public key")
	ErrInvalidTimestamp       = errors.New("invalid timestamp")
	ErrSignedMessageTooLarge  = errors.New("signed message exceeds maximum size")
)

const (
	MaxSignedMessageSize = 10*1024*1024 + 256
	DefaultMaxMessageAge = 5 * time.Minute
	CompressedPubKeyLen  = 33
)

type SignedMessage struct {
	MessageBytes []byte
	PublicKey    []byte
	Signature    []byte
	Timestamp    int64
	ChainID      uint64
}

func NewSignedMessage(msgBytes, pubKey, signature []byte, timestamp int64, chainID uint64) *SignedMessage {
	return &SignedMessage{
		MessageBytes: msgBytes,
		PublicKey:    pubKey,
		Signature:    signature,
		Timestamp:    timestamp,
		ChainID:      chainID,
	}
}

func (sm *SignedMessage) Encode() ([]byte, error) {
	if len(sm.PublicKey) == 0 {
		return nil, ErrInvalidPublicKey
	}
	if len(sm.Signature) == 0 {
		return nil, ErrInvalidSignature
	}

	pubKeyLen := len(sm.PublicKey)
	sigLen := len(sm.Signature)
	msgLen := len(sm.MessageBytes)

	totalLen := 8 + 8 + 2 + pubKeyLen + 2 + sigLen + 4 + msgLen
	if totalLen > MaxSignedMessageSize {
		return nil, ErrSignedMessageTooLarge
	}

	buf := make([]byte, totalLen)
	offset := 0

	binary.BigEndian.PutUint64(buf[offset:], uint64(sm.Timestamp))
	offset += 8

	binary.BigEndian.PutUint64(buf[offset:], sm.ChainID)
	offset += 8

	binary.BigEndian.PutUint16(buf[offset:], uint16(pubKeyLen))
	offset += 2
	copy(buf[offset:], sm.PublicKey)
	offset += pubKeyLen

	binary.BigEndian.PutUint16(buf[offset:], uint16(sigLen))
	offset += 2
	copy(buf[offset:], sm.Signature)
	offset += sigLen

	binary.BigEndian.PutUint32(buf[offset:], uint32(msgLen))
	offset += 4
	copy(buf[offset:], sm.MessageBytes)

	return buf, nil
}

func DecodeSignedMessage(data []byte) (*SignedMessage, error) {
	if len(data) < 24 {
		return nil, errors.New("signed message data too short")
	}

	offset := 0
	sm := &SignedMessage{}

	sm.Timestamp = int64(binary.BigEndian.Uint64(data[offset:]))
	offset += 8

	sm.ChainID = binary.BigEndian.Uint64(data[offset:])
	offset += 8

	if offset+2 > len(data) {
		return nil, errors.New("signed message truncated at public key length")
	}
	pubKeyLen := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2

	if offset+pubKeyLen+2 > len(data) {
		return nil, errors.New("signed message truncated at public key")
	}
	sm.PublicKey = make([]byte, pubKeyLen)
	copy(sm.PublicKey, data[offset:offset+pubKeyLen])
	offset += pubKeyLen

	if offset+2 > len(data) {
		return nil, errors.New("signed message truncated at signature length")
	}
	sigLen := int(binary.BigEndian.Uint16(data[offset:]))
	offset += 2

	if offset+sigLen+4 > len(data) {
		return nil, errors.New("signed message truncated at signature")
	}
	sm.Signature = make([]byte, sigLen)
	copy(sm.Signature, data[offset:offset+sigLen])
	offset += sigLen

	if offset+4 > len(data) {
		return nil, errors.New("signed message truncated at message length")
	}
	msgLen := int(binary.BigEndian.Uint32(data[offset:]))
	offset += 4

	if offset+msgLen > len(data) {
		return nil, errors.New("signed message truncated at message bytes")
	}
	sm.MessageBytes = make([]byte, msgLen)
	copy(sm.MessageBytes, data[offset:offset+msgLen])

	return sm, nil
}

func SignMessage(msg *Message, privKey *crypto.PrivateKey, chainID uint64) (*SignedMessage, error) {
	msgBytes, err := msg.Encode()
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Unix()

	signingPayload := make([]byte, 8+8+len(msgBytes))
	binary.BigEndian.PutUint64(signingPayload[:8], uint64(timestamp))
	binary.BigEndian.PutUint64(signingPayload[8:16], chainID)
	copy(signingPayload[16:], msgBytes)

	sig, err := privKey.Sign(signingPayload)
	if err != nil {
		return nil, err
	}

	pubKey := privKey.PubKey()
	pubKeyBytes := pubKey.Compressed()

	return &SignedMessage{
		MessageBytes: msgBytes,
		PublicKey:    pubKeyBytes,
		Signature:    sig.Bytes(),
		Timestamp:    timestamp,
		ChainID:      chainID,
	}, nil
}

func VerifySignedMessage(sm *SignedMessage, chainID uint64, maxAge time.Duration) error {
	if maxAge == 0 {
		maxAge = DefaultMaxMessageAge
	}

	now := time.Now().Unix()
	if sm.Timestamp > now+5 {
		return ErrInvalidTimestamp
	}
	if time.Duration(now-sm.Timestamp)*time.Second > maxAge {
		return ErrSignatureExpired
	}

	if sm.ChainID != chainID {
		return ErrChainIDMismatch
	}

	pubKey, err := crypto.DecompressPubKey(sm.PublicKey)
	if err != nil {
		return ErrInvalidPublicKey
	}

	sig, err := crypto.SignatureFromBytes(sm.Signature)
	if err != nil {
		return ErrInvalidSignature
	}

	signingPayload := make([]byte, 8+8+len(sm.MessageBytes))
	binary.BigEndian.PutUint64(signingPayload[:8], uint64(sm.Timestamp))
	binary.BigEndian.PutUint64(signingPayload[8:16], sm.ChainID)
	copy(signingPayload[16:], sm.MessageBytes)

	if !pubKey.Verify(signingPayload, sig) {
		return ErrInvalidSignature
	}

	return nil
}

func GetSenderAddress(sm *SignedMessage) ([]byte, error) {
	pubKey, err := crypto.DecompressPubKey(sm.PublicKey)
	if err != nil {
		return nil, ErrInvalidPublicKey
	}
	return pubKey.Address(), nil
}

type MessageAuthenticator struct {
	privKey *crypto.PrivateKey
	chainID uint64
	maxAge  time.Duration
}

func NewMessageAuthenticator(privKey *crypto.PrivateKey, chainID uint64, maxAge time.Duration) *MessageAuthenticator {
	if maxAge == 0 {
		maxAge = DefaultMaxMessageAge
	}
	return &MessageAuthenticator{
		privKey: privKey,
		chainID: chainID,
		maxAge:  maxAge,
	}
}

func (ma *MessageAuthenticator) Sign(msg *Message) (*SignedMessage, error) {
	return SignMessage(msg, ma.privKey, ma.chainID)
}

func (ma *MessageAuthenticator) Verify(sm *SignedMessage) error {
	return VerifySignedMessage(sm, ma.chainID, ma.maxAge)
}

func (ma *MessageAuthenticator) GetPeerID() string {
	pubKey := ma.privKey.PubKey()
	pubKeyBytes := pubKey.Compressed()
	if len(pubKeyBytes) == 0 {
		return "unknown"
	}
	hexStr := hex.EncodeToString(pubKeyBytes)
	if len(hexStr) > 16 {
		return hexStr[:16]
	}
	return hexStr
}

func (ma *MessageAuthenticator) PublicKey() *crypto.PublicKey {
	return ma.privKey.PubKey()
}

func (ma *MessageAuthenticator) ValidatorAddress() []byte {
	return ma.privKey.PubKey().Address()
}

func (ma *MessageAuthenticator) ChainID() uint64 {
	return ma.chainID
}
