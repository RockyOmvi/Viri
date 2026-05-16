package state

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

type BadgerStore struct {
	db *badger.DB
}

func NewBadgerStore(dir string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(dir)
	opts = opts.WithLogger(nil)
	opts = opts.WithValueLogFileSize(1 << 20)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	return &BadgerStore{db: db}, nil
}

func (b *BadgerStore) Get(key []byte) ([]byte, error) {
	var val []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err = item.ValueCopy(nil)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("key not found: %w", ErrKeyNotFound)
	}
	return val, nil
}

func (b *BadgerStore) Put(key []byte, value []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

func (b *BadgerStore) Delete(key []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

func (b *BadgerStore) Has(key []byte) (bool, error) {
	exists := false
	err := b.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
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

func (b *BadgerStore) Close() error {
	return b.db.Close()
}

func (b *BadgerStore) DB() *badger.DB {
	return b.db
}

func (b *BadgerStore) Batch() Batch {
	return &BadgerBatch{db: b.db}
}

func (b *BadgerStore) Iterator(prefix []byte) (Iterator, error) {
	txn := b.db.NewTransaction(false)
	opts := badger.DefaultIteratorOptions
	opts.Prefix = prefix
	it := txn.NewIterator(opts)

	return &BadgerIterator{
		it:  it,
		txn: txn,
		prefix: prefix,
	}, nil
}

type BadgerBatch struct {
	db  *badger.DB
	ops []batchOp
}

func (b *BadgerBatch) Put(key []byte, value []byte) error {
	k := make([]byte, len(key))
	copy(k, key)
	v := make([]byte, len(value))
	copy(v, value)
	b.ops = append(b.ops, batchOp{key: k, value: v})
	return nil
}

func (b *BadgerBatch) Delete(key []byte) error {
	k := make([]byte, len(key))
	copy(k, key)
	b.ops = append(b.ops, batchOp{key: k, delete: true})
	return nil
}

func (b *BadgerBatch) Write() error {
	err := b.db.Update(func(txn *badger.Txn) error {
		for _, op := range b.ops {
			if op.delete {
				if err := txn.Delete(op.key); err != nil {
					return err
				}
			} else {
				if err := txn.Set(op.key, op.value); err != nil {
					return err
				}
			}
		}
		return nil
	})
	b.ops = b.ops[:0]
	return err
}

func (b *BadgerBatch) Reset() {
	b.ops = b.ops[:0]
}

type BadgerIterator struct {
	it     *badger.Iterator
	txn    *badger.Txn
	prefix []byte
	seeked bool
}

func (bi *BadgerIterator) Next() bool {
	if !bi.seeked {
		bi.it.Rewind()
		bi.seeked = true
		if bi.prefix != nil {
			bi.it.Seek(bi.prefix)
		}
		return bi.it.Valid()
	}
	bi.it.Next()
	return bi.it.Valid()
}

func (bi *BadgerIterator) Key() []byte {
	if !bi.it.Valid() {
		return nil
	}
	item := bi.it.Item()
	return item.Key()
}

func (bi *BadgerIterator) Value() []byte {
	if !bi.it.Valid() {
		return nil
	}
	item := bi.it.Item()
	val, _ := item.ValueCopy(nil)
	return val
}

func (bi *BadgerIterator) Error() error {
	return nil
}

func (bi *BadgerIterator) Close() error {
	bi.it.Close()
	bi.txn.Discard()
	return nil
}
