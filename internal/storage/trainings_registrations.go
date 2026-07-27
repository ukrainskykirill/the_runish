package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"therunish/internal/domain"
)

var (
	ErrNoEntryFee        = domain.ErrTrainingNoEntryFee
	ErrNoAccess          = domain.ErrTrainingNoAccess
	ErrSessionFull       = domain.ErrTrainingSessionFull
	ErrAlreadyRegistered = domain.ErrTrainingAlreadyRegistered
	ErrSessionCancelled  = domain.ErrTrainingSessionCancelled
	ErrPastSession       = domain.ErrTrainingPastSession
)

func clubLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return time.FixedZone("MSK", 3*60*60)
	}
	return loc
}

func isoWeekday(t time.Time) int {
	wd := int(t.Weekday())
	if wd == 0 {
		return 7
	}
	return wd
}

func occurrenceStart(date time.Time, startHHMM string, loc *time.Location) (time.Time, error) {
	parts := strings.SplitN(startHHMM, ":", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("bad time %q", startHHMM)
	}
	hh, err1 := strconv.Atoi(parts[0])
	mm, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return time.Time{}, fmt.Errorf("bad time %q", startHHMM)
	}
	return time.Date(date.Year(), date.Month(), date.Day(), hh, mm, 0, 0, loc), nil
}

func (s *Store) RegisterForTrainingTx(ctx context.Context, userID, trainingID int64, date time.Time) (domain.TrainingRegistration, error) {
	loc := clubLocation()
	now := time.Now().In(loc)
	var reg domain.TrainingRegistration

	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		var entryFeePaid bool
		if err := tx.QueryRowContext(ctx, `SELECT entry_fee_paid FROM users WHERE id = $1`, userID).Scan(&entryFeePaid); err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		var (
			title, place, startHHMM, kind, trainingDate string
			capacity                                    sql.NullInt64
			isActive                                    bool
		)
		err := tx.QueryRowContext(ctx, `
			SELECT title, place, to_char(start_time,'HH24:MI'), kind, to_char(training_date,'YYYY-MM-DD'), capacity, is_active
			FROM trainings WHERE id = $1`, trainingID,
		).Scan(&title, &place, &startHHMM, &kind, &trainingDate, &capacity, &isActive)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("get training: %w", err)
		}
		if !isActive {
			return ErrNotFound
		}

		dateStr := date.Format("2006-01-02")
		if dateStr != trainingDate {
			return ErrPastSession
		}
		start, err := occurrenceStart(date, startHHMM, loc)
		if err != nil {
			return err
		}
		if !start.After(now) {
			return ErrPastSession
		}

		// Определяем доступ и (для regular с подпиской) энтайтлмент.
		var entID sql.NullInt64
		if kind != "sunday_runish" {
			var (
				id       int64
				entQuota sql.NullInt64
				entUsed  int
			)
			err = tx.QueryRowContext(ctx, `
				SELECT e.id, e.quota, e.used
				FROM training_entitlements e
				WHERE e.user_id = $1 AND e.status = 'active'
				  AND (e.valid_until IS NULL OR e.valid_until > now())
				  AND (e.quota IS NULL OR e.used < e.quota)
				  AND (NOT EXISTS (SELECT 1 FROM service_trainings st WHERE st.service_id = e.service_id)
				       OR EXISTS (SELECT 1 FROM service_trainings st WHERE st.service_id = e.service_id AND st.training_id = $2))
				ORDER BY (e.quota IS NULL) DESC, e.valid_until ASC NULLS LAST
				FOR UPDATE
				LIMIT 1`, userID, trainingID,
			).Scan(&id, &entQuota, &entUsed)
			switch {
			case err == nil:
				if !entryFeePaid {
					return ErrNoEntryFee
				}
				entID = sql.NullInt64{Int64: id, Valid: true}
			case errors.Is(err, sql.ErrNoRows):
				// Нет подписки — путь «первая бесплатная» для новичка.
				ok, ferr := canBookFreeFirstTx(ctx, tx, userID, entryFeePaid)
				if ferr != nil {
					return ferr
				}
				if !ok {
					return ErrNoAccess
				}
			default:
				return fmt.Errorf("find entitlement: %w", err)
			}
		}

		var sessionID int64
		var sessionStatus string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO training_sessions (training_id, session_date)
			VALUES ($1, $2)
			ON CONFLICT (training_id, session_date) DO UPDATE SET session_date = EXCLUDED.session_date
			RETURNING id, status`, trainingID, dateStr,
		).Scan(&sessionID, &sessionStatus); err != nil {
			return fmt.Errorf("get-or-create session: %w", err)
		}
		if sessionStatus == "cancelled" {
			return ErrSessionCancelled
		}

		var dummy int
		err = tx.QueryRowContext(ctx,
			`SELECT 1 FROM training_registrations WHERE user_id = $1 AND session_id = $2 AND status = 'active'`,
			userID, sessionID,
		).Scan(&dummy)
		if err == nil {
			return ErrAlreadyRegistered
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("check existing reg: %w", err)
		}

		if capacity.Valid {
			var cnt int64
			if err := tx.QueryRowContext(ctx,
				`SELECT count(*) FROM training_registrations WHERE session_id = $1 AND status = 'active'`,
				sessionID,
			).Scan(&cnt); err != nil {
				return fmt.Errorf("count regs: %w", err)
			}
			if cnt >= capacity.Int64 {
				return ErrSessionFull
			}
		}

		var regID int64
		var createdAt time.Time
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO training_registrations (user_id, session_id, entitlement_id)
			VALUES ($1, $2, $3)
			RETURNING id, created_at`, userID, sessionID, entID,
		).Scan(&regID, &createdAt); err != nil {
			return fmt.Errorf("insert registration: %w", err)
		}
		if entID.Valid {
			if _, err := tx.ExecContext(ctx, `
				UPDATE training_entitlements
				SET used = used + 1,
				    status = CASE WHEN quota IS NOT NULL AND used + 1 >= quota THEN 'exhausted' ELSE 'active' END,
				    updated_at = now()
				WHERE id = $1`, entID.Int64,
			); err != nil {
				return fmt.Errorf("spend entitlement: %w", err)
			}
		}

		reg = domain.TrainingRegistration{
			ID:          regID,
			TrainingID:  trainingID,
			Title:       title,
			SessionDate: date.Format("2006-01-02"),
			StartTime:   startHHMM,
			Place:       place,
			CreatedAt:   createdAt,
		}
		return nil
	})
	return reg, err
}

