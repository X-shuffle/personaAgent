package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 15 * time.Second

type ChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type ChatResponse struct {
	Response string `json:"response"`
}

type Error struct {
	StatusCode int    `json:"status_code"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return "chat request failed"
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	trimmed := strings.TrimSpace(baseURL)
	trimmed = strings.TrimRight(trimmed, "/")

	return &Client{
		baseURL: trimmed,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

func (c *Client) Send(ctx context.Context, req ChatRequest) (ChatResponse, *Error) {
	if c == nil || c.baseURL == "" {
		return ChatResponse{}, &Error{Code: "config_error", Message: "DESKTOP_CHAT_BASE_URL is not set"}
	}

	sessionID := strings.TrimSpace(req.SessionID)
	message := strings.TrimSpace(req.Message)
	if sessionID == "" || message == "" {
		return ChatResponse{}, &Error{StatusCode: http.StatusUnprocessableEntity, Code: "invalid_argument", Message: "session_id and message are required"}
	}

	payload, err := json.Marshal(ChatRequest{SessionID: sessionID, Message: message})
	if err != nil {
		return ChatResponse{}, &Error{Code: "encode_error", Message: "failed to encode chat request"}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat", bytes.NewReader(payload))
	if err != nil {
		return ChatResponse{}, &Error{Code: "request_build_error", Message: "failed to build chat request"}
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ChatResponse{}, &Error{Code: "network_error", Message: "failed to reach backend chat service"}
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusOK {
		var result ChatResponse
		if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
			return ChatResponse{}, &Error{Code: "decode_error", Message: "failed to decode chat response"}
		}
		if strings.TrimSpace(result.Response) == "" {
			return ChatResponse{}, &Error{Code: "decode_error", Message: "chat response is empty"}
		}
		return result, nil
	}

	return ChatResponse{}, parseErrorEnvelope(httpResp.StatusCode, httpResp.Body)
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseErrorEnvelope(status int, body io.Reader) *Error {
	if body == nil {
		return &Error{StatusCode: status, Code: "http_error", Message: http.StatusText(status)}
	}

	var envelope errorEnvelope
	if err := json.NewDecoder(body).Decode(&envelope); err != nil {
		return &Error{StatusCode: status, Code: "http_error", Message: fmt.Sprintf("chat request failed with status %d", status)}
	}

	code := strings.TrimSpace(envelope.Error.Code)
	if code == "" {
		code = "http_error"
	}
	message := strings.TrimSpace(envelope.Error.Message)
	if message == "" {
		message = fmt.Sprintf("chat request failed with status %d", status)
	}

	return &Error{StatusCode: status, Code: code, Message: message}
}
