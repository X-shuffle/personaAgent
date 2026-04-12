package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"persona_agent/internal/ingestion"
	"persona_agent/internal/model"
)

// IngestHandler handles POST /ingest.
type IngestHandler struct {
	Ingestor       ingestion.Service
	MaxUploadBytes int64
}

func (h IngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.MaxUploadBytes > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, h.MaxUploadBytes)
	}

	req, err := h.parseRequest(r)
	if err != nil {
		switch {
		case errors.Is(err, ingestion.ErrUnsupportedFormat):
			writeError(w, http.StatusBadRequest, "unsupported_format", err.Error())
		default:
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		}
		return
	}

	result, err := h.Ingestor.Ingest(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ingestion.ErrInvalidInput):
			writeError(w, http.StatusUnprocessableEntity, "invalid_argument", err.Error())
		case errors.Is(err, ingestion.ErrUnsupportedFormat):
			writeError(w, http.StatusBadRequest, "unsupported_format", err.Error())
		case errors.Is(err, ingestion.ErrNoValidMessages):
			writeError(w, http.StatusUnprocessableEntity, "no_valid_messages", err.Error())
		case errors.Is(err, ingestion.ErrDisabled):
			writeError(w, http.StatusServiceUnavailable, "unavailable", err.Error())
		default:
			// 这里返回真实错误，便于定位 embedding 维度或上游依赖问题。
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, model.IngestResponse{
		Status:    "ok",
		SessionID: result.SessionID,
		Source:    result.Source,
		Accepted:  result.Accepted,
		Rejected:  result.Rejected,
		Segments:  result.Segments,
		Stored:    result.Stored,
		DryRun:    result.DryRun,
		Warnings:  result.Warnings,
	})
}

func (h IngestHandler) parseRequest(r *http.Request) (ingestion.Request, error) {
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return parseMultipartIngestRequest(r)
	}

	var body model.IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return ingestion.Request{}, fmt.Errorf("invalid json")
	}
	return ingestion.Request{
		SessionID: strings.TrimSpace(body.SessionID),
		Source:    strings.TrimSpace(body.Source),
		Format:    strings.TrimSpace(body.Format),
		Filename:  "",
		Data:      []byte(body.Data),
		DryRun:    body.DryRun,
	}, nil
}

func parseMultipartIngestRequest(r *http.Request) (ingestion.Request, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return ingestion.Request{}, fmt.Errorf("invalid multipart form")
	}
	sessionID := strings.TrimSpace(r.FormValue("session_id"))
	source := strings.TrimSpace(r.FormValue("source"))
	format := strings.TrimSpace(r.FormValue("format"))
	dryRun := strings.EqualFold(strings.TrimSpace(r.FormValue("dry_run")), "true")

	file, header, err := r.FormFile("file")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return ingestion.Request{}, fmt.Errorf("file is required")
		}
		return ingestion.Request{}, fmt.Errorf("read form file: %w", err)
	}
	defer file.Close()

	data, err := readAllUploaded(file)
	if err != nil {
		return ingestion.Request{}, fmt.Errorf("read uploaded file: %w", err)
	}
	return ingestion.Request{
		SessionID: sessionID,
		Source:    source,
		Format:    format,
		Filename:  header.Filename,
		Data:      data,
		DryRun:    dryRun,
	}, nil
}

func readAllUploaded(file multipart.File) ([]byte, error) {
	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return b, nil
}
