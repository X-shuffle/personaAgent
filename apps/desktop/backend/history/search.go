package history

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SearchHit struct {
	MessageID    int64
	SessionID    string
	SessionTitle string
	Role         string
	Content      string
	Status       string
	ErrorCode    string
	CreatedAt    time.Time
}

func (s *Store) SearchMessages(ctx context.Context, keyword string, limit, offset int) ([]SearchHit, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("history store is not initialized")
	}

	kw := strings.TrimSpace(keyword)
	if kw == "" {
		return []SearchHit{}, nil
	}

	l, o := normalizeLimitOffset(limit, offset)
	likeArg := "%" + escapeLikePattern(kw) + "%"

	rows, err := s.db.QueryContext(ctx, `
SELECT
    m.id,
    m.session_id,
    s.title,
    m.role,
    m.content,
    m.status,
    m.error_code,
    m.created_at
FROM messages m
JOIN sessions s ON s.id = m.session_id
WHERE (m.content LIKE ? ESCAPE '/' OR instr(m.content, ?) > 0)
ORDER BY m.created_at DESC, m.id DESC
LIMIT ? OFFSET ?;
`, likeArg, kw, l, o)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	hits := make([]SearchHit, 0, l)
	for rows.Next() {
		var item SearchHit
		var createdAt int64
		if err := rows.Scan(
			&item.MessageID,
			&item.SessionID,
			&item.SessionTitle,
			&item.Role,
			&item.Content,
			&item.Status,
			&item.ErrorCode,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan search hit: %w", err)
		}
		item.CreatedAt = time.Unix(createdAt, 0)
		hits = append(hits, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search hits: %w", err)
	}

	return hits, nil
}

func escapeLikePattern(input string) string {
	replacer := strings.NewReplacer(
		`/`, `//`,
		`%`, `/%`,
		`_`, `/_`,
	)
	return replacer.Replace(input)
}
