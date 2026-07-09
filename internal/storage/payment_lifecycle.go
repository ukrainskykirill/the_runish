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

func (s *Store) CancelStalePendingPayments(ctx context.Context, userID int64) ([]string, error) {
	var tbankIDs []string
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT id, order_id, COALESCE(tbank_payment_id, '')
			FROM payments
			WHERE user_id = $1 AND status IN ($2, $3)
			FOR UPDATE`,
			userID, domain.PaymentStatusNew, domain.PaymentStatusPending,
		)
		if err != nil {
			return fmt.Errorf("select stale pending: %w", err)
		}
		type row struct {
			payID    int64
			orderID  int64
			tbankPID string
		}
		var found []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.payID, &r.orderID, &r.tbankPID); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan stale pending: %w", err)
			}
			found = append(found, r)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan stale pending: %w", err)
		}
		_ = rows.Close()

		for _, r := range found {
			if _, err := tx.ExecContext(ctx,
				`UPDATE payments SET status = $1, updated_at = now() WHERE id = $2`,
				domain.PaymentStatusCancelled, r.payID,
			); err != nil {
				return fmt.Errorf("cancel stale payment: %w", err)
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE orders SET status = $1 WHERE id = $2`,
				domain.OrderStatusCanceled, r.orderID,
			); err != nil {
				return fmt.Errorf("cancel stale order: %w", err)
			}
			if r.tbankPID != "" {
				tbankIDs = append(tbankIDs, r.tbankPID)
			}
		}
		return nil
	})
	return tbankIDs, err
}

func (s *Store) ActivateSubscriptionTx(ctx context.Context, tbankOrderID, tbankPaymentID, tbankStatus string) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		var (
			payID     int64
			curStatus string
			orderID   int64
		)
		err := tx.QueryRowContext(ctx,
			`SELECT id, status, order_id FROM payments WHERE tbank_order_id = $1 FOR UPDATE`,
			tbankOrderID,
		).Scan(&payID, &curStatus, &orderID)
		if err != nil {
			return fmt.Errorf("lock payment: %w", err)
		}
		if curStatus == string(domain.PaymentStatusConfirmed) {
			return nil
		}

		now := time.Now()
		if _, err := tx.ExecContext(ctx, `
			UPDATE payments
			SET status = $1, tbank_payment_id = $2, tbank_status = $3,
			    paid_at = $4, updated_at = $4
			WHERE id = $5`,
			domain.PaymentStatusConfirmed, tbankPaymentID, tbankStatus, now, payID,
		); err != nil {
			return fmt.Errorf("update payment confirmed: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE orders SET status = $1 WHERE id = $2`,
			domain.OrderStatusPaid, orderID,
		); err != nil {
			return fmt.Errorf("update order paid: %w", err)
		}

		rows, err := tx.QueryContext(ctx, `
			SELECT oi.service_id, s.duration_days
			FROM order_items oi
			JOIN services s ON s.id = oi.service_id
			WHERE oi.order_id = $1 AND s.kind IN ('subscription', 'bundle')`,
			orderID)
		if err != nil {
			return fmt.Errorf("select order items for subs: %w", err)
		}
		type subInfo struct {
			serviceID    int64
			durationDays int
		}
		var subs []subInfo
		for rows.Next() {
			var si subInfo
			if err := rows.Scan(&si.serviceID, &si.durationDays); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan sub info: %w", err)
			}
			subs = append(subs, si)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan sub info: %w", err)
		}
		_ = rows.Close()

		var userID int64
		if err := tx.QueryRowContext(ctx, `SELECT user_id FROM orders WHERE id = $1`, orderID).Scan(&userID); err != nil {
			return fmt.Errorf("get order user: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE users SET entry_fee_paid = true
			WHERE id = $1 AND EXISTS (
				SELECT 1 FROM order_items oi
				JOIN services s ON s.id = oi.service_id
				WHERE oi.order_id = $2 AND s.kind IN ('entry', 'bundle')
			)`,
			userID, orderID,
		); err != nil {
			return fmt.Errorf("grant entry fee: %w", err)
		}

		for _, si := range subs {
			if err := activateOrExtendSubTx(ctx, tx, userID, si.serviceID, si.durationDays, &payID, now); err != nil {
				return err
			}
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO training_entitlements (user_id, service_id, payment_id, quota, used, status)
			SELECT $1, oi.service_id, $2, COALESCE(svc.trainings_quota, 1), 0, 'active'
			FROM order_items oi
			JOIN services svc ON svc.id = oi.service_id
			CROSS JOIN generate_series(1, GREATEST(oi.qty, 1)) AS g
			WHERE oi.order_id = $3 AND svc.kind = 'training'`,
			userID, payID, orderID,
		); err != nil {
			return fmt.Errorf("grant training entitlements: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM cart_items WHERE user_id = $1`, userID); err != nil {
			return fmt.Errorf("clear cart on confirm: %w", err)
		}
		return nil
	})
}

