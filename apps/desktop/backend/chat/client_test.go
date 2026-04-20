package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSend_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"ok"}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL)
	resp, appErr := client.Send(context.Background(), ChatRequest{SessionID: "s1", Message: "hello"})
	if appErr != nil {
		t.Fatalf("unexpected error: %+v", appErr)
	}
	if resp.Response != "ok" {
		t.Fatalf("unexpected response: %s", resp.Response)
	}
}

func TestClientSend_ErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		expectCode string
	}{
		{name: "400", statusCode: 400, body: `{"error":{"code":"bad_request","message":"invalid json"}}`, expectCode: "bad_request"},
		{name: "422", statusCode: 422, body: `{"error":{"code":"invalid_argument","message":"session_id and message are required"}}`, expectCode: "invalid_argument"},
		{name: "502", statusCode: 502, body: `{"error":{"code":"upstream_error","message":"llm request failed"}}`, expectCode: "upstream_error"},
		{name: "500", statusCode: 500, body: `{"error":{"code":"internal_error","message":"internal error"}}`, expectCode: "internal_error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer ts.Close()

			client := NewClient(ts.URL)
			_, appErr := client.Send(context.Background(), ChatRequest{SessionID: "s1", Message: "hello"})
			if appErr == nil {
				t.Fatalf("expected error")
			}
			if appErr.StatusCode != tc.statusCode {
				t.Fatalf("unexpected status code: %d", appErr.StatusCode)
			}
			if appErr.Code != tc.expectCode {
				t.Fatalf("unexpected code: %s", appErr.Code)
			}
		})
	}
}

func TestClientSend_ConfigAndValidation(t *testing.T) {
	client := NewClient("")
	_, appErr := client.Send(context.Background(), ChatRequest{SessionID: "s1", Message: "hello"})
	if appErr == nil || appErr.Code != "config_error" {
		t.Fatalf("expected config_error, got %+v", appErr)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"response":"ok"}`))
	}))
	defer ts.Close()

	client = NewClient(ts.URL)
	_, appErr = client.Send(context.Background(), ChatRequest{SessionID: "", Message: "hello"})
	if appErr == nil || appErr.Code != "invalid_argument" {
		t.Fatalf("expected invalid_argument for empty session, got %+v", appErr)
	}
}
