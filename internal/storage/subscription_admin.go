package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"therunish/internal/domain"
)

func (s *Store) CreateSubscriptionAdmin(ctx context.Context, userID, serviceID int64, expiresAt time.Time) error {
	const q = `
		INSERT INTO subscriptions (user_id, service_id, status, started_at, expires_at)
		VALUES ($1, $2, 'active', now(), $3)
		RETURNING id`
	var subID int64
	if err := s.db.QueryRowContext(ctx, q, userID, serviceID, expiresAt).Scan(&subID); err != nil {
		return fmt.Errorf("create subscription: %w", err)
	}
	return refreshSubEntitlementTx(ctx, s.db, subID, true)
}

func (s *Store) DeleteSubscription(ctx context.Context, subID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM subscriptions WHERE id = $1`, subID)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	return nil
}

func (s *Store) DeleteSubscriptionWithEntryFeeRevoke(ctx context.Context, subID int64) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		var (
			userID int64
			kind   domain.ServiceKind
		)
		err := tx.QueryRowContext(ctx, `
			SELECT s.user_id, svc.kind
			FROM subscriptions s
			JOIN services svc ON svc.id = s.service_id
			WHERE s.id = $1
			FOR UPDATE`, subID,
		).Scan(&userID, &kind)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("get subscription for delete: %w", err)
		}

		if kind == domain.KindBundle {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET entry_fee_paid = false WHERE id = $1`, userID); err != nil {
				return fmt.Errorf("revoke entry fee on bundle sub delete: %w", err)
			}
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM subscriptions WHERE id = $1`, subID); err != nil {
			return fmt.Errorf("delete subscription: %w", err)
		}
		return nil
	})
}

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
	return refreshSubEntitlementTx(ctx, s.db, subID, false)
}

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
	return refreshSubEntitlementTx(ctx, s.db, subID, false)
}
