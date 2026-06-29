package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"therunish/internal/domain"
)

// serviceCols — общий список колонок для SELECT (порядок совпадает со scanService).
const serviceCols = `id, kind, title, description, price_kop, duration_days,
	sort_order, is_active, price_with_sub_kop, promo_price_kop, created_at, updated_at`

// rowScanner — общий интерфейс для *sql.Row и *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanService(sc rowScanner) (domain.Service, error) {
	var (
		svc       domain.Service
		duration  sql.NullInt64
		withSub   sql.NullInt64
		promo     sql.NullInt64
	)
	err := sc.Scan(
		&svc.ID, &svc.Kind, &svc.Title, &svc.Description, &svc.PriceKop,
		&duration, &svc.SortOrder, &svc.IsActive, &withSub, &promo,
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
	return svc, nil
}

// CreateService вставляет новую услугу и возвращает её ID.
func (s *Store) CreateService(ctx context.Context, svc *domain.Service) (int64, error) {
	const q = `
		INSERT INTO services (kind, title, description, price_kop, duration_days, sort_order, is_active, price_with_sub_kop, promo_price_kop)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`
	var id int64
	err := s.db.QueryRowContext(ctx, q,
		svc.Kind, svc.Title, svc.Description, svc.PriceKop,
		svc.DurationDays, svc.SortOrder, svc.IsActive, svc.PriceWithSubKop, svc.PromoPriceKop,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create service: %w", err)
	}
	return id, nil
}

// UpdateService обновляет услугу по ID.
func (s *Store) UpdateService(ctx context.Context, svc *domain.Service) error {
	const q = `
		UPDATE services
		SET kind = $1, title = $2, description = $3, price_kop = $4,
		    duration_days = $5, sort_order = $6, is_active = $7,
		    price_with_sub_kop = $8, promo_price_kop = $9, updated_at = now()
		WHERE id = $10`
	res, err := s.db.ExecContext(ctx, q,
		svc.Kind, svc.Title, svc.Description, svc.PriceKop,
		svc.DurationDays, svc.SortOrder, svc.IsActive, svc.PriceWithSubKop, svc.PromoPriceKop, svc.ID,
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

// DeactivateService — мягкое удаление (is_active = false).
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

// GetService возвращает услугу по ID (включая неактивные).
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

// ListServices возвращает услуги каталога. Если includeInactive=false — только активные.
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

// ListServicesByIDs возвращает услуги по списку ID (для резолва корзины).
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
