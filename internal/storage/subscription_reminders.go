package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"therunish/internal/domain"
)

type ReminderCandidate struct {
	domain.Subscription
	HoursBefore int
}

func (s *Store) ListReminderCandidatesByHours(ctx context.Context, hours []int) ([]ReminderCandidate, error) {
	if len(hours) == 0 {
		return []ReminderCandidate{}, nil
	}
	values := make([]string, 0, len(hours))
	args := make([]any, 0, len(hours))
	for i, h := range hours {
		values = append(values, fmt.Sprintf("($%d::int)", i+1))
		args = append(args, h)
	}
	q := `
		WITH thresholds(hours_before) AS (VALUES ` + strings.Join(values, ",") + `)
		SELECT s.id, s.user_id, s.service_id, s.payment_id, s.status, s.started_at, s.expires_at,
		       s.reminded_7d, s.reminded_3d, s.reminded_1d, s.created_at, th.hours_before
		FROM subscriptions s
		JOIN users u ON u.id = s.user_id
		JOIN thresholds th ON s.expires_at <= now() + make_interval(hours => th.hours_before)
		WHERE s.status = 'active'
		  AND s.expires_at > now()
		  AND u.bot_dialog_open = true
		  AND NOT EXISTS (
		    SELECT 1 FROM subscription_reminder_logs l
		    WHERE l.subscription_id = s.id AND l.hours_before = th.hours_before
		  )
		ORDER BY s.id, th.hours_before ASC`
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
			&item.HoursBefore,
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

func (s *Store) MarkRemindedHours(ctx context.Context, subID int64, hours int) error {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO subscription_reminder_logs (subscription_id, hours_before)
		VALUES ($1, $2)
		ON CONFLICT (subscription_id, hours_before) DO NOTHING`, subID, hours); err != nil {
		return fmt.Errorf("insert reminder log: %w", err)
	}
	return nil
}

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
