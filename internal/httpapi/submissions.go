package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gedex/batasd/internal/submission"
)

// SubmissionHandler serves submission create and fetch endpoints.
type SubmissionHandler struct {
	Service *submission.Service
}

// Create handles submission creation requests.
func (h SubmissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req submission.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONDecodeError(w, err)
		return
	}

	sub, err := h.Service.Create(r.Context(), req)
	if err != nil {
		var validation submission.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", validation.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create submission")
		return
	}

	writeJSON(w, http.StatusCreated, submission.ToCreatedResponse(sub))
}

// Get handles submission fetch requests.
func (h SubmissionHandler) Get(w http.ResponseWriter, r *http.Request) {
	sub, err := h.Service.Get(r.Context(), chi.URLParam(r, "token"))
	if errors.Is(err, submission.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "submission not found")
		return
	}
	if err != nil {
		var validation submission.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", validation.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "could not fetch submission")
		return
	}

	writeJSON(w, http.StatusOK, submission.ToResponse(sub))
}

// CallbackAttempts handles callback attempt fetch requests.
func (h SubmissionHandler) CallbackAttempts(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	attempts, err := h.Service.ListCallbackAttempts(r.Context(), token)
	if errors.Is(err, submission.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "submission not found")
		return
	}
	if err != nil {
		var validation submission.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", validation.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "could not fetch callback attempts")
		return
	}

	writeJSON(w, http.StatusOK, submission.ToCallbackAttemptsResponse(token, attempts))
}

func writeJSONDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the configured limit")
		return
	}
	writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
}
