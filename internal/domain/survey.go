package domain

import "time"

type SurveyStatus string

const (
	SurveyPending    SurveyStatus = "pending"
	SurveyInProgress SurveyStatus = "in_progress"
	SurveyCompleted  SurveyStatus = "completed"
)

type Survey struct {
	UserID      int64
	Status      SurveyStatus
	Branch      string
	Step        string
	Answers     map[string]any
	MsgID       *int64
	StartedAt   *time.Time
	CompletedAt *time.Time
	CreatedAt   time.Time
}

const (
	SurveyPhaseIntro  = "intro"
	SurveyPhaseBranch = "branch"
	SurveyPhaseOutro  = "outro"

	SurveyKindSingle = "single"
	SurveyKindMulti  = "multi"
	SurveyKindText   = "text"
)

type SurveyOption struct {
	Label  string `json:"label"`
	Branch string `json:"branch,omitempty"`
}

type SurveyQuestion struct {
	ID         int64          `json:"id"`
	Key        string         `json:"key"`
	Phase      string         `json:"phase"`
	Branch     string         `json:"branch"`
	Kind       string         `json:"kind"`
	Label      string         `json:"label"`
	Prompt     string         `json:"prompt"`
	Options    []SurveyOption `json:"options"`
	IsSelector bool           `json:"is_selector"`
	Position   int            `json:"position"`
	IsActive   bool           `json:"is_active"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}
