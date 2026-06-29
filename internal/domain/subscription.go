package domain

import "time"

type SubscriptionStatus string

const (
	SubStatusActive    SubscriptionStatus = "active"
	SubStatusExpired   SubscriptionStatus = "expired"
	SubStatusCancelled SubscriptionStatus = "cancelled"
)

type Subscription struct {
	ID           int64              `json:"id"`
	UserID       int64              `json:"user_id"`
	ServiceID    int64              `json:"service_id"`
	ServiceTitle string             `json:"service_title"` // JOIN с services для отображения
	PaymentID    *int64             `json:"payment_id,omitempty"`
	Status       SubscriptionStatus `json:"status"`
	StartedAt    time.Time          `json:"started_at"`
	ExpiresAt    time.Time          `json:"expires_at"`
	Reminded7d   bool               `json:"reminded_7d"`
	Reminded3d   bool               `json:"reminded_3d"`
	Reminded1d   bool               `json:"reminded_1d"`
	CreatedAt    time.Time          `json:"created_at"`
}
