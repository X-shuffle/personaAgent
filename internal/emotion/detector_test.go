package emotion

import (
	"context"
	"errors"
	"testing"
	"time"

	"persona_agent/internal/model"
)

type fakeLLMClient struct {
	resp       model.LLMResponse
	err        error
	req        model.LLMRequest
	reqContext context.Context
}

func (f *fakeLLMClient) Generate(ctx context.Context, req model.LLMRequest) (model.LLMResponse, error) {
	f.req = req
	f.reqContext = ctx
	if f.err != nil {
		return model.LLMResponse{}, f.err
	}
	return f.resp, nil
}

func TestLLMDetector_Detect_OK(t *testing.T) {
	client := &fakeLLMClient{resp: model.LLMResponse{Text: `{"label":"sad","intensity":0.8}`}}
	detector := LLMDetector{Client: client, Timeout: 20 * time.Second}

	state, err := detector.Detect(context.Background(), "我今天很难过")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if state.Label != "sad" || state.Intensity != 0.8 {
		t.Fatalf("unexpected state: %+v", state)
	}
	if len(client.req.Messages) != 2 {
		t.Fatalf("expected classifier request with 2 messages")
	}
	if client.req.MaxTokens != 80 || client.req.Temperature != 0 {
		t.Fatalf("unexpected llm request params: %+v", client.req)
	}
}

func TestLLMDetector_Detect_UsesConfiguredTimeout(t *testing.T) {
	client := &fakeLLMClient{resp: model.LLMResponse{Text: `{"label":"neutral","intensity":0}`}}
	detector := LLMDetector{Client: client, Timeout: 20 * time.Second}

	_, err := detector.Detect(context.Background(), "test")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	deadline, ok := client.reqContext.Deadline()
	if !ok {
		t.Fatalf("expected request context deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > 20*time.Second {
		t.Fatalf("expected deadline within 20s, got %v", remaining)
	}
}

func TestLLMDetector_Detect_InvalidJSON(t *testing.T) {
	client := &fakeLLMClient{resp: model.LLMResponse{Text: `not-json`}}
	detector := LLMDetector{Client: client}

	_, err := detector.Detect(context.Background(), "我今天很难过")
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestLLMDetector_Detect_InvalidLabel(t *testing.T) {
	client := &fakeLLMClient{resp: model.LLMResponse{Text: `{"label":"confused","intensity":0.8}`}}
	detector := LLMDetector{Client: client}

	_, err := detector.Detect(context.Background(), "我今天很难过")
	if err == nil {
		t.Fatalf("expected invalid label error")
	}
}

func TestLLMDetector_Detect_ClientError(t *testing.T) {
	client := &fakeLLMClient{err: errors.New("boom")}
	detector := LLMDetector{Client: client}

	_, err := detector.Detect(context.Background(), "我今天很难过")
	if err == nil {
		t.Fatalf("expected client error")
	}
}