func (s *Store) CancelRegistrationTx(ctx context.Context, userID, regID int64) error {
	loc := clubLocation()
	now := time.Now().In(loc)
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		var (
			entID       sql.NullInt64
			status      string
			sessionDate time.Time
			startHHMM   string
		)
		err := tx.QueryRowContext(ctx, `
			SELECT r.entitlement_id, r.status, ts.session_date, to_char(t.start_time,'HH24:MI')
			FROM training_registrations r
			JOIN training_sessions ts ON ts.id = r.session_id
			JOIN trainings t ON t.id = ts.training_id
			WHERE r.id = $1 AND r.user_id = $2
			FOR UPDATE OF r`, regID, userID,
		).Scan(&entID, &status, &sessionDate, &startHHMM)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("get registration: %w", err)
		}
		if status != "active" {
			return ErrNotFound
		}

		start, err := occurrenceStart(sessionDate, startHHMM, loc)
		if err != nil {
			return err
		}
		if !start.After(now) {
			return ErrPastSession
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE training_registrations SET status = 'cancelled', cancelled_at = now() WHERE id = $1`, regID,
		); err != nil {
			return fmt.Errorf("cancel registration: %w", err)
		}
		if entID.Valid {
			if _, err := tx.ExecContext(ctx, `
				UPDATE training_entitlements
				SET used = GREATEST(used - 1, 0),
				    status = CASE
				        WHEN status = 'expired' THEN 'expired'
				        WHEN valid_until IS NOT NULL AND valid_until <= now() THEN 'expired'
				        ELSE 'active'
				    END,
				    updated_at = now()
				WHERE id = $1`, entID.Int64,
			); err != nil {
				return fmt.Errorf("refund entitlement: %w", err)
			}
		}
		return nil
	})
}

// canBookFreeFirstTx — новичок без взноса, без подписок/платежей и без единой прошлой
// записи может один раз записаться на regular-тренировку бесплатно.
func canBookFreeFirstTx(ctx context.Context, tx *sql.Tx, userID int64, entryFeePaid bool) (bool, error) {
	if entryFeePaid {
		return false, nil
	}
	var hasPayments, hasSubs, hasRegs bool
	if err := tx.QueryRowContext(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM payments WHERE user_id = $1 AND status = 'confirmed'),
			EXISTS (SELECT 1 FROM subscriptions WHERE user_id = $1),
			EXISTS (SELECT 1 FROM training_registrations WHERE user_id = $1)`,
		userID,
	).Scan(&hasPayments, &hasSubs, &hasRegs); err != nil {
		return false, fmt.Errorf("check free-first eligibility: %w", err)
	}
	return !hasPayments && !hasSubs && !hasRegs, nil
}

func (s *Store) HasAnyTrainingRegistrationByUser(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM training_registrations WHERE user_id = $1)`, userID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("has training registration: %w", err)
	}
	return exists, nil
}
