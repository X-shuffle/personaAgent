package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"persona_agent/internal/ingestion"
)

type stubIngestService struct {
	result ingestion.Result
	err    error
}

func (s stubIngestService) Ingest(_ context.Context, _ ingestion.Request) (ingestion.Result, error) {
	if s.err != nil {
		return ingestion.Result{}, s.err
	}
	return s.result, nil
}

func TestIngestHandler_MethodNotAllowed(t *testing.T) {
	h := IngestHandler{Ingestor: stubIngestService{}}
	req := httptest.NewRequest(http.MethodGet, "/ingest", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestIngestHandler_BadJSON(t *testing.T) {
	h := IngestHandler{Ingestor: stubIngestService{}}
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestIngestHandler_JSON_OK(t *testing.T) {
	h := IngestHandler{Ingestor: stubIngestService{result: ingestion.Result{SessionID: "s1", Source: "wechat", Accepted: 2, Rejected: 1, Segments: 2, Stored: 2}}}
	body := `{"session_id":"s1","format":"txt","data":"2026-04-11 10:00:00 A: hi"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("bad json response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", payload["status"])
	}
}

func TestIngestHandler_MultipartMissingFile(t *testing.T) {
	h := IngestHandler{Ingestor: stubIngestService{}}
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	_ = w.WriteField("session_id", "s1")
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestIngestHandler_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
	}{
		{name: "invalid", err: ingestion.ErrInvalidInput, code: http.StatusUnprocessableEntity},
		{name: "unsupported", err: ingestion.ErrUnsupportedFormat, code: http.StatusBadRequest},
		{name: "empty", err: ingestion.ErrNoValidMessages, code: http.StatusUnprocessableEntity},
		{name: "disabled", err: ingestion.ErrDisabled, code: http.StatusServiceUnavailable},
		{name: "internal", err: errors.New("boom"), code: http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := IngestHandler{Ingestor: stubIngestService{err: tc.err}}
			req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"session_id":"s1","data":"x"}`))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != tc.code {
				t.Fatalf("expected %d, got %d", tc.code, rr.Code)
			}
		})
	}
}
