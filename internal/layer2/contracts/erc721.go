package contracts

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"sync"
)

type ERC721Selector [4]byte

var (
	sel721BalanceOf        = ERC721Selector{0x70, 0xa0, 0x82, 0x31}
	sel721OwnerOf          = ERC721Selector{0x63, 0x52, 0x21, 0x1e}
	sel721TransferFrom     = ERC721Selector{0x23, 0xb8, 0x72, 0xdd}
	sel721SafeTransferFrom = ERC721Selector{0x42, 0x84, 0x2e, 0x0e}
	sel721Approve          = ERC721Selector{0x09, 0x5e, 0xa7, 0xb3}
	sel721SetApprovalForAll = ERC721Selector{0xa2, 0x2c, 0xb4, 0x65}
	sel721GetApproved      = ERC721Selector{0x08, 0x18, 0x12, 0xfc}
	sel721IsApprovedForAll = ERC721Selector{0xe9, 0x85, 0xe7, 0xc5}
	sel721Name             = ERC721Selector{0x06, 0xfd, 0xde, 0x03}
	sel721Symbol           = ERC721Selector{0x95, 0xd8, 0x9b, 0x41}
	sel721TokenURI         = ERC721Selector{0xc8, 0x7f, 0x56, 0xbd}
	sel721TotalSupply      = ERC721Selector{0x18, 0x16, 0x0d, 0xdd}
	sel721Mint             = ERC721Selector{0xa0, 0x71, 0x0e, 0x8d} // custom mint(to, tokenId)
)

// ERC721Token is a native Go ERC721 NFT token contract.
type ERC721Token struct {
	mu          sync.RWMutex
	name        string
	symbol      string
	baseURI     string
	totalCount  uint64
	owners      map[uint64][]byte    // tokenId -> owner address
	balances    map[string]uint64    // owner address -> count
	tokenApprovals map[uint64][]byte // tokenId -> approved address
	operatorApprovals map[string]map[string]bool // owner+"|"+operator -> approved
	tokenURIs   map[uint64]string
}

// NewERC721Token creates a new ERC721 NFT contract.
func NewERC721Token(name, symbol, baseURI string) *ERC721Token {
	return &ERC721Token{
		name:    name,
		symbol:  symbol,
		baseURI: baseURI,
		owners:  make(map[uint64][]byte),
		balances: make(map[string]uint64),
		tokenApprovals: make(map[uint64][]byte),
		operatorApprovals: make(map[string]map[string]bool),
		tokenURIs: make(map[uint64]string),
	}
}

// ExecuteCall processes an ERC721 ABI-encoded call.
func (n *ERC721Token) ExecuteCall(caller, input []byte) ([]byte, error) {
	if len(input) < 4 {
		return nil, fmt.Errorf("input too short for selector")
	}
	var sel ERC721Selector
	copy(sel[:], input[:4])
	args := input[4:]

	switch sel {
	case sel721BalanceOf:
		return n.handleBalanceOf(args)
	case sel721OwnerOf:
		return n.handleOwnerOf(args)
	case sel721TransferFrom:
		return n.handleTransferFrom(caller, args)
	case sel721SafeTransferFrom:
		return n.handleSafeTransferFrom(caller, args)
	case sel721Approve:
		return n.handleApprove(caller, args)
	case sel721SetApprovalForAll:
		return n.handleSetApprovalForAll(caller, args)
	case sel721GetApproved:
		return n.handleGetApproved(args)
	case sel721IsApprovedForAll:
		return n.handleIsApprovedForAll(args)
	case sel721Name:
		return n.handleName()
	case sel721Symbol:
		return n.handleSymbol()
	case sel721TokenURI:
		return n.handleTokenURI(args)
	case sel721TotalSupply:
		return n.handleTotalSupply()
	case sel721Mint:
		return n.handleMint(caller, args)
	default:
		return nil, fmt.Errorf("unknown ERC721 selector: %x", sel)
	}
}

