package domain

import "time"

const (
	PlanStatusDraft     = "draft"
	PlanStatusPublished = "published"
)

// PlanDay — одна строка недельной сетки: «Дата · День недели · Тип тренировки · Задание».
type PlanDay struct {
	Date      string `json:"date"`
	Weekday   int    `json:"weekday"`
	Kind      string `json:"kind"`
	Task      string `json:"task"`
	LinkLabel string `json:"link_label,omitempty"`
	LinkURL   string `json:"link_url,omitempty"`
}

// PlanGroup — блок плана для одной группы («The Runish Start», «The Runish Progress»).
type PlanGroup struct {
	Title string    `json:"title"`
	Days  []PlanDay `json:"days"`
}

type PlanMaterial struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type TrainingPlan struct {
	ID          int64          `json:"id"`
	WeekStart   string         `json:"week_start"`
	Status      string         `json:"status"`
	Groups      []PlanGroup    `json:"groups"`
	Materials   []PlanMaterial `json:"materials"`
	PublishedAt *time.Time     `json:"published_at,omitempty"`
	NotifiedAt  *time.Time     `json:"notified_at,omitempty"`
	NotifySent  int            `json:"notify_sent"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func (p TrainingPlan) IsPublished() bool { return p.Status == PlanStatusPublished }
