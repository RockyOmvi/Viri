package accounts

import "testing"

func TestGetAccountMissing(t *testing.T) {
	am := NewAccountManager()
	if _, ok := am.GetAccount([]byte("missing")); ok {
		t.Fatalf("unexpected account")
	}
}
