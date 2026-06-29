package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"therunish/internal/domain"
)

// ReminderCandidate — подписка и конкретный период, который пора отправить.
type ReminderCandidate struct {
	domain.Subscription
	DaysBefore int
}

// ListActiveSubsByUser — активные подписки юзера (для личного кабинета).
// JOIN с services для получения названия услуги.
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

// HasAnySubscriptionByUser проверяет, была ли у пользователя хоть одна подписка.
func (s *Store) HasAnySubscriptionByUser(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	const q = `SELECT EXISTS (SELECT 1 FROM subscriptions WHERE user_id = $1)`
	if err := s.db.QueryRowContext(ctx, q, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check subscriptions: %w", err)
	}
	return exists, nil
}

// ListSubsByUser — все подписки юзера (любой статус, для админки).
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

// CreateSubscriptionAdmin — ручное создание активной подписки админом (без оплаты).
func (s *Store) CreateSubscriptionAdmin(ctx context.Context, userID, serviceID int64, expiresAt time.Time) error {
	const q = `
		INSERT INTO subscriptions (user_id, service_id, status, started_at, expires_at)
		VALUES ($1, $2, 'active', now(), $3)`
	_, err := s.db.ExecContext(ctx, q, userID, serviceID, expiresAt)
	if err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}
	return nil
}

// DeleteSubscription — удаление подписки админом.
func (s *Store) DeleteSubscription(ctx context.Context, subID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM subscriptions WHERE id = $1`, subID)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	return nil
}

// GetSubscriptionByID — подписка по ID (с названием услуги).
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

// ExtendSubscription — продление подписки на указанное число дней.
// Если подписка просрочена (expires_at < now), считаем от текущего момента.
// Если ещё активна — добавляем дни к текущему сроку.
func (s *Store) ExtendSubscription(ctx context.Context, subID int64, days int) error {
	const q = `
		UPDATE subscriptions
		SET expires_at = CASE
		    WHEN expires_at < now() THEN now() + make_interval(days => $2)
		    ELSE expires_at + make_interval(days => $2)
		END
		WHERE id = $1`
	res, err := s.db.ExecContext(ctx, q, subID, days)
	if err != nil {
		return fmt.Errorf("extend subscription: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateSubscription — обновление полей подписки админом.
func (s *Store) UpdateSubscription(ctx context.Context, subID, serviceID int64, status domain.SubscriptionStatus, expiresAt time.Time) error {
	const q = `
		UPDATE subscriptions
		SET service_id = $1,
		    status     = $2,
		    expires_at = $3
		WHERE id = $4`
	res, err := s.db.ExecContext(ctx, q, serviceID, status, expiresAt, subID)
	if err != nil {
		return fmt.Errorf("update subscription: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListReminderCandidates — кандидаты на напоминание за настроенные периоды.
// Только юзеры с bot_dialog_open=true (бот может писать в личку).
func (s *Store) ListReminderCandidates(ctx context.Context, days []int) ([]ReminderCandidate, error) {
	if len(days) == 0 {
		return []ReminderCandidate{}, nil
	}
	values := make([]string, 0, len(days))
	args := make([]any, 0, len(days))
	for i, day := range days {
		values = append(values, fmt.Sprintf("($%d::int)", i+1))
		args = append(args, day)
	}
	q := `
		WITH reminder_days(days_before) AS (VALUES ` + strings.Join(values, ",") + `)
		SELECT s.id, s.user_id, s.service_id, s.payment_id, s.status, s.started_at, s.expires_at,
		       s.reminded_7d, s.reminded_3d, s.reminded_1d, s.created_at, rd.days_before
		FROM subscriptions s
		JOIN users u ON u.id = s.user_id
		JOIN reminder_days rd ON (s.expires_at::date - CURRENT_DATE) = rd.days_before
		WHERE s.status = 'active'
		  AND u.bot_dialog_open = true
		  AND NOT EXISTS (
		    SELECT 1 FROM subscription_reminder_logs l
		    WHERE l.subscription_id = s.id AND l.days_before = rd.days_before
		  )
		ORDER BY s.expires_at, rd.days_before DESC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ReminderCandidate
	for rows.Next() {
		var item ReminderCandidate
		var payID sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.ServiceID, &payID, &item.Status,
			&item.StartedAt, &item.ExpiresAt,
			&item.Reminded7d, &item.Reminded3d, &item.Reminded1d, &item.CreatedAt,
			&item.DaysBefore,
		); err != nil {
			return nil, fmt.Errorf("scan reminder candidate: %w", err)
		}
		if payID.Valid {
			pid := payID.Int64
			item.PaymentID = &pid
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// MarkReminded фиксирует отправку напоминания.
func (s *Store) MarkReminded(ctx context.Context, subID int64, days int) error {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO subscription_reminder_logs (subscription_id, days_before)
		VALUES ($1, $2)
		ON CONFLICT (subscription_id, days_before) DO NOTHING`, subID, days); err != nil {
		return fmt.Errorf("insert reminder log: %w", err)
	}

	var col string
	switch days {
	case 7:
		col = "reminded_7d"
	case 3:
		col = "reminded_3d"
	case 1:
		col = "reminded_1d"
	default:
		return nil
	}
	q := fmt.Sprintf(`UPDATE subscriptions SET %s = true WHERE id = $1`, col)
	_, err := s.db.ExecContext(ctx, q, subID)
	if err != nil {
		return fmt.Errorf("mark reminded: %w", err)
	}
	return nil
}

// ExpireOverdue — батч-перевод просроченных подписок в expired.
func (s *Store) ExpireOverdue(ctx context.Context) (int64, error) {
	const q = `
		UPDATE subscriptions
		SET status = 'expired'
		WHERE status = 'active' AND expires_at < now()`
	res, err := s.db.ExecContext(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("expire overdue: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
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
