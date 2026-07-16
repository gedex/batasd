package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gedex/batasd/internal/config"
)

func TestAuthMiddlewareAcceptsBearerToken(t *testing.T) {
	handler := AuthMiddleware(config.AuthConfig{
		Tokens: map[string]struct{}{"dev-token": {}},
	})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthMiddlewareAcceptsCaseInsensitiveBearerScheme(t *testing.T) {
	handler := AuthMiddleware(config.AuthConfig{
		Tokens: map[string]struct{}{"dev-token": {}},
	})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "bearer dev-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthMiddlewareRejectsMissingBearerToken(t *testing.T) {
	handler := AuthMiddleware(config.AuthConfig{
		Tokens: map[string]struct{}{"dev-token": {}},
	})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != bearerChallenge {
		t.Fatalf("WWW-Authenticate = %q, want %q", got, bearerChallenge)
	}
}

func TestAuthMiddlewareRejectsInvalidBearerToken(t *testing.T) {
	handler := AuthMiddleware(config.AuthConfig{
		Tokens: map[string]struct{}{"dev-token": {}},
	})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	want := `Bearer realm="batasd", error="invalid_token"`
	if got := rec.Header().Get("WWW-Authenticate"); got != want {
		t.Fatalf("WWW-Authenticate = %q, want %q", got, want)
	}
}

func TestAuthMiddlewareRejectsCustomAuthHeader(t *testing.T) {
	handler := AuthMiddleware(config.AuthConfig{
		Tokens: map[string]struct{}{"dev-token": {}},
	})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Auth-Token", "dev-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareAllowsRequestsWhenNoTokensConfigured(t *testing.T) {
	handler := AuthMiddleware(config.AuthConfig{})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
