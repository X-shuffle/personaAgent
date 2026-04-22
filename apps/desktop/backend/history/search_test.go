package history

import (
	"context"
	"testing"
)

// 验证中文关键词可命中多条消息，并按时间倒序返回。
func TestSearchMessages_ChineseKeywordHit(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertSession(ctx, "s-cn", "中文会话"); err != nil {
		t.Fatalf("upsert session: %v", err)
	}
	if _, err := store.PersistUserTurn(ctx, "s-cn", "今天天气很好，我们去散步吧"); err != nil {
		t.Fatalf("persist user turn: %v", err)
	}
	if _, err := store.PersistAssistantTurn(ctx, "s-cn", "好的，我们去公园散步"); err != nil {
		t.Fatalf("persist assistant turn: %v", err)
	}
	if _, err := store.PersistAssistantTurn(ctx, "s-cn", "晚点记得带伞"); err != nil {
		t.Fatalf("persist assistant turn 2: %v", err)
	}

	hits, err := store.SearchMessages(ctx, "散步", 20, 0)
	if err != nil {
		t.Fatalf("search messages: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
	if hits[0].Content != "好的，我们去公园散步" || hits[1].Content != "今天天气很好，我们去散步吧" {
		t.Fatalf("unexpected search order/content: %+v", hits)
	}
}

// 验证空关键词时返回最近消息，用于前端“最近浏览”模式。
func TestSearchMessages_EmptyKeywordReturnsRecent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertSession(ctx, "s-recent", "recent"); err != nil {
		t.Fatalf("upsert session: %v", err)
	}
	if _, err := store.PersistUserTurn(ctx, "s-recent", "第一条"); err != nil {
		t.Fatalf("persist first message: %v", err)
	}
	if _, err := store.PersistAssistantTurn(ctx, "s-recent", "第二条"); err != nil {
		t.Fatalf("persist second message: %v", err)
	}

	hits, err := store.SearchMessages(context.Background(), "   ", 10, 0)
	if err != nil {
		t.Fatalf("search with empty keyword: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 recent results, got %d", len(hits))
	}
	if hits[0].Content != "第二条" || hits[1].Content != "第一条" {
		t.Fatalf("unexpected recent order/content: %+v", hits)
	}
}

// 验证 LIKE 特殊字符会被正确转义，避免误命中过多记录。
func TestSearchMessages_LikeEscaping(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertSession(ctx, "s-esc", "escape"); err != nil {
		t.Fatalf("upsert session: %v", err)
	}
	if _, err := store.PersistUserTurn(ctx, "s-esc", "100% coverage_1"); err != nil {
		t.Fatalf("persist message: %v", err)
	}
	if _, err := store.PersistUserTurn(ctx, "s-esc", "just normal text"); err != nil {
		t.Fatalf("persist message 2: %v", err)
	}

	hits, err := store.SearchMessages(ctx, "% coverage_1", 20, 0)
	if err != nil {
		t.Fatalf("search messages: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].Content != "100% coverage_1" {
		t.Fatalf("unexpected content: %+v", hits[0])
	}
}

// 验证分页参数生效，不同 offset 返回不同结果页。
func TestSearchMessages_Pagination(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertSession(ctx, "s-page", "分页"); err != nil {
		t.Fatalf("upsert session: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := store.PersistAssistantTurn(ctx, "s-page", "关键词 命中"); err != nil {
			t.Fatalf("persist message %d: %v", i, err)
		}
	}

	first, err := store.SearchMessages(ctx, "关键词", 2, 0)
	if err != nil {
		t.Fatalf("search first page: %v", err)
	}
	second, err := store.SearchMessages(ctx, "关键词", 2, 2)
	if err != nil {
		t.Fatalf("search second page: %v", err)
	}

	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("unexpected page sizes: %d, %d", len(first), len(second))
	}
	if first[0].MessageID == second[0].MessageID {
		t.Fatalf("expected different pages, got same message id %d", first[0].MessageID)
	}
}
