package domain

import "time"

// Training — тренировка в недельном расписании (повторяется по дню недели).
// Weekday по ISO: 1=Пн … 7=Вс. StartTime — строка "HH:MM" (локальное время клуба).
type Training struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Weekday   int       `json:"weekday"`
	StartTime string    `json:"start_time"`
	Place     string    `json:"place"`
	IsActive  bool      `json:"is_active"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
