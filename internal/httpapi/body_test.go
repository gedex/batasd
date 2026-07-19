package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBodyBytesMapsJSONDecodeErrorToRequestTooLarge(t *testing.T) {
	handler := maxBodyBytes(4)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSONDecodeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, payload)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/submissions", strings.NewReader(`{"hello":"world"}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body.String())
	}
	var response errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "request_too_large" {
		t.Fatalf("error code = %q, want request_too_large", response.Error.Code)
	}
}

func TestWriteJSONDecodeErrorMapsInvalidJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeJSONDecodeError(recorder, &json.SyntaxError{})

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	var response errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "invalid_json" {
		t.Fatalf("error code = %q, want invalid_json", response.Error.Code)
	}
}
