package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"therunish/internal/domain"
)

// trainingCols — общий SELECT-список. Время отдаём строкой "HH:MM".
const trainingCols = `id, title, weekday, to_char(start_time, 'HH24:MI') AS start_time,
	place, is_active, sort_order, created_at, updated_at`

// CreateTraining вставляет тренировку и возвращает её ID.
func (s *Store) CreateTraining(ctx context.Context, t *domain.Training) (int64, error) {
	const q = `
		INSERT INTO trainings (title, weekday, start_time, place, is_active, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`
	var id int64
	err := s.db.QueryRowContext(ctx, q,
		t.Title, t.Weekday, t.StartTime, t.Place, t.IsActive, t.SortOrder,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create training: %w", err)
	}
	return id, nil
}

// UpdateTraining обновляет тренировку по ID.
func (s *Store) UpdateTraining(ctx context.Context, t *domain.Training) error {
	const q = `
		UPDATE trainings
		SET title = $1, weekday = $2, start_time = $3, place = $4, is_active = $5, sort_order = $6, updated_at = now()
		WHERE id = $7`
	res, err := s.db.ExecContext(ctx, q,
		t.Title, t.Weekday, t.StartTime, t.Place, t.IsActive, t.SortOrder, t.ID,
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

// DeleteTraining удаляет тренировку по ID.
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

// GetTraining возвращает тренировку по ID (включая неактивные).
func (s *Store) GetTraining(ctx context.Context, id int64) (domain.Training, error) {
	q := `SELECT ` + trainingCols + ` FROM trainings WHERE id = $1`
	var t domain.Training
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&t.ID, &t.Title, &t.Weekday, &t.StartTime, &t.Place,
		&t.IsActive, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Training{}, ErrNotFound
	}
	if err != nil {
		return domain.Training{}, fmt.Errorf("get training: %w", err)
	}
	return t, nil
}

// ListTrainings возвращает тренировки. Если includeInactive=false — только активные.
// Отсортированы по дню недели, времени, порядку.
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
		var t domain.Training
		if err := rows.Scan(
			&t.ID, &t.Title, &t.Weekday, &t.StartTime, &t.Place,
			&t.IsActive, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan training: %w", err)
		}
		trainings = append(trainings, t)
	}
	return trainings, rows.Err()
}
