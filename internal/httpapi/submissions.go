package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gedex/batasd/internal/submission"
)

// SubmissionHandler serves submission create and fetch endpoints.
type SubmissionHandler struct {
	Service          *submission.Service
	WaitTimeout      time.Duration
	WaitPollInterval time.Duration
}

// List handles submission list requests.
func (h SubmissionHandler) List(w http.ResponseWriter, r *http.Request) {
	query, err := parseSubmissionListQuery(r)
	if err != nil {
		var validation submission.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", validation.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_query", "query parameters are invalid")
		return
	}
	fields, err := parseSubmissionFields(r)
	if err != nil {
		var validation submission.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", validation.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_query", "query parameters are invalid")
		return
	}

	result, err := h.Service.List(r.Context(), query)
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
		writeError(w, http.StatusInternalServerError, "internal_error", "could not list submissions")
		return
	}

	writeJSON(w, http.StatusOK, listResponseValue(submission.ToListResponse(result), fields))
}

// Create handles submission creation requests.
func (h SubmissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	wait, err := parseSubmissionWait(r)
	if err != nil {
		var validation submission.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", validation.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_query", "query parameters are invalid")
		return
	}
	var fields []string
	if wait {
		fields, err = parseSubmissionFields(r)
		if err != nil {
			var validation submission.ValidationError
			if errors.As(err, &validation) {
				writeError(w, http.StatusUnprocessableEntity, "validation_failed", validation.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "invalid_query", "query parameters are invalid")
			return
		}
	}

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

	if wait {
		h.waitForCreatedSubmission(w, r, sub, fields)
		return
	}

	writeJSON(w, http.StatusCreated, submission.ToCreatedResponse(sub))
}

// Get handles submission fetch requests.
func (h SubmissionHandler) Get(w http.ResponseWriter, r *http.Request) {
	fields, err := parseSubmissionFields(r)
	if err != nil {
		var validation submission.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", validation.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_query", "query parameters are invalid")
		return
	}

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

	writeJSON(w, http.StatusOK, submissionResponseValue(submission.ToResponse(sub), fields))
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

func parseSubmissionListQuery(r *http.Request) (submission.ListQuery, error) {
	values := r.URL.Query()
	query := submission.ListQuery{
		BeforeToken: values.Get("before"),
		StatusCode:  values.Get("status"),
		Language:    values.Get("language"),
	}

	if rawLimit := strings.TrimSpace(values.Get("limit")); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil {
			return query, submission.ValidationError{Field: "limit", Message: "must be an integer"}
		}
		query.Limit = limit
	}

	return query, nil
}

func parseSubmissionWait(r *http.Request) (bool, error) {
	rawWait := strings.TrimSpace(r.URL.Query().Get("wait"))
	if rawWait == "" {
		return false, nil
	}

	wait, err := strconv.ParseBool(rawWait)
	if err != nil {
		return false, submission.ValidationError{Field: "wait", Message: "must be boolean"}
	}
	return wait, nil
}

func parseSubmissionFields(r *http.Request) ([]string, error) {
	rawFields := strings.TrimSpace(r.URL.Query().Get("fields"))
	if rawFields == "" || rawFields == "*" {
		return nil, nil
	}

	parts := strings.Split(rawFields, ",")
	fields := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		field := strings.TrimSpace(part)
		if field == "" {
			continue
		}
		if field == "*" {
			return nil, submission.ValidationError{Field: "fields", Message: "cannot combine * with other fields"}
		}
		if !submission.ValidResponseField(field) {
			return nil, submission.ValidationError{Field: "fields", Message: fmt.Sprintf("unsupported field %q", field)}
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
	}
	if len(fields) == 0 {
		return nil, nil
	}
	return fields, nil
}

func (h SubmissionHandler) waitForCreatedSubmission(w http.ResponseWriter, r *http.Request, sub *submission.Submission, fields []string) {
	timeout := h.WaitTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	waitedSub, err := h.Service.WaitForCompletion(ctx, sub.Token, h.WaitPollInterval)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			if waitedSub == nil {
				waitedSub = sub
			}
			writeJSON(w, http.StatusAccepted, submissionResponseValue(submission.ToResponse(waitedSub), fields))
			return
		}
		var validation submission.ValidationError
		if errors.As(err, &validation) {
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", validation.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "could not wait for submission")
		return
	}

	writeJSON(w, http.StatusCreated, submissionResponseValue(submission.ToResponse(waitedSub), fields))
}

func submissionResponseValue(response submission.Response, fields []string) any {
	if len(fields) == 0 {
		return response
	}
	return submission.SelectResponseFields(response, fields)
}

func listResponseValue(response submission.ListResponse, fields []string) any {
	if len(fields) == 0 {
		return response
	}
	return submission.SelectListResponseFields(response, fields)
}

func writeJSONDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the configured limit")
		return
	}
	writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
}
