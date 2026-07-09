package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"therunish/internal/domain"
)

func (s *Store) ListRecentPayments(ctx context.Context, limit int) ([]PaymentWithDetails, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	const q = `
		SELECT p.id, p.order_id, p.amount_kop, p.status, p.provider,
		       p.tbank_order_id, p.payment_url, p.error_code,
		       p.created_at, p.paid_at,
		       COALESCE(u.full_name, ''), COALESCE(u.username, ''),
		       COALESCE(o.contact_name, ''), COALESCE(o.contact_phone, '')
		FROM payments p
		JOIN orders o ON o.id = p.order_id
		LEFT JOIN users u ON u.id = p.user_id
		ORDER BY p.created_at DESC
		LIMIT $1`
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent payments: %w", err)
	}
	defer rows.Close()

	var result []PaymentWithDetails
	for rows.Next() {
		var p PaymentWithDetails
		var paymentURL, errorCode sql.NullString
		var paidAt sql.NullTime
		if err := rows.Scan(
			&p.ID, &p.OrderID, &p.AmountKop, &p.Status, &p.Provider,
			&p.TBankOrderID, &paymentURL, &errorCode,
			&p.CreatedAt, &paidAt,
			&p.UserName, &p.UserTg,
			&p.ContactName, &p.ContactPhone,
		); err != nil {
			return nil, fmt.Errorf("scan payment details: %w", err)
		}
		p.PaymentURL = paymentURL.String
		p.ErrorCode = errorCode.String
		if paidAt.Valid {
			t := paidAt.Time
			p.PaidAt = &t
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s *Store) ListPaymentsByUser(ctx context.Context, userID int64, limit int) ([]PaymentWithDetails, error) {
	if limit <= 0 || limit > 500 {
		limit = 20
	}
	const q = `
		SELECT p.id, p.order_id, p.amount_kop, p.status, p.provider,
		       p.tbank_order_id, p.payment_url, p.error_code,
		       p.created_at, p.paid_at,
		       COALESCE(u.full_name, ''), COALESCE(u.username, ''),
		       COALESCE(o.contact_name, ''), COALESCE(o.contact_phone, '')
		FROM payments p
		JOIN orders o ON o.id = p.order_id
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.user_id = $1
		ORDER BY p.created_at DESC
		LIMIT $2`
	rows, err := s.db.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list payments by user: %w", err)
	}
	defer rows.Close()

	var result []PaymentWithDetails
	for rows.Next() {
		var p PaymentWithDetails
		var paymentURL, errorCode sql.NullString
		var paidAt sql.NullTime
		if err := rows.Scan(
			&p.ID, &p.OrderID, &p.AmountKop, &p.Status, &p.Provider,
			&p.TBankOrderID, &paymentURL, &errorCode,
			&p.CreatedAt, &paidAt,
			&p.UserName, &p.UserTg,
			&p.ContactName, &p.ContactPhone,
		); err != nil {
			return nil, fmt.Errorf("scan payment details: %w", err)
		}
		p.PaymentURL = paymentURL.String
		p.ErrorCode = errorCode.String
		if paidAt.Valid {
			t := paidAt.Time
			p.PaidAt = &t
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s *Store) GetPaymentDashboard(ctx context.Context) (PaymentDashboard, error) {
	now := time.Now()

	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	weekStart := now.AddDate(0, 0, -6)
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, weekStart.Location())

	monthStats, err := s.getPaymentStats(ctx, monthStart, now)
	if err != nil {
		return PaymentDashboard{}, fmt.Errorf("get month stats: %w", err)
	}
	weekStats, err := s.getPaymentStats(ctx, weekStart, now)
	if err != nil {
		return PaymentDashboard{}, fmt.Errorf("get week stats: %w", err)
	}
	return PaymentDashboard{Month: monthStats, Week: weekStats}, nil
}

func (s *Store) getPaymentStats(ctx context.Context, from, to time.Time) (PaymentPeriodStats, error) {
	const q = `
		SELECT DATE(p.paid_at) AS day, p.user_id,
		       COALESCE(u.full_name, ''), COALESCE(u.username, ''),
		       p.amount_kop
		FROM payments p
		LEFT JOIN users u ON u.id = p.user_id
		WHERE p.status = 'confirmed'
		  AND p.paid_at IS NOT NULL
		  AND p.paid_at >= $1
		  AND p.paid_at < $2
		ORDER BY day DESC`
	rows, err := s.db.QueryContext(ctx, q, from, to)
	if err != nil {
		return PaymentPeriodStats{}, fmt.Errorf("query payment stats: %w", err)
	}
	defer rows.Close()

	dayOrder := []string{}
	dayMap := map[string]*DailyPaymentStat{}
	dayUsers := map[string]map[int64]bool{}
	periodUsers := map[int64]bool{}

	for rows.Next() {
		var day time.Time
		var userID int64
		var fullName, username string
		var amountKop int64
		if err := rows.Scan(&day, &userID, &fullName, &username, &amountKop); err != nil {
			return PaymentPeriodStats{}, fmt.Errorf("scan payment stat: %w", err)
		}

		key := day.Format("2006-01-02")
		stat, ok := dayMap[key]
		if !ok {
			stat = &DailyPaymentStat{Date: day}
			dayMap[key] = stat
			dayOrder = append(dayOrder, key)
			dayUsers[key] = map[int64]bool{}
		}

		stat.TotalKop += amountKop
		stat.PayCount++
		periodUsers[userID] = true

		if !dayUsers[key][userID] {
			dayUsers[key][userID] = true
			stat.UserCount++
			name := fullName
			if name == "" && username != "" {
				name = "@" + username
			}
			if name != "" {
				stat.UserNames = append(stat.UserNames, name)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return PaymentPeriodStats{}, fmt.Errorf("rows err: %w", err)
	}

	var result PaymentPeriodStats
	result.UniqueUser = len(periodUsers)
	for _, key := range dayOrder {
		s := dayMap[key]
		result.TotalKop += s.TotalKop
		result.PayCount += s.PayCount
		if s.TotalKop > result.MaxDailyKop {
			result.MaxDailyKop = s.TotalKop
		}
		result.Daily = append(result.Daily, *s)
	}
	return result, nil
}

func (s *Store) GetPaymentByID(ctx context.Context, id int64) (domain.Payment, error) {
	const q = `
		SELECT id, user_id, order_id, amount_kop, status, provider,
		       tbank_payment_id, tbank_order_id, tbank_status, payment_url,
		       error_code, created_at, updated_at, paid_at
		FROM payments WHERE id = $1`
	var p domain.Payment
	var tbankPaymentID, paymentURL, errorCode, tbankStatus sql.NullString
	var paidAt sql.NullTime
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&p.ID, &p.UserID, &p.OrderID, &p.AmountKop, &p.Status, &p.Provider,
		&tbankPaymentID, &p.TBankOrderID, &tbankStatus, &paymentURL,
		&errorCode, &p.CreatedAt, &p.UpdatedAt, &paidAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Payment{}, ErrNotFound
	}
	if err != nil {
		return domain.Payment{}, fmt.Errorf("get payment by id: %w", err)
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