// Mint creates a new token and assigns it to the given address.
func (n *ERC721Token) Mint(to []byte, tokenID uint64, tokenURI string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, exists := n.owners[tokenID]; exists {
		return fmt.Errorf("token %d already exists", tokenID)
	}
	n.owners[tokenID] = append([]byte(nil), to...)
	n.balances[string(to)]++
	n.totalCount++
	if tokenURI != "" {
		n.tokenURIs[tokenID] = tokenURI
	}
	return nil
}

// Burn destroys a token.
func (n *ERC721Token) Burn(tokenID uint64) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	owner, exists := n.owners[tokenID]
	if !exists {
		return fmt.Errorf("token %d does not exist", tokenID)
	}
	delete(n.owners, tokenID)
	n.balances[string(owner)]--
	n.totalCount--
	delete(n.tokenApprovals, tokenID)
	delete(n.tokenURIs, tokenID)
	return nil
}

func (n *ERC721Token) handleBalanceOf(args []byte) ([]byte, error) {
	owner := readAddress(args)
	if owner == nil {
		return nil, fmt.Errorf("invalid balanceOf args")
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	count := n.balances[string(owner)]
	return padTo32(big.NewInt(int64(count)).Bytes()), nil
}

func (n *ERC721Token) handleOwnerOf(args []byte) ([]byte, error) {
	tokenID := readUint256(args)
	n.mu.RLock()
	defer n.mu.RUnlock()
	owner, exists := n.owners[tokenID.Uint64()]
	if !exists {
		return nil, fmt.Errorf("token %s does not exist", tokenID.String())
	}
	return padTo32(owner), nil
}

func (n *ERC721Token) handleTransferFrom(caller, args []byte) ([]byte, error) {
	if len(args) < 96 {
		return nil, fmt.Errorf("invalid transferFrom args")
	}
	from := readAddress(args)
	to := readAddress(args[32:])
	tokenID := readUint256(args[64:]).Uint64()

	n.mu.Lock()
	defer n.mu.Unlock()
	err := n.checkTransferAuth(caller, from, tokenID)
	if err != nil {
		return padTo32(big.NewInt(0).Bytes()), nil
	}
	n.executeTransfer(from, to, tokenID)
	return padTo32(big.NewInt(1).Bytes()), nil
}

func (n *ERC721Token) handleSafeTransferFrom(caller, args []byte) ([]byte, error) {
	if len(args) < 96 {
		return nil, fmt.Errorf("invalid safeTransferFrom args")
	}
	from := readAddress(args)
	to := readAddress(args[32:])
	tokenID := readUint256(args[64:]).Uint64()

	n.mu.Lock()
	defer n.mu.Unlock()
	err := n.checkTransferAuth(caller, from, tokenID)
	if err != nil {
		return padTo32(big.NewInt(0).Bytes()), nil
	}
	n.executeTransfer(from, to, tokenID)
	return padTo32(big.NewInt(1).Bytes()), nil
}

func (n *ERC721Token) handleApprove(caller, args []byte) ([]byte, error) {
	if len(args) < 64 {
		return nil, fmt.Errorf("invalid approve args")
	}
	approved := readAddress(args)
	tokenID := readUint256(args[32:]).Uint64()

	n.mu.Lock()
	defer n.mu.Unlock()
	owner, exists := n.owners[tokenID]
	if !exists {
		return nil, fmt.Errorf("token %d does not exist", tokenID)
	}
	if string(caller) != string(owner) {
		return padTo32(big.NewInt(0).Bytes()), nil
	}
	if len(approved) == 0 || new(big.Int).SetBytes(approved).Sign() == 0 {
		delete(n.tokenApprovals, tokenID)
	} else {
		n.tokenApprovals[tokenID] = append([]byte(nil), approved...)
	}
	return padTo32(big.NewInt(int64(n.totalCount)).Bytes()), nil
}

func (n *ERC721Token) handleMint(caller, args []byte) ([]byte, error) {
	if len(args) < 64 {
		return nil, fmt.Errorf("invalid mint args")
	}
	to := readAddress(args)
	tokenID := readUint256(args[32:]).Uint64()
	if err := n.Mint(to, tokenID, ""); err != nil {
		return padTo32(big.NewInt(0).Bytes()), nil
	}
	return padTo32(big.NewInt(1).Bytes()), nil
}

func (n *ERC721Token) handleSetApprovalForAll(caller, args []byte) ([]byte, error) {
	if len(args) < 64 {
		return nil, fmt.Errorf("invalid setApprovalForAll args")
	}
	operator := readAddress(args)
	approved := readBool(args[32:])

	n.mu.Lock()
	defer n.mu.Unlock()
	key := string(caller)
	if n.operatorApprovals[key] == nil {
		n.operatorApprovals[key] = make(map[string]bool)
	}
	n.operatorApprovals[key][string(operator)] = approved
	return padTo32(big.NewInt(1).Bytes()), nil
}

func (n *ERC721Token) handleGetApproved(args []byte) ([]byte, error) {
	tokenID := readUint256(args).Uint64()
	n.mu.RLock()
	defer n.mu.RUnlock()
	approved := n.tokenApprovals[tokenID]
	if approved == nil {
		return padTo32([]byte{}), nil
	}
	return padTo32(approved), nil
}

func (n *ERC721Token) handleIsApprovedForAll(args []byte) ([]byte, error) {
	if len(args) < 64 {
		return nil, fmt.Errorf("invalid isApprovedForAll args")
	}
	owner := readAddress(args)
	operator := readAddress(args[32:])

	n.mu.RLock()
	defer n.mu.RUnlock()
	operators := n.operatorApprovals[string(owner)]
	approved := operators != nil && operators[string(operator)]
	if approved {
		return padTo32(big.NewInt(1).Bytes()), nil
	}
	return padTo32(big.NewInt(0).Bytes()), nil
}

func (n *ERC721Token) handleName() ([]byte, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return encodeString(n.name), nil
}

func (n *ERC721Token) handleSymbol() ([]byte, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return encodeString(n.symbol), nil
}

func (n *ERC721Token) handleTokenURI(args []byte) ([]byte, error) {
	tokenID := readUint256(args).Uint64()
	n.mu.RLock()
	defer n.mu.RUnlock()
	uri, exists := n.tokenURIs[tokenID]
	if !exists {
		if n.baseURI != "" {
			uri = fmt.Sprintf("%s%d", n.baseURI, tokenID)
		} else {
			return encodeString(""), nil
		}
	}
	return encodeString(uri), nil
}

func (n *ERC721Token) handleTotalSupply() ([]byte, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return padTo32(big.NewInt(int64(n.totalCount)).Bytes()), nil
}

func (n *ERC721Token) checkTransferAuth(caller, from []byte, tokenID uint64) error {
	owner, exists := n.owners[tokenID]
	if !exists {
		return fmt.Errorf("token %d does not exist", tokenID)
	}
	if string(owner) != string(from) {
		return fmt.Errorf("from does not match owner")
	}
	if string(caller) == string(owner) {
		return nil
	}
	if approved, ok := n.tokenApprovals[tokenID]; ok && string(approved) == string(caller) {
		return nil
	}
	operators := n.operatorApprovals[string(owner)]
	if operators != nil && operators[string(caller)] {
		return nil
	}
	return fmt.Errorf("caller not authorized")
}

func (n *ERC721Token) executeTransfer(from, to []byte, tokenID uint64) {
	delete(n.tokenApprovals, tokenID)
	n.owners[tokenID] = append([]byte(nil), to...)
	n.balances[string(from)]--
	n.balances[string(to)]++
}

// ERC721EventData returns the ABI-encoded event data for Transfer events.
func ERC721EventData(from, to []byte, tokenID uint64) []byte {
	data := make([]byte, 32)
	binary.BigEndian.PutUint64(data[24:], tokenID)
	return data
}
