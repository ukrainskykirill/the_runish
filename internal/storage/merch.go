package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"therunish/internal/domain"
)

// CreateMerch вставляет новый товар мерча и возвращает его ID.
func (s *Store) CreateMerch(ctx context.Context, m *domain.Merch) (int64, error) {
	const q = `
		INSERT INTO merch (title, description, price_kop, sort_order, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`
	var id int64
	err := s.db.QueryRowContext(ctx, q,
		m.Title, m.Description, m.PriceKop, m.SortOrder, m.IsActive,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create merch: %w", err)
	}
	return id, nil
}

// UpdateMerch обновляет товар мерча по ID.
func (s *Store) UpdateMerch(ctx context.Context, m *domain.Merch) error {
	const q = `
		UPDATE merch
		SET title = $1, description = $2, price_kop = $3, sort_order = $4, is_active = $5, updated_at = now()
		WHERE id = $6`
	res, err := s.db.ExecContext(ctx, q,
		m.Title, m.Description, m.PriceKop, m.SortOrder, m.IsActive, m.ID,
	)
	if err != nil {
		return fmt.Errorf("update merch: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeactivateMerch — мягкое удаление (is_active = false).
func (s *Store) DeactivateMerch(ctx context.Context, id int64) error {
	const q = `UPDATE merch SET is_active = false, updated_at = now() WHERE id = $1`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("deactivate merch: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetMerch возвращает товар мерча по ID (включая неактивные).
func (s *Store) GetMerch(ctx context.Context, id int64) (domain.Merch, error) {
	const q = `
		SELECT id, title, description, price_kop, sort_order, is_active, created_at, updated_at
		FROM merch WHERE id = $1`
	var m domain.Merch
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&m.ID, &m.Title, &m.Description, &m.PriceKop, &m.SortOrder,
		&m.IsActive, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Merch{}, ErrNotFound
	}
	if err != nil {
		return domain.Merch{}, fmt.Errorf("get merch: %w", err)
	}
	return m, nil
}

// ListMerch возвращает товары мерча. Если includeInactive=false — только активные.
func (s *Store) ListMerch(ctx context.Context, includeInactive bool) ([]domain.Merch, error) {
	q := `
		SELECT id, title, description, price_kop, sort_order, is_active, created_at, updated_at
		FROM merch`
	if !includeInactive {
		q += ` WHERE is_active = true`
	}
	q += ` ORDER BY sort_order, id`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list merch: %w", err)
	}
	defer rows.Close()

	var items []domain.Merch
	for rows.Next() {
		var m domain.Merch
		if err := rows.Scan(
			&m.ID, &m.Title, &m.Description, &m.PriceKop, &m.SortOrder,
			&m.IsActive, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan merch: %w", err)
		}
		items = append(items, m)
	}
	return items, rows.Err()
}
