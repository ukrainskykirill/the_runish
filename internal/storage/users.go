package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"therunish/internal/domain"
)

// userCols — общий список колонок юзера (порядок совпадает со scanUser).
const userCols = `id, telegram_id, username, full_name, phone, bot_dialog_open, is_admin, entry_fee_paid, created_at`

func scanUser(sc rowScanner, u *domain.User) error {
	return sc.Scan(
		&u.ID, &u.TelegramID, &u.Username, &u.FullName, &u.Phone,
		&u.BotDialogOpen, &u.IsAdmin, &u.EntryFeePaid, &u.CreatedAt,
	)
}

// UpsertUser создаёт или обновляет юзера по telegram_id и возвращает актуального юзера.
// Используется при логине через Telegram (Login Widget и deep-link).
//
// Побочный эффект: при первой регистрации (строка именно вставлена) в той же транзакции
// заводится pending-запись онбординг-анкеты. Признак вставки берётся из служебного xmax
// (у только что вставленной версии строки он равен 0), поэтому существующим юзерам анкета
// не создаётся — она достаётся только новым.
func (s *Store) UpsertUser(ctx context.Context, u *domain.User) (domain.User, error) {
	const q = `
		WITH up AS (
			INSERT INTO users (telegram_id, username, full_name, phone)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (telegram_id) DO UPDATE
				SET username  = EXCLUDED.username,
				    full_name = EXCLUDED.full_name,
				    -- Пустую строку от бота (/start без телефона) не пишем,
				    -- чтобы не затереть уже сохранённый номер.
				    phone     = COALESCE(NULLIF(EXCLUDED.phone, ''), users.phone)
			RETURNING ` + userCols + `, (xmax = 0) AS inserted
		), seed_survey AS (
			INSERT INTO survey_responses (user_id, status)
			SELECT id, 'pending' FROM up WHERE inserted
			ON CONFLICT (user_id) DO NOTHING
		)
		SELECT ` + userCols + ` FROM up`
	var result domain.User
	if err := scanUser(s.db.QueryRowContext(ctx, q, u.TelegramID, u.Username, u.FullName, u.Phone), &result); err != nil {
		return domain.User{}, fmt.Errorf("upsert user: %w", err)
	}
	return result, nil
}

// GetUserByID возвращает юзера по ID.
func (s *Store) GetUserByID(ctx context.Context, id int64) (domain.User, error) {
	q := `SELECT ` + userCols + ` FROM users WHERE id = $1`
	var u domain.User
	err := scanUser(s.db.QueryRowContext(ctx, q, id), &u)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// GetUserByTelegramID возвращает юзера по telegram_id.
func (s *Store) GetUserByTelegramID(ctx context.Context, tgID int64) (domain.User, error) {
	q := `SELECT ` + userCols + ` FROM users WHERE telegram_id = $1`
	var u domain.User
	err := scanUser(s.db.QueryRowContext(ctx, q, tgID), &u)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user by telegram id: %w", err)
	}
	return u, nil
}

// SetEntryFeePaid выставляет флаг вступительного взноса (ручное управление из админки).
func (s *Store) SetEntryFeePaid(ctx context.Context, userID int64, paid bool) error {
	const q = `UPDATE users SET entry_fee_paid = $2 WHERE id = $1`
	if _, err := s.db.ExecContext(ctx, q, userID, paid); err != nil {
		return fmt.Errorf("set entry_fee_paid: %w", err)
	}
	return nil
}

// CountEntryFeePaid — число юзеров, оплативших взнос (для льготы «первых 30»).
func (s *Store) CountEntryFeePaid(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE entry_fee_paid`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count entry_fee_paid: %w", err)
	}
	return n, nil
}

// UserListItem — юзер + краткая информация о его ближайшей активной подписке (для админки).
type UserListItem struct {
	domain.User
	SubscriptionTitle     string
	SubscriptionExpiresAt *time.Time
}

// ListUsers — список юзеров для админки. С непустым search фильтрует
// по username/full_name (ILIKE) или telegram_id (точное совпадение).
// Для каждого юзера дополнительно отдаёт его ближайшую по сроку активную подписку (если есть).
func (s *Store) ListUsers(ctx context.Context, search string) ([]UserListItem, error) {
	const q = `
		SELECT u.id, u.telegram_id, u.username, u.full_name, u.phone, u.bot_dialog_open, u.is_admin, u.entry_fee_paid, u.created_at,
		       sub.title, sub.expires_at
		FROM users u
		LEFT JOIN (
			SELECT DISTINCT ON (s.user_id) s.user_id, svc.title, s.expires_at
			FROM subscriptions s
			JOIN services svc ON svc.id = s.service_id
			WHERE s.status = 'active'
			ORDER BY s.user_id, s.expires_at ASC
		) sub ON sub.user_id = u.id
		WHERE $1 = '' OR u.username ILIKE '%' || $1 || '%' OR u.full_name ILIKE '%' || $1 || '%'
		   OR u.telegram_id::text = $1
		ORDER BY u.created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, search)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []UserListItem
	for rows.Next() {
		var u UserListItem
		var subTitle sql.NullString
		var subExpiresAt sql.NullTime
		if err := rows.Scan(
			&u.ID, &u.TelegramID, &u.Username, &u.FullName,
			&u.Phone, &u.BotDialogOpen, &u.IsAdmin, &u.EntryFeePaid, &u.CreatedAt,
			&subTitle, &subExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		if subTitle.Valid {
			u.SubscriptionTitle = subTitle.String
		}
		if subExpiresAt.Valid {
			t := subExpiresAt.Time
			u.SubscriptionExpiresAt = &t
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ListNotificationUsers возвращает пользователей, которым можно писать в Telegram.
// audience: "all" — все открывшие диалог с ботом; "active" — только с активной подпиской.
func (s *Store) ListNotificationUsers(ctx context.Context, audience string) ([]domain.User, error) {
	q := `SELECT ` + userCols + ` FROM users u WHERE u.bot_dialog_open = true`
	if audience == "active" {
		q += ` AND EXISTS (
			SELECT 1 FROM subscriptions s
			WHERE s.user_id = u.id AND s.status = 'active'
		)`
	}
	q += ` ORDER BY u.created_at DESC`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list notification users: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		if err := scanUser(rows, &u); err != nil {
			return nil, fmt.Errorf("scan notification user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetNotificationUser возвращает конкретного пользователя, если бот может ему писать.
func (s *Store) GetNotificationUser(ctx context.Context, userID int64) (domain.User, error) {
	q := `SELECT ` + userCols + ` FROM users WHERE id = $1 AND bot_dialog_open = true`
	var u domain.User
	err := scanUser(s.db.QueryRowContext(ctx, q, userID), &u)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get notification user: %w", err)
	}
	return u, nil
}

// SetUserPhone сохраняет нормализованный телефон юзера.
func (s *Store) SetUserPhone(ctx context.Context, userID int64, phone string) error {
	const q = `UPDATE users SET phone = $2 WHERE id = $1`
	if _, err := s.db.ExecContext(ctx, q, userID, phone); err != nil {
		return fmt.Errorf("set user phone: %w", err)
	}
	return nil
}

// SetBotDialogOpen выставляет флаг, что юзер нажал /start у бота.
func (s *Store) SetBotDialogOpen(ctx context.Context, userID int64, open bool) error {
	const q = `UPDATE users SET bot_dialog_open = $1 WHERE id = $2`
	_, err := s.db.ExecContext(ctx, q, open, userID)
	if err != nil {
		return fmt.Errorf("set bot_dialog_open: %w", err)
	}
	return nil
}
