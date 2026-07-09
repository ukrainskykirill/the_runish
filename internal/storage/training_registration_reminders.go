package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type TrainingReminderCandidate struct {
	RegID       int64
	UserID      int64
	TelegramID  int64
	Title       string
	Place       string
	StartTime   string
	SessionDate string
	StartAt     time.Time
}

func (s *Store) ListTrainingReminderCandidates(ctx context.Context, leadHours int) ([]TrainingReminderCandidate, error) {
	const q = `
		SELECT r.id, r.user_id, u.telegram_id, t.title, t.place,
		       to_char(t.start_time,'HH24:MI'), to_char(ts.session_date,'YYYY-MM-DD'),
		       (ts.session_date + t.start_time) AT TIME ZONE 'Europe/Moscow'
		FROM training_registrations r
		JOIN training_sessions ts ON ts.id = r.session_id
		JOIN trainings t ON t.id = ts.training_id
		JOIN users u ON u.id = r.user_id
		WHERE r.status = 'active' AND r.reminder_sent_at IS NULL
		  AND ts.status = 'scheduled' AND u.bot_dialog_open = true
		  AND (ts.session_date + t.start_time) AT TIME ZONE 'Europe/Moscow' > now()
		  AND (ts.session_date + t.start_time) AT TIME ZONE 'Europe/Moscow' < now() + make_interval(hours => $1)
		ORDER BY (ts.session_date + t.start_time) AT TIME ZONE 'Europe/Moscow'`
	rows, err := s.db.QueryContext(ctx, q, leadHours)
	if err != nil {
		return nil, fmt.Errorf("list training reminder candidates: %w", err)
	}
	defer rows.Close()

	var result []TrainingReminderCandidate
	for rows.Next() {
		var c TrainingReminderCandidate
		if err := rows.Scan(&c.RegID, &c.UserID, &c.TelegramID, &c.Title, &c.Place,
			&c.StartTime, &c.SessionDate, &c.StartAt); err != nil {
			return nil, fmt.Errorf("scan reminder candidate: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (s *Store) MarkTrainingReminded(ctx context.Context, regID int64) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE training_registrations SET reminder_sent_at = now() WHERE id = $1`, regID,
	); err != nil {
		return fmt.Errorf("mark training reminded: %w", err)
	}
	return nil
}

func (s *Store) CancelSessionAdmin(ctx context.Context, trainingID int64, date time.Time) ([]int64, error) {
	var tgIDs []int64
	dateStr := date.Format("2006-01-02")
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		var sessionID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO training_sessions (training_id, session_date, status)
			VALUES ($1, $2, 'cancelled')
			ON CONFLICT (training_id, session_date) DO UPDATE SET status = 'cancelled'
			RETURNING id`, trainingID, dateStr,
		).Scan(&sessionID)
		if err != nil {
			return fmt.Errorf("cancel session: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE training_entitlements e
			SET used = GREATEST(e.used - 1, 0), status = 'active', updated_at = now()
			FROM training_registrations r
			WHERE r.session_id = $1 AND r.status = 'active' AND r.entitlement_id = e.id`, sessionID,
		); err != nil {
			return fmt.Errorf("refund entitlements on session cancel: %w", err)
		}

		rows, err := tx.QueryContext(ctx, `
			SELECT u.telegram_id FROM training_registrations r
			JOIN users u ON u.id = r.user_id
			WHERE r.session_id = $1 AND r.status = 'active' AND u.bot_dialog_open = true`, sessionID)
		if err != nil {
			return fmt.Errorf("list affected users: %w", err)
		}
		for rows.Next() {
			var tg int64
			if err := rows.Scan(&tg); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan tg id: %w", err)
			}
			tgIDs = append(tgIDs, tg)
		}
		_ = rows.Close()

		if _, err := tx.ExecContext(ctx,
			`UPDATE training_registrations SET status = 'cancelled', cancelled_at = now()
			 WHERE session_id = $1 AND status = 'active'`, sessionID,
		); err != nil {
			return fmt.Errorf("cancel registrations: %w", err)
		}
		return nil
	})
	return tgIDs, err
}
