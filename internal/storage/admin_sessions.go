package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) CreateAdminSession(ctx context.Context, token string, expiresAt time.Time) error {
	const q = `INSERT INTO admin_sessions (token, expires_at) VALUES ($1, $2)`
	_, err := s.db.ExecContext(ctx, q, token, expiresAt)
	if err != nil {
		return fmt.Errorf("create admin session: %w", err)
	}
	return nil
}

func (s *Store) GetAdminSession(ctx context.Context, token string) error {
	const q = `SELECT 1 FROM admin_sessions WHERE token = $1 AND expires_at > now()`
	var tmp int
	err := s.db.QueryRowContext(ctx, q, token).Scan(&tmp)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get admin session: %w", err)
	}
	return nil
}

func (s *Store) DeleteAdminSession(ctx context.Context, token string) error {
	const q = `DELETE FROM admin_sessions WHERE token = $1`
	_, err := s.db.ExecContext(ctx, q, token)
	if err != nil {
		return fmt.Errorf("delete admin session: %w", err)
	}
	return nil
}
