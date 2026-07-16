package httpapi

import (
	"net/http"
	"strings"

	"github.com/gedex/batasd/internal/config"
)

const bearerChallenge = `Bearer realm="batasd"`

// AuthMiddleware rejects requests without a valid bearer token.
func AuthMiddleware(cfg config.AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(cfg.Tokens) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				writeAuthError(w, "")
				return
			}
			if _, ok := cfg.Tokens[token]; !ok {
				writeAuthError(w, "invalid_token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func writeAuthError(w http.ResponseWriter, bearerError string) {
	challenge := bearerChallenge
	if bearerError != "" {
		challenge += `, error="` + bearerError + `"`
	}
	w.Header().Set("WWW-Authenticate", challenge)
	writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or missing bearer token")
}
