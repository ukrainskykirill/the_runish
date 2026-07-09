package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"therunish/internal/domain"
)

func (s *Store) CreateNews(ctx context.Context, n *domain.News) (int64, error) {
	const q = `
		INSERT INTO news (title, content, sort_order, is_active, published_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`
	var id int64
	err := s.db.QueryRowContext(ctx, q,
		n.Title, n.Content, n.SortOrder, n.IsActive, n.PublishedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create news: %w", err)
	}
	return id, nil
}

func (s *Store) UpdateNews(ctx context.Context, n *domain.News) error {
	const q = `
		UPDATE news
		SET title = $1, content = $2, sort_order = $3, is_active = $4, published_at = $5, updated_at = now()
		WHERE id = $6`
	res, err := s.db.ExecContext(ctx, q,
		n.Title, n.Content, n.SortOrder, n.IsActive, n.PublishedAt, n.ID,
	)
	if err != nil {
		return fmt.Errorf("update news: %w", err)
	}
	n2, _ := res.RowsAffected()
	if n2 == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeactivateNews(ctx context.Context, id int64) error {
	const q = `UPDATE news SET is_active = false, updated_at = now() WHERE id = $1`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("deactivate news: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetNews(ctx context.Context, id int64) (domain.News, error) {
	const q = `
		SELECT id, title, content, sort_order, is_active, published_at, created_at, updated_at
		FROM news WHERE id = $1`
	var n domain.News
	var publishedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&n.ID, &n.Title, &n.Content, &n.SortOrder, &n.IsActive,
		&publishedAt, &n.CreatedAt, &n.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.News{}, ErrNotFound
	}
	if err != nil {
		return domain.News{}, fmt.Errorf("get news: %w", err)
	}
	if publishedAt.Valid {
		t := publishedAt.Time
		n.PublishedAt = &t
	}
	return n, nil
}

func (s *Store) ListNews(ctx context.Context, includeInactive bool) ([]domain.News, error) {
	q := `
		SELECT id, title, content, sort_order, is_active, published_at, created_at, updated_at
		FROM news`
	if !includeInactive {
		q += ` WHERE is_active = true`
	}
	q += ` ORDER BY sort_order, id`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list news: %w", err)
	}
	defer rows.Close()

	var news []domain.News
	for rows.Next() {
		var n domain.News
		var publishedAt sql.NullTime
		if err := rows.Scan(
			&n.ID, &n.Title, &n.Content, &n.SortOrder, &n.IsActive,
			&publishedAt, &n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan news: %w", err)
		}
		if publishedAt.Valid {
			t := publishedAt.Time
			n.PublishedAt = &t
		}
		news = append(news, n)
	}
	return news, rows.Err()
}
