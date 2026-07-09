package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"therunish/internal/domain"
)

func (s *Store) ListActiveSubsByUser(ctx context.Context, userID int64) ([]domain.Subscription, error) {
	const q = `
		SELECT s.id, s.user_id, s.service_id, svc.title, s.payment_id, s.status,
		       s.started_at, s.expires_at, s.reminded_7d, s.reminded_3d, s.reminded_1d, s.created_at
		FROM subscriptions s
		JOIN services svc ON svc.id = s.service_id
		WHERE s.user_id = $1 AND s.status = 'active'
		ORDER BY s.expires_at`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		var sub domain.Subscription
		var payID sql.NullInt64
		if err := rows.Scan(
			&sub.ID, &sub.UserID, &sub.ServiceID, &sub.ServiceTitle, &payID, &sub.Status,
			&sub.StartedAt, &sub.ExpiresAt,
			&sub.Reminded7d, &sub.Reminded3d, &sub.Reminded1d, &sub.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		if payID.Valid {
			pid := payID.Int64
			sub.PaymentID = &pid
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

func (s *Store) HasAnySubscriptionByUser(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	const q = `SELECT EXISTS (SELECT 1 FROM subscriptions WHERE user_id = $1)`
	if err := s.db.QueryRowContext(ctx, q, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check subscriptions: %w", err)
	}
	return exists, nil
}

func (s *Store) ListSubsByUser(ctx context.Context, userID int64) ([]domain.Subscription, error) {
	const q = `
		SELECT s.id, s.user_id, s.service_id, svc.title, s.payment_id, s.status,
		       s.started_at, s.expires_at, s.reminded_7d, s.reminded_3d, s.reminded_1d, s.created_at
		FROM subscriptions s
		JOIN services svc ON svc.id = s.service_id
		WHERE s.user_id = $1
		ORDER BY s.created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		var sub domain.Subscription
		var payID sql.NullInt64
		if err := rows.Scan(
			&sub.ID, &sub.UserID, &sub.ServiceID, &sub.ServiceTitle, &payID, &sub.Status,
			&sub.StartedAt, &sub.ExpiresAt,
			&sub.Reminded7d, &sub.Reminded3d, &sub.Reminded1d, &sub.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		if payID.Valid {
			pid := payID.Int64
			sub.PaymentID = &pid
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

func (s *Store) GetSubscriptionByID(ctx context.Context, subID int64) (domain.Subscription, error) {
	const q = `
		SELECT s.id, s.user_id, s.service_id, svc.title, s.payment_id, s.status,
		       s.started_at, s.expires_at, s.reminded_7d, s.reminded_3d, s.reminded_1d, s.created_at
		FROM subscriptions s
		JOIN services svc ON svc.id = s.service_id
		WHERE s.id = $1`
	var sub domain.Subscription
	var payID sql.NullInt64
	err := s.db.QueryRowContext(ctx, q, subID).Scan(
		&sub.ID, &sub.UserID, &sub.ServiceID, &sub.ServiceTitle, &payID, &sub.Status,
		&sub.StartedAt, &sub.ExpiresAt,
		&sub.Reminded7d, &sub.Reminded3d, &sub.Reminded1d, &sub.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Subscription{}, ErrNotFound
	}
	if err != nil {
		return domain.Subscription{}, fmt.Errorf("get subscription by id: %w", err)
	}
	if payID.Valid {
		pid := payID.Int64
		sub.PaymentID = &pid
	}
	return sub, nil
}

func (s *Store) scanSubs(rows *sql.Rows, err error) ([]domain.Subscription, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		var sub domain.Subscription
		var payID sql.NullInt64
		if err := rows.Scan(
			&sub.ID, &sub.UserID, &sub.ServiceID, &payID, &sub.Status,
			&sub.StartedAt, &sub.ExpiresAt,
			&sub.Reminded7d, &sub.Reminded3d, &sub.Reminded1d, &sub.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		if payID.Valid {
			pid := payID.Int64
			sub.PaymentID = &pid
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}
