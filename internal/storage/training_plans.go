package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"therunish/internal/domain"
)

const planCols = `id, to_char(week_start, 'YYYY-MM-DD') AS week_start, status, groups, materials,
	published_at, notified_at, notify_sent, created_at, updated_at`

func scanPlan(sc rowScanner) (domain.TrainingPlan, error) {
	var p domain.TrainingPlan
	var groups, materials []byte
	var publishedAt, notifiedAt sql.NullTime
	if err := sc.Scan(
		&p.ID, &p.WeekStart, &p.Status, &groups, &materials,
		&publishedAt, &notifiedAt, &p.NotifySent, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return domain.TrainingPlan{}, err
	}
	if err := json.Unmarshal(groups, &p.Groups); err != nil {
		return domain.TrainingPlan{}, fmt.Errorf("unmarshal plan groups: %w", err)
	}
	if err := json.Unmarshal(materials, &p.Materials); err != nil {
		return domain.TrainingPlan{}, fmt.Errorf("unmarshal plan materials: %w", err)
	}
	if publishedAt.Valid {
		t := publishedAt.Time
		p.PublishedAt = &t
	}
	if notifiedAt.Valid {
		t := notifiedAt.Time
		p.NotifiedAt = &t
	}
	return p, nil
}

func (s *Store) ListPlans(ctx context.Context, publishedOnly bool) ([]domain.TrainingPlan, error) {
	q := `SELECT ` + planCols + ` FROM training_plans`
	if publishedOnly {
		q += ` WHERE status = 'published'`
	}
	q += ` ORDER BY week_start DESC`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list training plans: %w", err)
	}
	defer rows.Close()

	var plans []domain.TrainingPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan training plan: %w", err)
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func (s *Store) GetPlan(ctx context.Context, id int64) (domain.TrainingPlan, error) {
	q := `SELECT ` + planCols + ` FROM training_plans WHERE id = $1`
	p, err := scanPlan(s.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TrainingPlan{}, ErrNotFound
	}
	if err != nil {
		return domain.TrainingPlan{}, fmt.Errorf("get training plan: %w", err)
	}
	return p, nil
}

// GetPlanByWeekAny — план недели в любом статусе (проверка дубля при создании).
func (s *Store) GetPlanByWeekAny(ctx context.Context, weekStart string) (domain.TrainingPlan, error) {
	q := `SELECT ` + planCols + ` FROM training_plans WHERE week_start = $1`
	p, err := scanPlan(s.db.QueryRowContext(ctx, q, weekStart))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TrainingPlan{}, ErrNotFound
	}
	if err != nil {
		return domain.TrainingPlan{}, fmt.Errorf("get plan by week: %w", err)
	}
	return p, nil
}

// GetLatestPlanBefore — ближайший предыдущий план (источник для копирования недели).
func (s *Store) GetLatestPlanBefore(ctx context.Context, weekStart string) (domain.TrainingPlan, error) {
	q := `SELECT ` + planCols + ` FROM training_plans
		WHERE week_start < $1 ORDER BY week_start DESC LIMIT 1`
	p, err := scanPlan(s.db.QueryRowContext(ctx, q, weekStart))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TrainingPlan{}, ErrNotFound
	}
	if err != nil {
		return domain.TrainingPlan{}, fmt.Errorf("get latest plan before: %w", err)
	}
	return p, nil
}

// GetPublishedPlanByWeek — опубликованный план конкретной недели (для пользователя).
func (s *Store) GetPublishedPlanByWeek(ctx context.Context, weekStart string) (domain.TrainingPlan, error) {
	q := `SELECT ` + planCols + ` FROM training_plans WHERE week_start = $1 AND status = 'published'`
	p, err := scanPlan(s.db.QueryRowContext(ctx, q, weekStart))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TrainingPlan{}, ErrNotFound
	}
	if err != nil {
		return domain.TrainingPlan{}, fmt.Errorf("get published plan by week: %w", err)
	}
	return p, nil
}

