package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CreateSession создаёт новую сессию.
func (s *Store) CreateSession(ctx context.Context, sessionID string, userID int64, expiresAt time.Time) error {
	const q = `
		INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`
	_, err := s.db.ExecContext(ctx, q, sessionID, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// GetSession возвращает userID по sessionID, если сессия не протухла.
func (s *Store) GetSession(ctx context.Context, sessionID string) (userID int64, err error) {
	const q = `
		SELECT user_id FROM sessions
		WHERE id = $1 AND expires_at > now()`
	err = s.db.QueryRowContext(ctx, q, sessionID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get session: %w", err)
	}
	return userID, nil
}

// DeleteSession удаляет сессию (logout).
func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	const q = `DELETE FROM sessions WHERE id = $1`
	_, err := s.db.ExecContext(ctx, q, sessionID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions — очистка протухших сессий (раз в N часов из воркера).
func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	const q = `DELETE FROM sessions WHERE expires_at < now()`
	res, err := s.db.ExecContext(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
