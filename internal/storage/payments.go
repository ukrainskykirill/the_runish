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

// CreatePayment вставляет новый платёж (status=new) и возвращает его ID.
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

// HasConfirmedPaymentsByUser проверяет, были ли у пользователя успешные оплаты.
func (s *Store) HasConfirmedPaymentsByUser(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	const q = `SELECT EXISTS (SELECT 1 FROM payments WHERE user_id = $1 AND status = $2)`
	if err := s.db.QueryRowContext(ctx, q, userID, domain.PaymentStatusConfirmed).Scan(&exists); err != nil {
		return false, fmt.Errorf("check confirmed payments: %w", err)
	}
	return exists, nil
}

// UpdatePaymentAfterInit сохраняет PaymentId/URL со статусом после Init.
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

// GetPaymentByTBankOrderID — поиск платежа по OrderId из webhook T-Bank.
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

// ListPendingPaymentsOlderThan — для воркера GetState-подстраховки.
// Возвращает pending-платежи, созданные раньше cutoff.
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

// ActivateSubscriptionTx — идемпотентная транзакция обработки CONFIRMED webhook.
// 1. Блокирует платёж FOR UPDATE, если уже confirmed — выход без изменений.
// 2. Обновляет payments, orders.
// 3. Создаёт/продлевает подписки по позициям-подпискам заказа.
func (s *Store) ActivateSubscriptionTx(ctx context.Context, tbankOrderID, tbankPaymentID, tbankStatus string) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		// Идемпотентность: блокируем и проверяем статус.
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

		// Позиции заказа → создаём/продлеваем подписки для kind ∈ {subscription, bundle}.
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

		// Взнос/бандл → навсегда помечаем вступительный взнос оплаченным.
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
		return nil
	})
}

// activateOrExtendSubTx: если есть active-подписка — продлевает (expires_at += duration),
// сбрасывает reminded_*. Иначе создаёт новую.
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
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("select active sub: %w", err)
	}

	// Создаём новую.
	expires = now.AddDate(0, 0, durationDays)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO subscriptions (user_id, service_id, payment_id, status, started_at, expires_at)
		VALUES ($1, $2, $3, 'active', $4, $5)`,
		userID, serviceID, payID, now, expires,
	); err != nil {
		return fmt.Errorf("insert subscription: %w", err)
	}
	return nil
}

// PaymentWithDetails — платёж с деталями заказа и юзера (для админки).
type PaymentWithDetails struct {
	ID           int64
	OrderID      int64
	AmountKop    int64
	Status       string
	Provider     string
	TBankOrderID string
	PaymentURL   string
	ErrorCode    string
	CreatedAt    time.Time
	PaidAt       *time.Time
	UserName     string
	UserTg       string
	ContactName  string
	ContactPhone string
}

// ListRecentPayments — последние N платежей с деталями (для админки).
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

// ListPaymentsByUser — последние N платежей конкретного юзера (для карточки в админке).
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

// DailyPaymentStat — статистика оплат за один день.
type DailyPaymentStat struct {
	Date      time.Time
	UserCount int
	TotalKop  int64
	PayCount  int
	UserNames []string
}

// PaymentPeriodStats — итоги за период + разбивка по дням.
type PaymentPeriodStats struct {
	TotalKop    int64
	PayCount    int
	UniqueUser  int
	MaxDailyKop int64
	Daily       []DailyPaymentStat
}

// PaymentDashboard — статистика оплат за месяц и неделю.
type PaymentDashboard struct {
	Month PaymentPeriodStats
	Week  PaymentPeriodStats
}

// GetPaymentDashboard возвращает статистику оплат:
//   - за текущий календарный месяц (с 1-го числа);
//   - за последние 7 дней (включая сегодня).
//
// Считаются только confirmed-платежи. Группировка по дате paid_at.
func (s *Store) GetPaymentDashboard(ctx context.Context) (PaymentDashboard, error) {
	now := time.Now()

	// Календарный месяц: с 1-го числа текущего месяца.
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	// Неделя: последние 7 дней (сегодня минус 6 = 7 дней всего).
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

// getPaymentStats собирает статистику оплат за произвольный период.
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

// GetPaymentByID — платёж по внутреннему ID (для админки).
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

// RejectPaymentTx помечает платёж rejected (REJECTED/AUTH_FAIL от T-Bank).
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

		// Отменяем заказ, привязанный к платежу.
		if _, err := tx.ExecContext(ctx,
			`UPDATE orders SET status = $1 WHERE id = (SELECT order_id FROM payments WHERE id = $2)`,
			domain.OrderStatusCanceled, payID,
		); err != nil {
			return fmt.Errorf("cancel order on reject: %w", err)
		}
		return nil
	})
}

// RefundPaymentTx — идемпотентная транзакция обработки REFUNDED webhook.
// 1. Блокирует платёж FOR UPDATE; если уже refunded — выход без изменений.
// 2. Помечает платёж refunded, пишет tbank_status.
// 3. Отменяет заказ.
// 4. Аннулирует активные подписки, выданные по этому платежу.
//
// Для частичного возврата (PARTIAL_REFUNDED) только обновляет tbank_status —
// подписки не трогаются (нужен ручной разбор, какие позиции компенсировать).
func (s *Store) RefundPaymentTx(ctx context.Context, tbankOrderID, tbankPaymentID, tbankStatus string) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		var (
			payID     int64
			curStatus string
		)
		err := tx.QueryRowContext(ctx,
			`SELECT id, status FROM payments WHERE tbank_order_id = $1 FOR UPDATE`,
			tbankOrderID,
		).Scan(&payID, &curStatus)
		if err != nil {
			return fmt.Errorf("lock payment: %w", err)
		}

		// Идемпотентность: уже возвращён — ничего не делаем.
		if curStatus == string(domain.PaymentStatusRefunded) {
			return nil
		}

		// Частичный возврат: только фиксируем tbank_status, подписки не трогаем.
		isPartial := strings.EqualFold(tbankStatus, "PARTIAL_REFUNDED")
		if isPartial {
			if _, err := tx.ExecContext(ctx,
				`UPDATE payments SET tbank_status = $1, updated_at = now() WHERE id = $2`,
				tbankStatus, payID,
			); err != nil {
				return fmt.Errorf("update payment partial refund: %w", err)
			}
			return nil
		}

		// Полный возврат: помечаем платёж, отменяем заказ и подписки.
		if _, err := tx.ExecContext(ctx, `
			UPDATE payments
			SET status = $1, tbank_payment_id = $2, tbank_status = $3, updated_at = now()
			WHERE id = $4`,
			domain.PaymentStatusRefunded, tbankPaymentID, tbankStatus, payID,
		); err != nil {
			return fmt.Errorf("update payment refunded: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE orders SET status = $1 WHERE id = (SELECT order_id FROM payments WHERE id = $2)`,
			domain.OrderStatusCanceled, payID,
		); err != nil {
			return fmt.Errorf("cancel order on refund: %w", err)
		}

		// Аннулируем только активные подписки, выданные по этому платежу.
		if _, err := tx.ExecContext(ctx,
			`UPDATE subscriptions SET status = $1 WHERE payment_id = $2 AND status = $3`,
			domain.SubStatusCancelled, payID, domain.SubStatusActive,
		); err != nil {
			return fmt.Errorf("cancel subscriptions on refund: %w", err)
		}

		// Возврат взноса/бандла → снимаем флаг вступительного взноса (зеркало активации):
		// деньги вернули — членство аннулируем.
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
