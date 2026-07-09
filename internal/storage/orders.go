package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"therunish/internal/domain"
)

func (s *Store) CreateOrder(ctx context.Context, order *domain.Order) (int64, error) {
	var orderID int64
	err := s.WithTx(ctx, func(tx *sql.Tx) error {
		const q = `
			INSERT INTO orders (user_id, amount_kop, status, contact_phone, contact_name, contact_tg)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id`
		err := tx.QueryRowContext(ctx, q,
			order.UserID, order.AmountKop, order.Status,
			order.ContactPhone, order.ContactName, order.ContactTg,
		).Scan(&orderID)
		if err != nil {
			return fmt.Errorf("insert order: %w", err)
		}

		const qi = `
			INSERT INTO order_items (order_id, service_id, title, price_kop, qty)
			VALUES ($1, $2, $3, $4, $5)`
		for _, item := range order.Items {
			if _, err := tx.ExecContext(ctx, qi,
				orderID, item.ServiceID, item.Title, item.PriceKop, item.Qty,
			); err != nil {
				return fmt.Errorf("insert order item: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return orderID, nil
}

func (s *Store) GetOrderByID(ctx context.Context, id int64) (domain.Order, error) {
	const q = `
		SELECT id, user_id, amount_kop, status, contact_phone, contact_name, contact_tg, created_at
		FROM orders WHERE id = $1`
	var o domain.Order
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&o.ID, &o.UserID, &o.AmountKop, &o.Status,
		&o.ContactPhone, &o.ContactName, &o.ContactTg, &o.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Order{}, ErrNotFound
	}
	if err != nil {
		return domain.Order{}, fmt.Errorf("get order: %w", err)
	}

	items, err := s.getOrderItems(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}
	o.Items = items
	return o, nil
}

func (s *Store) ListOrdersByUser(ctx context.Context, userID int64) ([]domain.Order, error) {
	const q = `
		SELECT id, user_id, amount_kop, status, contact_phone, contact_name, contact_tg, created_at
		FROM orders WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.AmountKop, &o.Status,
			&o.ContactPhone, &o.ContactName, &o.ContactTg, &o.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (s *Store) getOrderItems(ctx context.Context, orderID int64) ([]domain.OrderItem, error) {
	const q = `
		SELECT id, order_id, service_id, title, price_kop, qty
		FROM order_items WHERE order_id = $1 ORDER BY id`
	rows, err := s.db.QueryContext(ctx, q, orderID)
	if err != nil {
		return nil, fmt.Errorf("get order items: %w", err)
	}
	defer rows.Close()

	var items []domain.OrderItem
	for rows.Next() {
		var it domain.OrderItem
		if err := rows.Scan(&it.ID, &it.OrderID, &it.ServiceID, &it.Title, &it.PriceKop, &it.Qty); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
