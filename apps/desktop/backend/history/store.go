package history

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	defaultPageSize = 20
	maxPageSize     = 200
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

const (
	StatusPending = "pending"
	StatusOK      = "ok"
	StatusError   = "error"
)

//go:embed schema.sql
var schemaFS embed.FS

type Store struct {
	db  *sql.DB
	now func() time.Time
}

type Session struct {
	ID        string
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Message struct {
	ID        int64
	SessionID string
	Role      string
	Content   string
	Status    string
	ErrorCode string
	CreatedAt time.Time
}

func Open(dbPath string) (*Store, error) {
	path := strings.TrimSpace(dbPath)
	if path == "" {
		return nil, errors.New("history db path is required")
	}

	db, err := sql.Open("sqlite3", filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}

	if err := applyPragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db, now: time.Now}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("history store is not initialized")
	}

	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema.sql: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	return nil
}

func (s *Store) UpsertSession(ctx context.Context, sessionID, title string) error {
	if s == nil || s.db == nil {
		return errors.New("history store is not initialized")
	}

	sid := strings.TrimSpace(sessionID)
	if sid == "" {
		return errors.New("session id is required")
	}

	now := s.currentUnix()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sessions (id, title, created_at, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	title = CASE WHEN excluded.title <> '' THEN excluded.title ELSE sessions.title END,
	updated_at = excluded.updated_at;
`, sid, strings.TrimSpace(title), now, now)
	if err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}
	return nil
}

func (s *Store) AppendMessage(ctx context.Context, sessionID, role, content, status, errorCode string) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("history store is not initialized")
	}

	sid := strings.TrimSpace(sessionID)
	if sid == "" {
		return 0, errors.New("session id is required")
	}

	msg := strings.TrimSpace(content)
	if msg == "" {
		return 0, errors.New("content is required")
	}

	if !isValidRole(role) {
		return 0, fmt.Errorf("invalid role: %s", role)
	}
	if !isValidStatus(status) {
		return 0, fmt.Errorf("invalid status: %s", status)
	}

	now := s.currentUnix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `
INSERT INTO sessions (id, title, created_at, updated_at)
VALUES (?, '', ?, ?)
ON CONFLICT(id) DO UPDATE SET updated_at = excluded.updated_at;
`, sid, now, now); err != nil {
		return 0, fmt.Errorf("upsert session in tx: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
INSERT INTO messages (session_id, role, content, status, error_code, created_at)
VALUES (?, ?, ?, ?, ?, ?);
`, sid, role, msg, status, strings.TrimSpace(errorCode), now)
	if err != nil {
		return 0, fmt.Errorf("insert message: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get inserted message id: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return id, nil
}

func (s *Store) ListSessions(ctx context.Context, limit, offset int) ([]Session, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("history store is not initialized")
	}

	l, o := normalizeLimitOffset(limit, offset)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, title, created_at, updated_at
FROM sessions
ORDER BY updated_at DESC, id DESC
LIMIT ? OFFSET ?;
`, l, o)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]Session, 0, l)
	for rows.Next() {
		var item Session
		var createdAt int64
		var updatedAt int64
		if err := rows.Scan(&item.ID, &item.Title, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		item.CreatedAt = time.Unix(createdAt, 0)
		item.UpdatedAt = time.Unix(updatedAt, 0)
		sessions = append(sessions, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	return sessions, nil
}

func (s *Store) GetMessages(ctx context.Context, sessionID string, limit, offset int) ([]Message, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("history store is not initialized")
	}

	sid := strings.TrimSpace(sessionID)
	if sid == "" {
		return nil, errors.New("session id is required")
	}

	l, o := normalizeLimitOffset(limit, offset)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, session_id, role, content, status, error_code, created_at
FROM messages
WHERE session_id = ?
ORDER BY created_at ASC, id ASC
LIMIT ? OFFSET ?;
`, sid, l, o)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer rows.Close()

	messages := make([]Message, 0, l)
	for rows.Next() {
		var item Message
		var createdAt int64
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Role, &item.Content, &item.Status, &item.ErrorCode, &createdAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		item.CreatedAt = time.Unix(createdAt, 0)
		messages = append(messages, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}

	return messages, nil
}

func (s *Store) PersistUserTurn(ctx context.Context, sessionID, content string) (int64, error) {
	return s.AppendMessage(ctx, sessionID, RoleUser, content, StatusOK, "")
}

func (s *Store) PersistAssistantTurn(ctx context.Context, sessionID, content string) (int64, error) {
	return s.AppendMessage(ctx, sessionID, RoleAssistant, content, StatusOK, "")
}

func (s *Store) PersistAssistantError(ctx context.Context, sessionID, errorCode, content string) (int64, error) {
	return s.AppendMessage(ctx, sessionID, RoleAssistant, content, StatusError, errorCode)
}

func (s *Store) LoadRecentSession(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("history store is not initialized")
	}

	sid := strings.TrimSpace(sessionID)
	if sid == "" {
		return nil, errors.New("session id is required")
	}

	l := normalizeLimit(limit)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, session_id, role, content, status, error_code, created_at
FROM messages
WHERE session_id = ?
ORDER BY created_at DESC, id DESC
LIMIT ?;
`, sid, l)
	if err != nil {
		return nil, fmt.Errorf("load recent session: %w", err)
	}
	defer rows.Close()

	recent := make([]Message, 0, l)
	for rows.Next() {
		var item Message
		var createdAt int64
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Role, &item.Content, &item.Status, &item.ErrorCode, &createdAt); err != nil {
			return nil, fmt.Errorf("scan recent message: %w", err)
		}
		item.CreatedAt = time.Unix(createdAt, 0)
		recent = append(recent, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent messages: %w", err)
	}

	for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
		recent[i], recent[j] = recent[j], recent[i]
	}

	return recent, nil
}

func applyPragmas(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return fmt.Errorf("set foreign_keys pragma: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		return fmt.Errorf("set journal_mode pragma: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
		return fmt.Errorf("set busy_timeout pragma: %w", err)
	}
	return nil
}

func isValidRole(role string) bool {
	switch role {
	case RoleUser, RoleAssistant:
		return true
	default:
		return false
	}
}

func isValidStatus(status string) bool {
	switch status {
	case StatusPending, StatusOK, StatusError:
		return true
	default:
		return false
	}
}

func normalizeLimitOffset(limit, offset int) (int, int) {
	return normalizeLimit(limit), normalizeOffset(offset)
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultPageSize
	}
	if limit > maxPageSize {
		return maxPageSize
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func (s *Store) currentUnix() int64 {
	if s != nil && s.now != nil {
		return s.now().Unix()
	}
	return time.Now().Unix()
}
