// Package httpapi wires HTTP routes, authentication, and JSON responses.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/gedex/batasd/internal/config"
	"github.com/gedex/batasd/internal/language"
	"github.com/gedex/batasd/internal/queue/memory"
	"github.com/gedex/batasd/internal/status"
	"github.com/gedex/batasd/internal/submission"
	"github.com/gedex/batasd/internal/worker"
)

// Dependencies contains the services needed by the HTTP router.
type Dependencies struct {
	Config      config.Config
	Logger      *slog.Logger
	Submissions *submission.Service
	Languages   *language.Registry
	Queue       *memory.Queue
	Workers     *worker.Monitor
}

// NewRouter builds the HTTP handler tree for the public API.
func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(maxBodyBytes(deps.Config.HTTP.MaxBodyBytes))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	auth := AuthMiddleware(deps.Config.Auth)

	r.Route("/v1", func(r chi.Router) {
		r.Use(auth)

		r.Post("/authenticate", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"authenticated": true})
		})

		r.Get("/languages", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, deps.Languages.List())
		})

		r.Get("/languages/{slug}", func(w http.ResponseWriter, r *http.Request) {
			lang, ok := deps.Languages.Get(chi.URLParam(r, "slug"))
			if !ok {
				writeError(w, http.StatusNotFound, "not_found", "language not found")
				return
			}
			writeJSON(w, http.StatusOK, lang)
		})

		r.Get("/statuses", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, status.List())
		})

		r.Get("/config", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, publicConfig(deps.Config))
		})

		r.Get("/system", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"environment": deps.Config.AppEnv,
				"sandbox":     deps.Config.Sandbox.Driver,
			})
		})

		r.Get("/workers", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"configured_workers": deps.Config.Queue.Workers,
				"queue":              deps.Queue.Stats(),
				"workers":            deps.Workers.Snapshot(time.Now().UTC()),
			})
		})

		submissionHandler := SubmissionHandler{Service: deps.Submissions}
		r.Get("/submissions", submissionHandler.List)
		r.Post("/submissions", submissionHandler.Create)
		r.Get("/submissions/{token}/callbacks", submissionHandler.CallbackAttempts)
		r.Get("/submissions/{token}", submissionHandler.Get)
	})

	return r
}

func publicConfig(cfg config.Config) map[string]any {
	return map[string]any{
		"environment": cfg.AppEnv,
		"http": map[string]any{
			"max_body_bytes": cfg.HTTP.MaxBodyBytes,
		},
		"sandbox": map[string]any{
			"driver":         cfg.Sandbox.Driver,
			"docker_image":   cfg.Sandbox.DockerImage,
			"isolate_cgroup": cfg.Sandbox.IsolateControlGroup,
		},
		"submissions": map[string]any{
			"default_limits": cfg.Submissions.DefaultLimits,
			"max_limits":     cfg.Submissions.MaxLimits,
		},
		"callbacks": map[string]any{
			"timeout_ms": cfg.Callback.Timeout.Milliseconds(),
		},
	}
}
