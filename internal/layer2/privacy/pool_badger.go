package privacy

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

const (
	bPrefixNote       = "note:"
	bPrefixNullifier  = "nullifier:"
	bPrefixCommitment = "commitment:"
	bKeyTotalShielded = "total_shielded"
)

type BadgerBackend struct {
	db *badger.DB
}

func NewBadgerBackend(dir string) (*BadgerBackend, error) {
	opts := badger.DefaultOptions(dir).
		WithLogger(nil).
		WithSyncWrites(true)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("badger open: %w", err)
	}
	return &BadgerBackend{db: db}, nil
}

func (b *BadgerBackend) SaveNote(note *Note) error {
	data, err := json.Marshal(note)
	if err != nil {
		return fmt.Errorf("marshal note: %w", err)
	}
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(bPrefixNote+string(note.Nullifier)), data)
	})
}

func (b *BadgerBackend) SaveNullifier(nullifier []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(bPrefixNullifier+string(nullifier)), []byte{0x01})
	})
}

func (b *BadgerBackend) SaveCommitment(commitment []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(bPrefixCommitment+string(commitment)), []byte{0x01})
	})
}

func (b *BadgerBackend) HasCommitment(commitment []byte) (bool, error) {
	var exists bool
	err := b.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(bPrefixCommitment + string(commitment)))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	})
	return exists, err
}

func (b *BadgerBackend) HasNullifier(nullifier []byte) (bool, error) {
	var exists bool
	err := b.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(bPrefixNullifier + string(nullifier)))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	})
	return exists, err
}

func (b *BadgerBackend) LoadNotes() ([]*Note, error) {
	var notes []*Note
	err := b.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(bPrefixNote)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var note Note
				if err := json.Unmarshal(val, &note); err != nil {
					return fmt.Errorf("unmarshal note: %w", err)
				}
				notes = append(notes, &note)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if notes == nil {
		notes = make([]*Note, 0)
	}
	return notes, err
}

func (b *BadgerBackend) LoadNullifiers() (map[string]bool, error) {
	result := make(map[string]bool)
	err := b.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(bPrefixNullifier)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := string(it.Item().Key()[len(prefix):])
			result[key] = true
		}
		return nil
	})
	return result, err
}

func (b *BadgerBackend) LoadCommitments() (map[string]bool, error) {
	result := make(map[string]bool)
	err := b.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(bPrefixCommitment)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := string(it.Item().Key()[len(prefix):])
			result[key] = true
		}
		return nil
	})
	return result, err
}

func (b *BadgerBackend) SaveTotalShielded(value uint64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], value)
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(bKeyTotalShielded), buf[:])
	})
}

func (b *BadgerBackend) LoadTotalShielded() (uint64, error) {
	var val uint64
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(bKeyTotalShielded))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		return item.Value(func(v []byte) error {
			val = binary.BigEndian.Uint64(v)
			return nil
		})
	})
	return val, err
}

func (b *BadgerBackend) DeleteNote(nullifier []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(bPrefixNote + string(nullifier)))
	})
}

func (b *BadgerBackend) Close() error {
	return b.db.Close()
}
