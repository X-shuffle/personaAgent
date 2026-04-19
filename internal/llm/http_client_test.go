package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"persona_agent/internal/model"
)

func TestHTTPClientGenerate_ParsesTextResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "test-model",
			"choices": []map[string]any{{
				"finish_reason": "stop",
				"message": map[string]any{"content": "hello"},
			}},
		})
	}))
	defer srv.Close()

	c := HTTPClient{Endpoint: srv.URL, Logger: zap.NewNop()}
	resp, err := c.Generate(context.Background(), model.LLMRequest{Messages: []model.LLMMessage{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Text != "hello" {
		t.Fatalf("expected hello, got %q", resp.Text)
	}
	if len(resp.ToolCalls) != 0 {
		t.Fatalf("expected no tool calls, got %d", len(resp.ToolCalls))
	}
}

func TestHTTPClientGenerate_ParsesToolCallsOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "test-model",
			"choices": []map[string]any{{
				"finish_reason": "tool_calls",
				"message": map[string]any{
					"content": "",
					"tool_calls": []map[string]any{{
						"id":   "call_1",
						"type": "function",
						"function": map[string]any{
							"name":      "demo_server::weather",
							"arguments": "{\"city\":\"shanghai\"}",
						},
					}},
				},
			}},
		})
	}))
	defer srv.Close()

	c := HTTPClient{Endpoint: srv.URL, Logger: zap.NewNop()}
	resp, err := c.Generate(context.Background(), model.LLMRequest{Messages: []model.LLMMessage{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "demo_server::weather" {
		t.Fatalf("unexpected function name: %s", resp.ToolCalls[0].Function.Name)
	}
}

func TestHTTPClientGenerate_SendsToolsInRequest(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "test-model",
			"choices": []map[string]any{{
				"finish_reason": "stop",
				"message": map[string]any{"content": "ok"},
			}},
		})
	}))
	defer srv.Close()

	c := HTTPClient{Endpoint: srv.URL, Logger: zap.NewNop()}
	_, err := c.Generate(context.Background(), model.LLMRequest{
		Messages:   []model.LLMMessage{{Role: "user", Content: "hi"}},
		ToolChoice: "auto",
		Tools: []model.LLMTool{{
			Type: "function",
			Function: model.LLMFunctionSpec{
				Name:       "demo_server::weather",
				Parameters: map[string]any{"type": "object"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if captured["tool_choice"] != "auto" {
		t.Fatalf("expected tool_choice auto, got %+v", captured["tool_choice"])
	}
	if _, ok := captured["tools"]; !ok {
		t.Fatal("expected tools in request payload")
	}
}

func TestHTTPClientGenerate_NormalizesToolMessagesInRequest_On(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "test-model",
			"choices": []map[string]any{{
				"finish_reason": "stop",
				"message": map[string]any{"content": "ok"},
			}},
		})
	}))
	defer srv.Close()

	c := HTTPClient{Endpoint: srv.URL, ToolPayloadNormalization: "on", Logger: zap.NewNop()}
	_, err := c.Generate(context.Background(), model.LLMRequest{
		Messages: []model.LLMMessage{
			{Role: "assistant", ToolCalls: []model.LLMToolCall{{
				ID:   "fc_call_function_x_1",
				Type: "function",
				Function: model.LLMFunctionCall{
					Name:      "demo",
					Arguments: "{} {\"a\":1}",
				},
			}}},
			{Role: "tool", ToolCallID: "fc_call_function_x_1", Content: "ok"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	messages, ok := captured["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("unexpected messages payload: %#v", captured["messages"])
	}
	assistant, _ := messages[0].(map[string]any)
	assistantCalls, _ := assistant["tool_calls"].([]any)
	if len(assistantCalls) != 1 {
		t.Fatalf("expected 1 assistant tool call, got %#v", assistant["tool_calls"])
	}
	call, _ := assistantCalls[0].(map[string]any)
	if call["id"] != "call_function_x_1" {
		t.Fatalf("expected normalized call id, got %#v", call["id"])
	}
	fn, _ := call["function"].(map[string]any)
	if fn["arguments"] != "{\"a\":1}" {
		t.Fatalf("expected normalized arguments, got %#v", fn["arguments"])
	}
	tool, _ := messages[1].(map[string]any)
	if tool["tool_call_id"] != "call_function_x_1" {
		t.Fatalf("expected normalized tool_call_id, got %#v", tool["tool_call_id"])
	}
}

func TestHTTPClientGenerate_DoesNotNormalizeToolMessagesInRequest_Off(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "test-model",
			"choices": []map[string]any{{
				"finish_reason": "stop",
				"message": map[string]any{"content": "ok"},
			}},
		})
	}))
	defer srv.Close()

	c := HTTPClient{Endpoint: srv.URL, ToolPayloadNormalization: "off", Logger: zap.NewNop()}
	_, err := c.Generate(context.Background(), model.LLMRequest{
		Messages: []model.LLMMessage{
			{Role: "assistant", ToolCalls: []model.LLMToolCall{{
				ID:   "fc_call_function_x_1",
				Type: "function",
				Function: model.LLMFunctionCall{Name: "demo", Arguments: "{} {\"a\":1}"},
			}}},
			{Role: "tool", ToolCallID: "fc_call_function_x_1", Content: "ok"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	messages, _ := captured["messages"].([]any)
	assistant, _ := messages[0].(map[string]any)
	assistantCalls, _ := assistant["tool_calls"].([]any)
	call, _ := assistantCalls[0].(map[string]any)
	if call["id"] != "fc_call_function_x_1" {
		t.Fatalf("expected raw call id, got %#v", call["id"])
	}
	fn, _ := call["function"].(map[string]any)
	if fn["arguments"] != "{} {\"a\":1}" {
		t.Fatalf("expected raw arguments, got %#v", fn["arguments"])
	}
	tool, _ := messages[1].(map[string]any)
	if tool["tool_call_id"] != "fc_call_function_x_1" {
		t.Fatalf("expected raw tool_call_id, got %#v", tool["tool_call_id"])
	}
}

func TestHTTPClientGenerate_NormalizesToolMessagesInRequest_AutoMinimax(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "test-model",
			"choices": []map[string]any{{
				"finish_reason": "stop",
				"message": map[string]any{"content": "ok"},
			}},
		})
	}))
	defer srv.Close()

	c := HTTPClient{Endpoint: srv.URL, ToolPayloadNormalization: "auto", Provider: "minimax", Logger: zap.NewNop()}
	_, err := c.Generate(context.Background(), model.LLMRequest{
		Messages: []model.LLMMessage{
			{Role: "assistant", ToolCalls: []model.LLMToolCall{{ID: "fc_call_function_x_1", Type: "function", Function: model.LLMFunctionCall{Name: "demo", Arguments: "{\"a\":1}"}}}},
			{Role: "tool", ToolCallID: "fc_call_function_x_1", Content: "ok"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	messages, _ := captured["messages"].([]any)
	assistant, _ := messages[0].(map[string]any)
	assistantCalls, _ := assistant["tool_calls"].([]any)
	call, _ := assistantCalls[0].(map[string]any)
	if call["id"] != "call_function_x_1" {
		t.Fatalf("expected normalized id in auto minimax, got %#v", call["id"])
	}
}
