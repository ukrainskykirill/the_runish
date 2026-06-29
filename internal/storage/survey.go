package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"therunish/internal/domain"
)

// GetSurvey возвращает анкету юзера. ErrNotFound — если записи нет
// (например, пользователь зарегистрирован до выката фичи).
func (s *Store) GetSurvey(ctx context.Context, userID int64) (domain.Survey, error) {
	const q = `
		SELECT user_id, status, branch, step, answers, msg_id, started_at, completed_at, created_at
		FROM survey_responses WHERE user_id = $1`

	var (
		sv          domain.Survey
		branch      sql.NullString
		step        sql.NullString
		answersRaw  []byte
		msgID       sql.NullInt64
		startedAt   sql.NullTime
		completedAt sql.NullTime
	)
	err := s.db.QueryRowContext(ctx, q, userID).Scan(
		&sv.UserID, &sv.Status, &branch, &step, &answersRaw, &msgID, &startedAt, &completedAt, &sv.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Survey{}, ErrNotFound
	}
	if err != nil {
		return domain.Survey{}, fmt.Errorf("get survey: %w", err)
	}

	sv.Branch = branch.String
	sv.Step = step.String
	if len(answersRaw) > 0 {
		if err := json.Unmarshal(answersRaw, &sv.Answers); err != nil {
			return domain.Survey{}, fmt.Errorf("unmarshal survey answers: %w", err)
		}
	}
	if sv.Answers == nil {
		sv.Answers = map[string]any{}
	}
	if msgID.Valid {
		sv.MsgID = &msgID.Int64
	}
	if startedAt.Valid {
		sv.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		sv.CompletedAt = &completedAt.Time
	}
	return sv, nil
}

// SaveSurvey обновляет состояние анкеты (статус, ветку, текущий шаг, ответы, msg_id).
// started_at проставляется один раз при первом переходе в работу.
func (s *Store) SaveSurvey(ctx context.Context, userID int64, status domain.SurveyStatus, branch, step string, answers map[string]any, msgID *int64) error {
	raw, err := marshalAnswers(answers)
	if err != nil {
		return err
	}
	const q = `
		UPDATE survey_responses
		SET status = $2,
		    branch = NULLIF($3, ''),
		    step   = NULLIF($4, ''),
		    answers = $5,
		    msg_id = $6,
		    started_at = COALESCE(started_at, now())
		WHERE user_id = $1`
	if _, err := s.db.ExecContext(ctx, q, userID, status, branch, step, raw, nullInt64(msgID)); err != nil {
		return fmt.Errorf("save survey: %w", err)
	}
	return nil
}

// CompleteSurvey помечает анкету пройденной — повторно показываться не будет.
func (s *Store) CompleteSurvey(ctx context.Context, userID int64, answers map[string]any) error {
	raw, err := marshalAnswers(answers)
	if err != nil {
		return err
	}
	const q = `
		UPDATE survey_responses
		SET status = 'completed', step = NULL, answers = $2, completed_at = now()
		WHERE user_id = $1`
	if _, err := s.db.ExecContext(ctx, q, userID, raw); err != nil {
		return fmt.Errorf("complete survey: %w", err)
	}
	return nil
}

func marshalAnswers(answers map[string]any) ([]byte, error) {
	if answers == nil {
		return []byte("{}"), nil
	}
	raw, err := json.Marshal(answers)
	if err != nil {
		return nil, fmt.Errorf("marshal survey answers: %w", err)
	}
	return raw, nil
}

func nullInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}