func activateOrExtendSubTx(ctx context.Context, tx *sql.Tx, userID, serviceID int64, durationDays int, payID *int64, now time.Time) error {
	var subID int64
	var expires time.Time
	err := tx.QueryRowContext(ctx, `
		SELECT id, expires_at FROM subscriptions
		WHERE user_id = $1 AND service_id = $2 AND status = 'active'
		FOR UPDATE`, userID, serviceID,
	).Scan(&subID, &expires)

	if err == nil {
		newExpires := expires.AddDate(0, 0, durationDays)
		if newExpires.Before(now) {
			newExpires = now.AddDate(0, 0, durationDays)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE subscriptions
			SET expires_at = $1, payment_id = $2,
			    reminded_7d = false, reminded_3d = false, reminded_1d = false
			WHERE id = $3`,
			newExpires, payID, subID,
		); err != nil {
			return fmt.Errorf("extend subscription: %w", err)
		}
		return refreshSubEntitlementTx(ctx, tx, subID, true)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("select active sub: %w", err)
	}

	expires = now.AddDate(0, 0, durationDays)
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO subscriptions (user_id, service_id, payment_id, status, started_at, expires_at)
		VALUES ($1, $2, $3, 'active', $4, $5)
		RETURNING id`,
		userID, serviceID, payID, now, expires,
	).Scan(&subID); err != nil {
		return fmt.Errorf("insert subscription: %w", err)
	}
	return refreshSubEntitlementTx(ctx, tx, subID, true)
}

func refreshSubEntitlementTx(ctx context.Context, q dbExecQuerier, subID int64, resetUsed bool) error {
	var status domain.SubscriptionStatus
	err := q.QueryRowContext(ctx, `SELECT status FROM subscriptions WHERE id = $1`, subID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get subscription status: %w", err)
	}
	if status != domain.SubStatusActive {
		if _, err := q.ExecContext(ctx, `
			UPDATE training_entitlements
			SET status = 'expired', updated_at = now()
			WHERE subscription_id = $1 AND status <> 'expired'`, subID); err != nil {
			return fmt.Errorf("expire sub entitlement: %w", err)
		}
		return nil
	}

	if resetUsed {
		const sql = `
		INSERT INTO training_entitlements (user_id, service_id, subscription_id, payment_id, quota, used, valid_until, status)
		SELECT s.user_id, s.service_id, s.id, s.payment_id, svc.trainings_quota, 0, s.expires_at, 'active'
		FROM subscriptions s
		JOIN services svc ON svc.id = s.service_id
		WHERE s.id = $1
		ON CONFLICT (subscription_id) WHERE subscription_id IS NOT NULL
		DO UPDATE SET quota = EXCLUDED.quota, used = 0, valid_until = EXCLUDED.valid_until,
		              payment_id = EXCLUDED.payment_id, status = 'active', updated_at = now()`
		if _, err := q.ExecContext(ctx, sql, subID); err != nil {
			return fmt.Errorf("refresh sub entitlement: %w", err)
		}
		return nil
	}

	const sql = `
		INSERT INTO training_entitlements (user_id, service_id, subscription_id, payment_id, quota, used, valid_until, status)
		SELECT s.user_id, s.service_id, s.id, s.payment_id, svc.trainings_quota, 0, s.expires_at, 'active'
		FROM subscriptions s
		JOIN services svc ON svc.id = s.service_id
		WHERE s.id = $1
		ON CONFLICT (subscription_id) WHERE subscription_id IS NOT NULL
		DO UPDATE SET quota = EXCLUDED.quota,
		              valid_until = EXCLUDED.valid_until,
		              payment_id = EXCLUDED.payment_id,
		              status = CASE
		                  WHEN EXCLUDED.quota IS NOT NULL AND training_entitlements.used >= EXCLUDED.quota THEN 'exhausted'
		                  ELSE 'active'
		              END,
		              updated_at = now()`
	if _, err := q.ExecContext(ctx, sql, subID); err != nil {
		return fmt.Errorf("refresh sub entitlement: %w", err)
	}
	return nil
}

func (s *Store) RejectPaymentTx(ctx context.Context, tbankOrderID, tbankPaymentID, tbankStatus, errorCode string) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		var payID int64
		var curStatus string
		err := tx.QueryRowContext(ctx,
			`SELECT id, status FROM payments WHERE tbank_order_id = $1 FOR UPDATE`,
			tbankOrderID,
		).Scan(&payID, &curStatus)
		if err != nil {
			return fmt.Errorf("lock payment: %w", err)
		}
		if curStatus == string(domain.PaymentStatusRejected) || curStatus == string(domain.PaymentStatusConfirmed) {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE payments
			SET status = $1, tbank_payment_id = $2, tbank_status = $3,
			    error_code = $4, updated_at = now()
			WHERE id = $5`,
			domain.PaymentStatusRejected, tbankPaymentID, tbankStatus, errorCode, payID,
		); err != nil {
			return fmt.Errorf("update payment rejected: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE orders SET status = $1 WHERE id = (SELECT order_id FROM payments WHERE id = $2)`,
			domain.OrderStatusCanceled, payID,
		); err != nil {
			return fmt.Errorf("cancel order on reject: %w", err)
		}
		return nil
	})
}

