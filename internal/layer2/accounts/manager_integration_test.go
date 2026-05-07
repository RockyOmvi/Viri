package accounts

import "testing"

func TestTransferCreatesRecipient(t *testing.T) {
	am := NewAccountManager()
	_, _ = am.CreateAccount([]byte("from"), AccountTypeNormal, 10)

	if err := am.Transfer([]byte("from"), []byte("to"), 5); err != nil {
		t.Fatalf("transfer failed: %v", err)
	}

	toAcc, ok := am.GetAccount([]byte("to"))
	if !ok || toAcc.Balance != 5 {
		t.Fatalf("recipient not created")
	}
}
