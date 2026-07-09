package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"therunish/internal/domain"
)

const trainingCols = `id, title, weekday, to_char(start_time, 'HH24:MI') AS start_time,
	place, is_active, sort_order, capacity, created_at, updated_at`

func scanTraining(sc rowScanner) (domain.Training, error) {
	var t domain.Training
	var capacity sql.NullInt64
	if err := sc.Scan(
		&t.ID, &t.Title, &t.Weekday, &t.StartTime, &t.Place,
		&t.IsActive, &t.SortOrder, &capacity, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return domain.Training{}, err
	}
	if capacity.Valid {
		c := int(capacity.Int64)
		t.Capacity = &c
	}
	return t, nil
}

func (s *Store) CreateTraining(ctx context.Context, t *domain.Training) (int64, error) {
	const q = `
		INSERT INTO trainings (title, weekday, start_time, place, is_active, sort_order, capacity)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`
	var id int64
	err := s.db.QueryRowContext(ctx, q,
		t.Title, t.Weekday, t.StartTime, t.Place, t.IsActive, t.SortOrder, t.Capacity,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create training: %w", err)
	}
	return id, nil
}

func (s *Store) UpdateTraining(ctx context.Context, t *domain.Training) error {
	const q = `
		UPDATE trainings
		SET title = $1, weekday = $2, start_time = $3, place = $4, is_active = $5, sort_order = $6, capacity = $7, updated_at = now()
		WHERE id = $8`
	res, err := s.db.ExecContext(ctx, q,
		t.Title, t.Weekday, t.StartTime, t.Place, t.IsActive, t.SortOrder, t.Capacity, t.ID,
	)
	if err != nil {
		return fmt.Errorf("update training: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTraining(ctx context.Context, id int64) error {
	const q = `DELETE FROM trainings WHERE id = $1`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete training: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetTraining(ctx context.Context, id int64) (domain.Training, error) {
	q := `SELECT ` + trainingCols + ` FROM trainings WHERE id = $1`
	t, err := scanTraining(s.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Training{}, ErrNotFound
	}
	if err != nil {
		return domain.Training{}, fmt.Errorf("get training: %w", err)
	}
	return t, nil
}

func (s *Store) ListTrainings(ctx context.Context, includeInactive bool) ([]domain.Training, error) {
	q := `SELECT ` + trainingCols + ` FROM trainings`
	if !includeInactive {
		q += ` WHERE is_active = true`
	}
	q += ` ORDER BY weekday, start_time, sort_order, id`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list trainings: %w", err)
	}
	defer rows.Close()

	var trainings []domain.Training
	for rows.Next() {
		t, err := scanTraining(rows)
		if err != nil {
			return nil, fmt.Errorf("scan training: %w", err)
		}
		trainings = append(trainings, t)
	}
	return trainings, rows.Err()
}
