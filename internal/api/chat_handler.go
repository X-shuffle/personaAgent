package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"persona_agent/internal/agent"
	"persona_agent/internal/model"
)

// ChatHandler handles POST /chat.
type ChatHandler struct {
	Orchestrator agent.Orchestrator
}

func (h ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req model.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid json")
		return
	}

	responseText, err := h.Orchestrator.Chat(r.Context(), req.SessionID, req.Message)
	if err != nil {
		switch {
		case errors.Is(err, agent.ErrInvalidInput):
			writeError(w, http.StatusUnprocessableEntity, "invalid_argument", err.Error())
		case errors.Is(err, agent.ErrUpstreamLLM):
			writeError(w, http.StatusBadGateway, "upstream_error", "llm request failed")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, model.ChatResponse{Response: responseText})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