// GetLatestPublishedPlan — план текущей недели, а если её ещё нет — ближайший прошлый.
func (s *Store) GetLatestPublishedPlan(ctx context.Context, onOrBefore string) (domain.TrainingPlan, error) {
	q := `SELECT ` + planCols + ` FROM training_plans
		WHERE status = 'published' AND week_start <= $1
		ORDER BY week_start DESC LIMIT 1`
	p, err := scanPlan(s.db.QueryRowContext(ctx, q, onOrBefore))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TrainingPlan{}, ErrNotFound
	}
	if err != nil {
		return domain.TrainingPlan{}, fmt.Errorf("get latest published plan: %w", err)
	}
	return p, nil
}

func (s *Store) ListPublishedWeeks(ctx context.Context) ([]string, error) {
	const q = `SELECT to_char(week_start, 'YYYY-MM-DD') FROM training_plans
		WHERE status = 'published' ORDER BY week_start DESC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list published weeks: %w", err)
	}
	defer rows.Close()

	var weeks []string
	for rows.Next() {
		var w string
		if err := rows.Scan(&w); err != nil {
			return nil, fmt.Errorf("scan published week: %w", err)
		}
		weeks = append(weeks, w)
	}
	return weeks, rows.Err()
}

func (s *Store) CreatePlan(ctx context.Context, weekStart string, groups []domain.PlanGroup, materials []domain.PlanMaterial) (int64, error) {
	groupsJSON, materialsJSON, err := marshalPlanContent(groups, materials)
	if err != nil {
		return 0, err
	}
	const q = `INSERT INTO training_plans (week_start, groups, materials) VALUES ($1, $2, $3) RETURNING id`
	var id int64
	if err := s.db.QueryRowContext(ctx, q, weekStart, groupsJSON, materialsJSON).Scan(&id); err != nil {
		return 0, fmt.Errorf("create training plan: %w", err)
	}
	return id, nil
}

func (s *Store) UpdatePlanContent(ctx context.Context, id int64, groups []domain.PlanGroup, materials []domain.PlanMaterial) error {
	groupsJSON, materialsJSON, err := marshalPlanContent(groups, materials)
	if err != nil {
		return err
	}
	const q = `UPDATE training_plans SET groups = $2, materials = $3, updated_at = now() WHERE id = $1`
	res, err := s.db.ExecContext(ctx, q, id, groupsJSON, materialsJSON)
	if err != nil {
		return fmt.Errorf("update training plan: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetPlanStatus — публикация и снятие с публикации. published_at ставится один раз.
func (s *Store) SetPlanStatus(ctx context.Context, id int64, status string) error {
	const q = `
		UPDATE training_plans
		SET status = $2,
			published_at = CASE WHEN $2 = 'published' AND published_at IS NULL THEN now() ELSE published_at END,
			updated_at = now()
		WHERE id = $1`
	res, err := s.db.ExecContext(ctx, q, id, status)
	if err != nil {
		return fmt.Errorf("set training plan status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MarkPlanNotified(ctx context.Context, id int64, sent int) error {
	const q = `UPDATE training_plans SET notified_at = now(), notify_sent = $2, updated_at = now() WHERE id = $1`
	if _, err := s.db.ExecContext(ctx, q, id, sent); err != nil {
		return fmt.Errorf("mark training plan notified: %w", err)
	}
	return nil
}

func (s *Store) DeletePlan(ctx context.Context, id int64) error {
	const q = `DELETE FROM training_plans WHERE id = $1`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete training plan: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func marshalPlanContent(groups []domain.PlanGroup, materials []domain.PlanMaterial) ([]byte, []byte, error) {
	if groups == nil {
		groups = []domain.PlanGroup{}
	}
	if materials == nil {
		materials = []domain.PlanMaterial{}
	}
	groupsJSON, err := json.Marshal(groups)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal plan groups: %w", err)
	}
	materialsJSON, err := json.Marshal(materials)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal plan materials: %w", err)
	}
	return groupsJSON, materialsJSON, nil
}
