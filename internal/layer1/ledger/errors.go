package ledger

import "errors"

var (
	ErrInvalidBlock      = errors.New("invalid block")
	ErrInvalidHeight     = errors.New("invalid block height")
	ErrInvalidPrevHash   = errors.New("invalid previous block hash")
	ErrGenesisExists     = errors.New("genesis block already exists")
	ErrDuplicateTx       = errors.New("duplicate transaction")
	ErrNonceTooLow       = errors.New("nonce too low")
	ErrGasLimitExceeded  = errors.New("gas limit exceeded")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrTxPoolFull        = errors.New("transaction pool is full")
	ErrGasPriceTooLow    = errors.New("gas price below minimum")
	ErrAccountTxLimit    = errors.New("account transaction limit reached")
	ErrInvalidSignature  = errors.New("invalid transaction signature")
	ErrEmptyBlock        = errors.New("block has no transactions")
	ErrMaxSupplyExceeded = errors.New("max supply would be exceeded")
	ErrValidatorJailed   = errors.New("validator is jailed")
	ErrAlreadySlashed    = errors.New("validator already slashed for this offense")
)
