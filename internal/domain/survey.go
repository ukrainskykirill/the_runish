package domain

import "time"

// SurveyStatus — статус прохождения онбординг-анкеты.
type SurveyStatus string

const (
	SurveyPending    SurveyStatus = "pending"     // создана при регистрации, ещё не начата
	SurveyInProgress SurveyStatus = "in_progress" // идёт диалог в боте
	SurveyCompleted  SurveyStatus = "completed"   // пройдена
)

// Survey — состояние и ответы онбординг-анкеты пользователя.
type Survey struct {
	UserID      int64
	Status      SurveyStatus
	Branch      string         // novice | casual | regular (пусто до выбора)
	Step        string         // ключ текущего шага
	Answers     map[string]any // step_key -> string | []any (мультивыбор)
	MsgID       *int64         // message_id текущего вопроса
	StartedAt   *time.Time
	CompletedAt *time.Time
	CreatedAt   time.Time
}
