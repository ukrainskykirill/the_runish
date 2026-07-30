package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) CreateAdminSession(ctx context.Context, token, role string, expiresAt time.Time) error {
	const q = `INSERT INTO admin_sessions (token, role, expires_at) VALUES ($1, $2, $3)`
	_, err := s.db.ExecContext(ctx, q, token, role, expiresAt)
	if err != nil {
		return fmt.Errorf("create admin session: %w", err)
	}
	return nil
}

// GetAdminSession возвращает роль живой панельной сессии (admin | coach).
func (s *Store) GetAdminSession(ctx context.Context, token string) (string, error) {
	const q = `SELECT role FROM admin_sessions WHERE token = $1 AND expires_at > now()`
	var role string
	err := s.db.QueryRowContext(ctx, q, token).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get admin session: %w", err)
	}
	return role, nil
}

func (s *Store) DeleteAdminSession(ctx context.Context, token string) error {
	const q = `DELETE FROM admin_sessions WHERE token = $1`
	_, err := s.db.ExecContext(ctx, q, token)
	if err != nil {
		return fmt.Errorf("delete admin session: %w", err)
	}
	return nil
}
