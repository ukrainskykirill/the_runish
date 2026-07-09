package domain

import "time"

type Training struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Weekday   int       `json:"weekday"`
	StartTime string    `json:"start_time"`
	Place     string    `json:"place"`
	IsActive  bool      `json:"is_active"`
	SortOrder int       `json:"sort_order"`
	Capacity  *int      `json:"capacity,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
