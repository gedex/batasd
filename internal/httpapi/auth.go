package httpapi

import (
	"net/http"
	"strings"

	"github.com/gedex/batasd/internal/config"
)

func AuthMiddleware(cfg config.AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(cfg.Tokens) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			token := strings.TrimSpace(r.Header.Get(cfg.Header))
			if token == "" {
				token = strings.TrimSpace(r.URL.Query().Get(cfg.Header))
			}
			if _, ok := cfg.Tokens[token]; !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or missing authentication token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
