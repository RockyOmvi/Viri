# State Management

Viri uses a **Merkle-Patricia Trie** for state storage, similar to Ethereum.

## State Structure

```
StateRoot = MerkleRoot(AccountTrie)
  ├── Account1 (nonce, balance, codeHash, storageRoot)
  ├── Account2 (nonce, balance, codeHash, storageRoot)
  └── ...
```

## Storage Backend

- **BadgerDB**: Embedded key-value store (LSM tree)
- **In-Memory**: Optional memory store for testing
- **Key Prefixes**: Namespaced storage for different data types

## State Pruning

The `StatePruner` periodically removes old state data:

- Configurable pruning depth
- Interface-based design (`StateDeleter`) for testability
- Safe cleanup of finalized blocks

## State Snapshots

Periodic snapshots enable fast node recovery:

- Export to JSON format
- Import on node startup
- Integrity verification via state root comparison
