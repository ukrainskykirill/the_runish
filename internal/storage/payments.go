package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"therunish/internal/domain"
)

func (s *Store) CreatePayment(ctx context.Context, p *domain.Payment) (int64, error) {
	const q = `
		INSERT INTO payments (user_id, order_id, amount_kop, status, provider, tbank_order_id, payment_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`
	var id int64
	err := s.db.QueryRowContext(ctx, q,
		p.UserID, p.OrderID, p.AmountKop, p.Status, p.Provider, p.TBankOrderID, p.PaymentURL,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create payment: %w", err)
	}
	return id, nil
}

func (s *Store) HasConfirmedPaymentsByUser(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	const q = `SELECT EXISTS (SELECT 1 FROM payments WHERE user_id = $1 AND status = $2)`
	if err := s.db.QueryRowContext(ctx, q, userID, domain.PaymentStatusConfirmed).Scan(&exists); err != nil {
		return false, fmt.Errorf("check confirmed payments: %w", err)
	}
	return exists, nil
}

func (s *Store) UpdatePaymentAfterInit(ctx context.Context, id int64, status domain.PaymentStatus, tbankPaymentID, paymentURL string) error {
	const q = `
		UPDATE payments
		SET status = $1, tbank_payment_id = $2, payment_url = $3, updated_at = now()
		WHERE id = $4`
	_, err := s.db.ExecContext(ctx, q, status, tbankPaymentID, paymentURL, id)
	if err != nil {
		return fmt.Errorf("update payment after init: %w", err)
	}
	return nil
}

func (s *Store) GetPaymentByTBankOrderID(ctx context.Context, tbankOrderID string) (domain.Payment, error) {
	const q = `
		SELECT id, user_id, order_id, amount_kop, status, provider,
		       tbank_payment_id, tbank_order_id, tbank_status, payment_url,
		       error_code, created_at, updated_at, paid_at
		FROM payments WHERE tbank_order_id = $1`
	var p domain.Payment
	var tbankPaymentID, paymentURL, errorCode, tbankStatus sql.NullString
	var paidAt sql.NullTime
	err := s.db.QueryRowContext(ctx, q, tbankOrderID).Scan(
		&p.ID, &p.UserID, &p.OrderID, &p.AmountKop, &p.Status, &p.Provider,
		&tbankPaymentID, &p.TBankOrderID, &tbankStatus, &paymentURL,
		&errorCode, &p.CreatedAt, &p.UpdatedAt, &paidAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Payment{}, ErrNotFound
	}
	if err != nil {
		return domain.Payment{}, fmt.Errorf("get payment by tbank order id: %w", err)
	}
	p.TBankPaymentID = tbankPaymentID.String
	p.PaymentURL = paymentURL.String
	p.ErrorCode = errorCode.String
	p.TBankStatus = tbankStatus.String
	if paidAt.Valid {
		t := paidAt.Time
		p.PaidAt = &t
	}
	return p, nil
}

func (s *Store) ListPendingPaymentsOlderThan(ctx context.Context, cutoff time.Duration) ([]domain.Payment, error) {
	const q = `
		SELECT id, user_id, order_id, amount_kop, status, provider,
		       tbank_payment_id, tbank_order_id, tbank_status, payment_url,
		       error_code, created_at, updated_at, paid_at
		FROM payments
		WHERE status = $1 AND updated_at < $2`
	rows, err := s.db.QueryContext(ctx, q, domain.PaymentStatusPending, time.Now().Add(-cutoff))
	if err != nil {
		return nil, fmt.Errorf("list pending payments: %w", err)
	}
	defer rows.Close()

	var result []domain.Payment
	for rows.Next() {
		var p domain.Payment
		var tbankPaymentID, paymentURL, errorCode, tbankStatus sql.NullString
		var paidAt sql.NullTime
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.OrderID, &p.AmountKop, &p.Status, &p.Provider,
			&tbankPaymentID, &p.TBankOrderID, &tbankStatus, &paymentURL,
			&errorCode, &p.CreatedAt, &p.UpdatedAt, &paidAt,
		); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		p.TBankPaymentID = tbankPaymentID.String
		p.PaymentURL = paymentURL.String
		p.ErrorCode = errorCode.String
		p.TBankStatus = tbankStatus.String
		if paidAt.Valid {
			t := paidAt.Time
			p.PaidAt = &t
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
