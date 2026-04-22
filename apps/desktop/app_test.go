package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"desktop/backend/chat"
	"desktop/backend/history"
)

func TestAppSendChat_PersistsSuccessTurns(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"收到"}`))
	}))
	defer ts.Close()

	store := newHistoryStoreForAppTest(t)

	app := &App{
		ctx:          context.Background(),
		chatClient:   chat.NewClient(ts.URL),
		historyStore: store,
		sessionID:    fixedSessionID,
	}

	result := app.SendChat("你好")
	if result.Error != nil {
		t.Fatalf("unexpected error: %+v", result.Error)
	}
	if result.Response != "收到" {
		t.Fatalf("unexpected response: %s", result.Response)
	}

	messages, err := store.GetMessages(context.Background(), fixedSessionID, 10, 0)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != history.RoleUser || messages[0].Content != "你好" {
		t.Fatalf("unexpected user message: %+v", messages[0])
	}
	if messages[1].Role != history.RoleAssistant || messages[1].Status != history.StatusOK || messages[1].Content != "收到" {
		t.Fatalf("unexpected assistant message: %+v", messages[1])
	}
}

func TestAppSendChat_DoesNotPersistErrorTurn(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"code":"upstream_error","message":"llm request failed"}}`))
	}))
	defer ts.Close()

	store := newHistoryStoreForAppTest(t)

	app := &App{
		ctx:          context.Background(),
		chatClient:   chat.NewClient(ts.URL),
		historyStore: store,
		sessionID:    fixedSessionID,
	}

	result := app.SendChat("请总结一下")
	if result.Error == nil {
		t.Fatal("expected error")
	}
	if result.Error.Code != "upstream_error" {
		t.Fatalf("unexpected error code: %s", result.Error.Code)
	}

	messages, err := store.GetMessages(context.Background(), fixedSessionID, 10, 0)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(messages))
	}
}

func TestAppSearchHistory(t *testing.T) {
	store := newHistoryStoreForAppTest(t)
	ctx := context.Background()

	if err := store.UpsertSession(ctx, fixedSessionID, "测试会话"); err != nil {
		t.Fatalf("upsert session: %v", err)
	}
	if _, err := store.PersistUserTurn(ctx, fixedSessionID, "今天我们讨论历史搜索"); err != nil {
		t.Fatalf("persist user turn: %v", err)
	}
	if _, err := store.PersistAssistantTurn(ctx, fixedSessionID, "历史搜索支持关键词匹配"); err != nil {
		t.Fatalf("persist assistant turn: %v", err)
	}

	app := &App{
		ctx:          ctx,
		historyStore: store,
		sessionID:    fixedSessionID,
	}

	hits, err := app.SearchHistory("关键词", 10, 0)
	if err != nil {
		t.Fatalf("search history: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].SessionID != fixedSessionID || hits[0].Role != history.RoleAssistant {
		t.Fatalf("unexpected hit: %+v", hits[0])
	}
}

func newHistoryStoreForAppTest(t *testing.T) *history.Store {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "history.sqlite")
	store, err := history.Open(dbPath)
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate history store: %v", err)
	}

	return store
}
