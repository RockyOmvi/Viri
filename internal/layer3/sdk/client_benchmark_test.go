package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkHealthCheck(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewL3Client(srv.URL)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.HealthCheck()
	}
}
