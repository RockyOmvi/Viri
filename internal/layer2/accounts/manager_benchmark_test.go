package accounts

import "testing"

func BenchmarkAccountTransfer(b *testing.B) {
	am := NewAccountManager()
	from := []byte("from")
	to := []byte("to")
	_, _ = am.CreateAccount(from, AccountTypeNormal, 1000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = am.Transfer(from, to, 1)
	}
}
