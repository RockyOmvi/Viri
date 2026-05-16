package accounts

import (
	"math/big"
	"testing"
)

func TestTransferCreatesRecipient(t *testing.T) {
	am := NewAccountManager()
	_, _ = am.CreateAccount([]byte("from"), AccountTypeNormal, 10)

	if err := am.Transfer([]byte("from"), []byte("to"), 5); err != nil {
		t.Fatalf("transfer failed: %v", err)
	}

	toAcc, ok := am.GetAccount([]byte("to"))
	if !ok || toAcc.Balance.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("recipient not created, balance=%s", toAcc.Balance)
	}
}
