package history

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "history.sqlite")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	base := time.Unix(1700000000, 0)
	step := int64(0)
	store.now = func() time.Time {
		at := base.Add(time.Duration(step) * time.Second)
		step++
		return at
	}

	return store
}

func TestStoreMigrate_CreatesTablesAndIndexes(t *testing.T) {
	store := newTestStore(t)

	assertObjectExists(t, store.db, "table", "sessions")
	assertObjectExists(t, store.db, "table", "messages")
	assertObjectExists(t, store.db, "index", "idx_sessions_updated_at")
	assertObjectExists(t, store.db, "index", "idx_messages_session_created")
	assertObjectExists(t, store.db, "index", "idx_messages_created_at")
}

func TestStoreCRUDAndAutoPersist(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertSession(ctx, "s1", "First Session"); err != nil {
		t.Fatalf("upsert session: %v", err)
	}

	if _, err := store.PersistUserTurn(ctx, "s1", "你好，今天怎么样"); err != nil {
		t.Fatalf("persist user turn: %v", err)
	}
	if _, err := store.PersistAssistantTurn(ctx, "s1", "我很好，谢谢你"); err != nil {
		t.Fatalf("persist assistant turn: %v", err)
	}
	if _, err := store.PersistAssistantError(ctx, "s1", "upstream_error", "请求失败，请重试"); err != nil {
		t.Fatalf("persist assistant error: %v", err)
	}

	messages, err := store.GetMessages(ctx, "s1", 20, 0)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	if messages[0].Role != RoleUser || messages[0].Status != StatusOK {
		t.Fatalf("unexpected first message: %+v", messages[0])
	}
	if messages[2].Role != RoleAssistant || messages[2].Status != StatusError || messages[2].ErrorCode != "upstream_error" {
		t.Fatalf("unexpected error message: %+v", messages[2])
	}

	recent, err := store.LoadRecentSession(ctx, "s1", 2)
	if err != nil {
		t.Fatalf("load recent session: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent messages, got %d", len(recent))
	}
	if recent[0].Content != "我很好，谢谢你" || recent[1].Content != "请求失败，请重试" {
		t.Fatalf("unexpected recent messages order: %+v", recent)
	}

	sessions, err := store.ListSessions(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "s1" || sessions[0].Title != "First Session" {
		t.Fatalf("unexpected session: %+v", sessions[0])
	}
}

func TestStoreValidation(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if _, err := store.AppendMessage(ctx, "", RoleUser, "hi", StatusOK, ""); err == nil {
		t.Fatal("expected error for empty session id")
	}
	if _, err := store.AppendMessage(ctx, "s1", "system", "hi", StatusOK, ""); err == nil {
		t.Fatal("expected error for invalid role")
	}
	if _, err := store.AppendMessage(ctx, "s1", RoleUser, "hi", "done", ""); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func assertObjectExists(t *testing.T, db *sql.DB, typ, name string) {
	t.Helper()

	var count int
	err := db.QueryRow(`
SELECT COUNT(1)
FROM sqlite_master
WHERE type = ? AND name = ?;
`, typ, name).Scan(&count)
	if err != nil {
		t.Fatalf("query sqlite_master for %s %s: %v", typ, name, err)
	}
	if count != 1 {
		t.Fatalf("%s %s not found", typ, name)
	}
}

func TestStoreListSessionsOrder(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		sid := fmt.Sprintf("s%d", i+1)
		if err := store.UpsertSession(ctx, sid, sid); err != nil {
			t.Fatalf("upsert session %s: %v", sid, err)
		}
	}

	sessions, err := store.ListSessions(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}
	if sessions[0].ID != "s3" || sessions[1].ID != "s2" || sessions[2].ID != "s1" {
		t.Fatalf("unexpected order: %+v", sessions)
	}
}
