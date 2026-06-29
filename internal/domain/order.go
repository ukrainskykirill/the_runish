package domain

import "time"

type OrderStatus string

const (
	OrderStatusCreated  OrderStatus = "created"
	OrderStatusPaid     OrderStatus = "paid"
	OrderStatusCanceled OrderStatus = "cancelled"
)

type Order struct {
	ID           int64       `json:"id"`
	UserID       int64       `json:"user_id"`
	AmountKop    int64       `json:"amount_kop"`
	Status       OrderStatus `json:"status"`
	ContactPhone string      `json:"contact_phone"`
	ContactName  string      `json:"contact_name"`
	ContactTg    string      `json:"contact_tg"`
	CreatedAt    time.Time   `json:"created_at"`

	Items []OrderItem `json:"items,omitempty"`
}

type OrderItem struct {
	ID        int64  `json:"id"`
	OrderID   int64  `json:"order_id"`
	ServiceID int64  `json:"service_id"`
	Title     string `json:"title"`
	PriceKop  int64  `json:"price_kop"`
	Qty       int    `json:"qty"`
}
