package store

import (
	"context"
	"time"
)

type AuditEvent struct {
	ID           int64     `json:"id"`
	At           time.Time `json:"at"`
	Username     string    `json:"username"`
	UserKind     string    `json:"userKind"`
	ConnectionID string    `json:"connectionId"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	Status       int       `json:"status"`
	UPID         string    `json:"upid,omitempty"`
}

func (s *Store) InsertAudit(ctx context.Context, ev AuditEvent) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO audit_events (at, username, user_kind, connection_id, method, path, status, upid)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, ev.At.UTC(), ev.Username, ev.UserKind, ev.ConnectionID, ev.Method, ev.Path, ev.Status, ev.UPID)
	return err
}

// AuditFilter narrows ListAudit. An empty field matches every event.
type AuditFilter struct {
	ConnectionID string
	Username     string
}

func (s *Store) ListAudit(ctx context.Context, limit int, filter AuditFilter) ([]AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, at, username, user_kind, connection_id, method, path, status, upid
		FROM audit_events
		WHERE ($2 = '' OR connection_id = $2) AND ($3 = '' OR username = $3)
		ORDER BY at DESC
		LIMIT $1
	`, limit, filter.ConnectionID, filter.Username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEvent
	for rows.Next() {
		var ev AuditEvent
		if err := rows.Scan(&ev.ID, &ev.At, &ev.Username, &ev.UserKind, &ev.ConnectionID, &ev.Method, &ev.Path, &ev.Status, &ev.UPID); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}
