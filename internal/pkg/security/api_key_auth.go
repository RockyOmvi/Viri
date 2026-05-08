package security

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

type APIKeyAuth struct {
	keyHash string
}

func NewAPIKeyAuth(apiKey string) *APIKeyAuth {
	if apiKey == "" {
		return &APIKeyAuth{keyHash: ""}
	}
	return &APIKeyAuth{
		keyHash: hashAPIKey(apiKey),
	}
}

func NewAPIKeyAuthFromHash(keyHash string) *APIKeyAuth {
	return &APIKeyAuth{
		keyHash: keyHash,
	}
}

func (a *APIKeyAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.keyHash == "" {
			next.ServeHTTP(w, r)
			return
		}

		key := ExtractAPIKey(r)
		if key == "" {
			http.Error(w, `{"error":"missing API key"}`, http.StatusUnauthorized)
			return
		}

		if subtle.ConstantTimeCompare([]byte(hashAPIKey(key)), []byte(a.keyHash)) != 1 {
			http.Error(w, `{"error":"invalid API key"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (a *APIKeyAuth) IsValid(key string) bool {
	return subtle.ConstantTimeCompare([]byte(hashAPIKey(key)), []byte(a.keyHash)) == 1
}

func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func ExtractAPIKey(r *http.Request) string {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}

	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}

	return r.URL.Query().Get("api_key")
}
