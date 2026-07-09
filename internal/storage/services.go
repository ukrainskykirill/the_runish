package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"therunish/internal/domain"
)

const serviceCols = `id, kind, title, description, price_kop, duration_days,
	sort_order, is_active, price_with_sub_kop, promo_price_kop, trainings_quota, created_at, updated_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanService(sc rowScanner) (domain.Service, error) {
	var (
		svc      domain.Service
		duration sql.NullInt64
		withSub  sql.NullInt64
		promo    sql.NullInt64
		quota    sql.NullInt64
	)
	err := sc.Scan(
		&svc.ID, &svc.Kind, &svc.Title, &svc.Description, &svc.PriceKop,
		&duration, &svc.SortOrder, &svc.IsActive, &withSub, &promo, &quota,
		&svc.CreatedAt, &svc.UpdatedAt,
	)
	if err != nil {
		return domain.Service{}, err
	}
	if duration.Valid {
		d := int(duration.Int64)
		svc.DurationDays = &d
	}
	if withSub.Valid {
		v := withSub.Int64
		svc.PriceWithSubKop = &v
	}
	if promo.Valid {
		v := promo.Int64
		svc.PromoPriceKop = &v
	}
	if quota.Valid {
		v := int(quota.Int64)
		svc.TrainingsQuota = &v
	}
	return svc, nil
}

func (s *Store) CreateService(ctx context.Context, svc *domain.Service) (int64, error) {
	const q = `
		INSERT INTO services (kind, title, description, price_kop, duration_days, sort_order, is_active, price_with_sub_kop, promo_price_kop, trainings_quota)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`
	var id int64
	err := s.db.QueryRowContext(ctx, q,
		svc.Kind, svc.Title, svc.Description, svc.PriceKop,
		svc.DurationDays, svc.SortOrder, svc.IsActive, svc.PriceWithSubKop, svc.PromoPriceKop, svc.TrainingsQuota,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create service: %w", err)
	}
	return id, nil
}

func (s *Store) CreateServiceWithTrainings(ctx context.Context, svc *domain.Service, trainingIDs []int64) (int64, error) {
	var id int64
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		createdID, err := createService(ctx, tx, svc)
		if err != nil {
			return err
		}
		if err := setServiceTrainings(ctx, tx, createdID, trainingIDs); err != nil {
			return err
		}
		id = createdID
		return nil
	})
	return id, err
}

func (s *Store) UpdateService(ctx context.Context, svc *domain.Service) error {
	const q = `
		UPDATE services
		SET kind = $1, title = $2, description = $3, price_kop = $4,
		    duration_days = $5, sort_order = $6, is_active = $7,
		    price_with_sub_kop = $8, promo_price_kop = $9, trainings_quota = $10, updated_at = now()
		WHERE id = $11`
	res, err := s.db.ExecContext(ctx, q,
		svc.Kind, svc.Title, svc.Description, svc.PriceKop,
		svc.DurationDays, svc.SortOrder, svc.IsActive, svc.PriceWithSubKop, svc.PromoPriceKop, svc.TrainingsQuota, svc.ID,
	)
	if err != nil {
		return fmt.Errorf("update service: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateServiceWithTrainings(ctx context.Context, svc *domain.Service, trainingIDs []int64) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		if err := updateService(ctx, tx, svc); err != nil {
			return err
		}
		return setServiceTrainings(ctx, tx, svc.ID, trainingIDs)
	})
}

func (s *Store) DeactivateService(ctx context.Context, id int64) error {
	const q = `UPDATE services SET is_active = false, updated_at = now() WHERE id = $1`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("deactivate service: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetService(ctx context.Context, id int64) (domain.Service, error) {
	q := `SELECT ` + serviceCols + ` FROM services WHERE id = $1`
	svc, err := scanService(s.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Service{}, ErrNotFound
	}
	if err != nil {
		return domain.Service{}, fmt.Errorf("get service: %w", err)
	}
	return svc, nil
}

func (s *Store) ListServices(ctx context.Context, includeInactive bool) ([]domain.Service, error) {
	q := `SELECT ` + serviceCols + ` FROM services`
	if !includeInactive {
		q += ` WHERE is_active = true`
	}
	q += ` ORDER BY sort_order, id`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	var services []domain.Service
	for rows.Next() {
		svc, err := scanService(rows)
		if err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		services = append(services, svc)
	}
	return services, rows.Err()
}

func (s *Store) GetServiceTrainingIDs(ctx context.Context, serviceID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT training_id FROM service_trainings WHERE service_id = $1 ORDER BY training_id`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("get service trainings: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan service training: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) SetServiceTrainings(ctx context.Context, serviceID int64, trainingIDs []int64) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		return setServiceTrainings(ctx, tx, serviceID, trainingIDs)
	})
}

func createService(ctx context.Context, q dbExecQuerier, svc *domain.Service) (int64, error) {
	const stmt = `
		INSERT INTO services (kind, title, description, price_kop, duration_days, sort_order, is_active, price_with_sub_kop, promo_price_kop, trainings_quota)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`
	var id int64
	err := q.QueryRowContext(ctx, stmt,
		svc.Kind, svc.Title, svc.Description, svc.PriceKop,
		svc.DurationDays, svc.SortOrder, svc.IsActive, svc.PriceWithSubKop, svc.PromoPriceKop, svc.TrainingsQuota,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create service: %w", err)
	}
	return id, nil
}

func updateService(ctx context.Context, q dbExecQuerier, svc *domain.Service) error {
	const stmt = `
		UPDATE services
		SET kind = $1, title = $2, description = $3, price_kop = $4,
		    duration_days = $5, sort_order = $6, is_active = $7,
		    price_with_sub_kop = $8, promo_price_kop = $9, trainings_quota = $10, updated_at = now()
		WHERE id = $11`
	res, err := q.ExecContext(ctx, stmt,
		svc.Kind, svc.Title, svc.Description, svc.PriceKop,
		svc.DurationDays, svc.SortOrder, svc.IsActive, svc.PriceWithSubKop, svc.PromoPriceKop, svc.TrainingsQuota, svc.ID,
	)
	if err != nil {
		return fmt.Errorf("update service: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func setServiceTrainings(ctx context.Context, q dbExecQuerier, serviceID int64, trainingIDs []int64) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM service_trainings WHERE service_id = $1`, serviceID); err != nil {
		return fmt.Errorf("clear service trainings: %w", err)
	}
	for _, tid := range trainingIDs {
		if _, err := q.ExecContext(ctx,
			`INSERT INTO service_trainings (service_id, training_id) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`, serviceID, tid); err != nil {
			return fmt.Errorf("insert service training: %w", err)
		}
	}
	return nil
}

func (s *Store) ListServicesByIDs(ctx context.Context, ids []int64) (map[int64]domain.Service, error) {
	if len(ids) == 0 {
		return map[int64]domain.Service{}, nil
	}

	q := `SELECT ` + serviceCols + ` FROM services WHERE id = ANY($1)`
	rows, err := s.db.QueryContext(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("list services by ids: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]domain.Service, len(ids))
	for rows.Next() {
		svc, err := scanService(rows)
		if err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		result[svc.ID] = svc
	}
	return result, rows.Err()
}
