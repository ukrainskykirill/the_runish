package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) CreateLoginRequest(ctx context.Context, nonce string, expiresAt time.Time) error {
	const q = `INSERT INTO login_requests (nonce, expires_at) VALUES ($1, $2)`
	if _, err := s.db.ExecContext(ctx, q, nonce, expiresAt); err != nil {
		return fmt.Errorf("create login request: %w", err)
	}
	return nil
}

func (s *Store) ConfirmLoginRequest(ctx context.Context, nonce string, userID int64) error {
	const q = `
		UPDATE login_requests SET status = 'confirmed', user_id = $2
		WHERE nonce = $1 AND status = 'pending' AND expires_at > now()`
	res, err := s.db.ExecContext(ctx, q, nonce, userID)
	if err != nil {
		return fmt.Errorf("confirm login request: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetLoginRequestStatus(ctx context.Context, nonce string) (status string, userID int64, err error) {
	const q = `
		SELECT status, COALESCE(user_id, 0) FROM login_requests
		WHERE nonce = $1 AND expires_at > now()`
	err = s.db.QueryRowContext(ctx, q, nonce).Scan(&status, &userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, ErrNotFound
	}
	if err != nil {
		return "", 0, fmt.Errorf("get login request status: %w", err)
	}
	return status, userID, nil
}

func (s *Store) DeleteLoginRequest(ctx context.Context, nonce string) error {
	const q = `DELETE FROM login_requests WHERE nonce = $1`
	if _, err := s.db.ExecContext(ctx, q, nonce); err != nil {
		return fmt.Errorf("delete login request: %w", err)
	}
	return nil
}
