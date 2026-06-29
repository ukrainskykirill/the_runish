package storage

import (
	"context"
	"fmt"
)

// CartItem — позиция корзины.
type CartItem struct {
	ID        int64
	SessionID string
	ServiceID int64
	Qty       int
}

// AddToCart добавляет услугу в корзину сессии. Каждый товар — максимум 1 шт.:
// повторное добавление того же товара ничего не меняет (qty всегда 1).
func (s *Store) AddToCart(ctx context.Context, sessionID string, serviceID int64) error {
	const q = `
		INSERT INTO cart_items (session_id, service_id, qty)
		VALUES ($1, $2, 1)
		ON CONFLICT (session_id, service_id) DO NOTHING`
	_, err := s.db.ExecContext(ctx, q, sessionID, serviceID)
	if err != nil {
		return fmt.Errorf("add to cart: %w", err)
	}
	return nil
}

// GetCart возвращает все позиции корзины для сессии.
func (s *Store) GetCart(ctx context.Context, sessionID string) ([]CartItem, error) {
	const q = `
		SELECT id, session_id, service_id, qty
		FROM cart_items
		WHERE session_id = $1
		ORDER BY id`
	rows, err := s.db.QueryContext(ctx, q, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get cart: %w", err)
	}
	defer rows.Close()

	var items []CartItem
	for rows.Next() {
		var it CartItem
		if err := rows.Scan(&it.ID, &it.SessionID, &it.ServiceID, &it.Qty); err != nil {
			return nil, fmt.Errorf("scan cart item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// RemoveFromCart удаляет позицию из корзины.
func (s *Store) RemoveFromCart(ctx context.Context, sessionID string, serviceID int64) error {
	const q = `DELETE FROM cart_items WHERE session_id = $1 AND service_id = $2`
	_, err := s.db.ExecContext(ctx, q, sessionID, serviceID)
	if err != nil {
		return fmt.Errorf("remove from cart: %w", err)
	}
	return nil
}

// ClearCart полностью очищает корзину сессии.
func (s *Store) ClearCart(ctx context.Context, sessionID string) error {
	const q = `DELETE FROM cart_items WHERE session_id = $1`
	_, err := s.db.ExecContext(ctx, q, sessionID)
	if err != nil {
		return fmt.Errorf("clear cart: %w", err)
	}
	return nil
}
