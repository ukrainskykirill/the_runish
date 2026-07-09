package storage

import (
	"context"
	"fmt"
)

type CartItem struct {
	ID        int64
	UserID    int64
	ServiceID int64
	Qty       int
}

func (s *Store) AddToCart(ctx context.Context, userID, serviceID int64) error {
	const q = `
		INSERT INTO cart_items (user_id, service_id, qty)
		VALUES ($1, $2, 1)
		ON CONFLICT (user_id, service_id) DO NOTHING`
	_, err := s.db.ExecContext(ctx, q, userID, serviceID)
	if err != nil {
		return fmt.Errorf("add to cart: %w", err)
	}
	return nil
}

func (s *Store) GetCart(ctx context.Context, userID int64) ([]CartItem, error) {
	const q = `
		SELECT id, user_id, service_id, qty
		FROM cart_items
		WHERE user_id = $1
		ORDER BY id`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("get cart: %w", err)
	}
	defer rows.Close()

	var items []CartItem
	for rows.Next() {
		var it CartItem
		if err := rows.Scan(&it.ID, &it.UserID, &it.ServiceID, &it.Qty); err != nil {
			return nil, fmt.Errorf("scan cart item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *Store) RemoveFromCart(ctx context.Context, userID, serviceID int64) error {
	const q = `DELETE FROM cart_items WHERE user_id = $1 AND service_id = $2`
	_, err := s.db.ExecContext(ctx, q, userID, serviceID)
	if err != nil {
		return fmt.Errorf("remove from cart: %w", err)
	}
	return nil
}

func (s *Store) ClearCart(ctx context.Context, userID int64) error {
	const q = `DELETE FROM cart_items WHERE user_id = $1`
	_, err := s.db.ExecContext(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("clear cart: %w", err)
	}
	return nil
}
