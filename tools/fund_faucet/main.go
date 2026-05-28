package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
)

type account struct {
	Address     []byte            `json:"Address"`
	Type        uint8             `json:"Type"`
	Balance     string            `json:"Balance"`
	Nonce       uint64            `json:"Nonce"`
	Code        []byte            `json:"Code"`
	CodeHash    []byte            `json:"CodeHash"`
	StorageRoot []byte            `json:"StorageRoot"`
	Storage     map[string][]byte `json:"Storage"`
	Metadata    map[string]string `json:"Metadata"`
}

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("Usage: %s <badger-dir> <address-hex-with-0x> <balance-wei>\n"+
			"Example: %s /root/.viri/badger 0xdb02aaecf33fcb5d10b0e4eaf77ce04dae67890f 1000000000000000000",
			filepath.Base(os.Args[0]), filepath.Base(os.Args[0]))
	}

	dbDir := os.Args[1]
	addrHex := os.Args[2]
	balanceStr := os.Args[3]

	addr := parseHex(addrHex)
	if len(addr) != 20 {
		log.Fatalf("Invalid address: got %d bytes, want 20", len(addr))
	}

	fmt.Printf("Opening BadgerDB at: %s\n", dbDir)
	fmt.Printf("Target address: 0x%x\n", addr)
	fmt.Printf("Balance to set: %s wei\n", balanceStr)

	opts := badger.DefaultOptions(dbDir).WithLogger(nil)
	opts = opts.WithValueLogFileSize(1 << 20)

	db, err := badger.Open(opts)
	if err != nil {
		log.Fatalf("Failed to open BadgerDB: %v", err)
	}
	defer db.Close()

	key := append([]byte{0x01}, addr...)

	var existing []byte
	err = db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		existing, err = item.ValueCopy(nil)
		return err
	})
	if err == nil {
		fmt.Printf("Account already exists. Current value:\n%s\n", string(existing))
	} else {
		fmt.Println("Account does not exist yet. Creating new account.")
	}

	acct := account{
		Address:     addr,
		Type:        2,
		Balance:     balanceStr,
		Nonce:       0,
		Storage:     make(map[string][]byte),
		Metadata:    make(map[string]string),
	}

	data, err := json.Marshal(acct)
	if err != nil {
		log.Fatalf("Failed to marshal account: %v", err)
	}

	err = db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
	if err != nil {
		log.Fatalf("Failed to write account: %v", err)
	}

	fmt.Printf("Successfully wrote account (type=2 for AccountTypeValidator):\n%s\n", string(data))

	var verify []byte
	db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err == nil {
			verify, _ = item.ValueCopy(nil)
		}
		return nil
	})
	fmt.Printf("Verified readback: %s\n", string(verify))

	return
}

func parseHex(s string) []byte {
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	if len(s)%2 != 0 {
		s = "0" + s
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		b[i/2] = (nibble(s[i]) << 4) | nibble(s[i+1])
	}
	return b
}

func nibble(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}
