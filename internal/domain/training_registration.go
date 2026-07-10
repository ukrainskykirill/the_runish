package domain

import "time"

type TrainingOccurrence struct {
	TrainingID       int64  `json:"training_id"`
	Title            string `json:"title"`
	Weekday          int    `json:"weekday"`
	StartTime        string `json:"start_time"`
	Place            string `json:"place"`
	PlaceURL         *string `json:"place_url,omitempty"`
	SessionDate      string `json:"session_date"`
	Capacity         *int   `json:"capacity,omitempty"`
	RegisteredCount  int    `json:"registered_count"`
	Available        bool   `json:"available"`
	Cancelled        bool   `json:"cancelled"`
	Registered       bool   `json:"registered"`
	MyRegistrationID *int64 `json:"my_registration_id,omitempty"`
	InMyAccess       bool   `json:"in_my_access"`
	QuotaLeft        *int   `json:"quota_left,omitempty"`
}

type TrainingRegistration struct {
	ID          int64     `json:"id"`
	TrainingID  int64     `json:"training_id"`
	Title       string    `json:"title"`
	SessionDate string    `json:"session_date"`
	StartTime   string    `json:"start_time"`
	Place       string    `json:"place"`
	CreatedAt   time.Time `json:"created_at"`
}