func (s *Store) RefundPaymentTx(ctx context.Context, tbankOrderID, tbankPaymentID, tbankStatus string, amountKop int64) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		var (
			payID       int64
			curStatus   string
			totalKop    int64
			refundedKop int64
		)
		err := tx.QueryRowContext(ctx,
			`SELECT id, status, amount_kop, refunded_kop FROM payments WHERE tbank_order_id = $1 FOR UPDATE`,
			tbankOrderID,
		).Scan(&payID, &curStatus, &totalKop, &refundedKop)
		if err != nil {
			return fmt.Errorf("lock payment: %w", err)
		}

		if curStatus == string(domain.PaymentStatusRefunded) {
			return nil
		}

		isPartial := strings.EqualFold(tbankStatus, "PARTIAL_REFUNDED")
		newRefunded := totalKop
		if isPartial {
			newRefunded = refundedKop + amountKop
		}

		if isPartial && newRefunded < totalKop {
			if _, err := tx.ExecContext(ctx,
				`UPDATE payments SET tbank_status = $1, refunded_kop = $2, updated_at = now() WHERE id = $3`,
				tbankStatus, newRefunded, payID,
			); err != nil {
				return fmt.Errorf("update payment partial refund: %w", err)
			}
			return nil
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE payments
			SET status = $1, tbank_payment_id = $2, tbank_status = $3,
			    refunded_kop = $4, updated_at = now()
			WHERE id = $5`,
			domain.PaymentStatusRefunded, tbankPaymentID, tbankStatus, totalKop, payID,
		); err != nil {
			return fmt.Errorf("update payment refunded: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE orders SET status = $1 WHERE id = (SELECT order_id FROM payments WHERE id = $2)`,
			domain.OrderStatusCanceled, payID,
		); err != nil {
			return fmt.Errorf("cancel order on refund: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE subscriptions SET status = $1 WHERE payment_id = $2 AND status = $3`,
			domain.SubStatusCancelled, payID, domain.SubStatusActive,
		); err != nil {
			return fmt.Errorf("cancel subscriptions on refund: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE training_entitlements SET status = 'expired', updated_at = now()
			 WHERE payment_id = $1 AND status <> 'expired'`,
			payID,
		); err != nil {
			return fmt.Errorf("expire training entitlements on refund: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE users SET entry_fee_paid = false
			WHERE id = (SELECT user_id FROM payments WHERE id = $1)
			  AND EXISTS (
				SELECT 1 FROM order_items oi
				JOIN services s ON s.id = oi.service_id
				WHERE oi.order_id = (SELECT order_id FROM payments WHERE id = $1)
				  AND s.kind IN ('entry', 'bundle')
			  )`,
			payID,
		); err != nil {
			return fmt.Errorf("revoke entry fee on refund: %w", err)
		}

		return nil
	})
}
