package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"therunish/internal/domain"
)

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

const surveyQuestionCols = `id, qkey, phase, branch, kind, label, prompt, options, is_selector, position, is_active, created_at, updated_at`

func scanSurveyQuestion(sc rowScanner, q *domain.SurveyQuestion) error {
	var (
		branch     sql.NullString
		optionsRaw []byte
	)
	if err := sc.Scan(
		&q.ID, &q.Key, &q.Phase, &branch, &q.Kind, &q.Label, &q.Prompt,
		&optionsRaw, &q.IsSelector, &q.Position, &q.IsActive, &q.CreatedAt, &q.UpdatedAt,
	); err != nil {
		return err
	}
	q.Branch = branch.String
	q.Options = []domain.SurveyOption{}
	if len(optionsRaw) > 0 {
		if err := json.Unmarshal(optionsRaw, &q.Options); err != nil {
			return fmt.Errorf("unmarshal survey options: %w", err)
		}
	}
	return nil
}

func (s *Store) ListSurveyQuestions(ctx context.Context, activeOnly bool) ([]domain.SurveyQuestion, error) {
	q := `SELECT ` + surveyQuestionCols + ` FROM survey_questions`
	if activeOnly {
		q += ` WHERE is_active = true`
	}
	q += `
		ORDER BY CASE phase WHEN 'intro' THEN 0 WHEN 'branch' THEN 1 WHEN 'outro' THEN 2 ELSE 3 END,
		         branch NULLS FIRST, position, id`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list survey questions: %w", err)
	}
	defer rows.Close()

	var out []domain.SurveyQuestion
	for rows.Next() {
		var sq domain.SurveyQuestion
		if err := scanSurveyQuestion(rows, &sq); err != nil {
			return nil, fmt.Errorf("scan survey question: %w", err)
		}
		out = append(out, sq)
	}
	return out, rows.Err()
}

func (s *Store) GetSurveyQuestion(ctx context.Context, id int64) (domain.SurveyQuestion, error) {
	q := `SELECT ` + surveyQuestionCols + ` FROM survey_questions WHERE id = $1`
	var sq domain.SurveyQuestion
	err := scanSurveyQuestion(s.db.QueryRowContext(ctx, q, id), &sq)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SurveyQuestion{}, ErrNotFound
	}
	if err != nil {
		return domain.SurveyQuestion{}, fmt.Errorf("get survey question: %w", err)
	}
	return sq, nil
}

func marshalSurveyOptions(opts []domain.SurveyOption) ([]byte, error) {
	if opts == nil {
		opts = []domain.SurveyOption{}
	}
	raw, err := json.Marshal(opts)
	if err != nil {
		return nil, fmt.Errorf("marshal survey options: %w", err)
	}
	return raw, nil
}

func (s *Store) CreateSurveyQuestion(ctx context.Context, q *domain.SurveyQuestion) (int64, error) {
	raw, err := marshalSurveyOptions(q.Options)
	if err != nil {
		return 0, err
	}
	const sqlStmt = `
		INSERT INTO survey_questions (qkey, phase, branch, kind, label, prompt, options, is_selector, position, is_active)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`
	var id int64
	err = s.db.QueryRowContext(ctx, sqlStmt,
		q.Key, q.Phase, q.Branch, q.Kind, q.Label, q.Prompt, raw, q.IsSelector, q.Position, q.IsActive,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create survey question: %w", err)
	}
	return id, nil
}

func (s *Store) UpdateSurveyQuestion(ctx context.Context, q *domain.SurveyQuestion) error {
	raw, err := marshalSurveyOptions(q.Options)
	if err != nil {
		return err
	}
	const sqlStmt = `
		UPDATE survey_questions
		SET qkey = $1, phase = $2, branch = NULLIF($3, ''), kind = $4, label = $5,
		    prompt = $6, options = $7, is_selector = $8, position = $9, is_active = $10, updated_at = now()
		WHERE id = $11`
	res, err := s.db.ExecContext(ctx, sqlStmt,
		q.Key, q.Phase, q.Branch, q.Kind, q.Label, q.Prompt, raw, q.IsSelector, q.Position, q.IsActive, q.ID,
	)
	if err != nil {
		return fmt.Errorf("update survey question: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteSurveyQuestion(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM survey_questions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete survey question: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MoveSurveyQuestion(ctx context.Context, id int64, dir int) error {
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		var (
			phase    string
			branch   sql.NullString
			position int
		)
		err := tx.QueryRowContext(ctx,
			`SELECT phase, branch, position FROM survey_questions WHERE id = $1`, id,
		).Scan(&phase, &branch, &position)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("move survey question: load: %w", err)
		}

		cmp, order := ">", "ASC"
		if dir < 0 {
			cmp, order = "<", "DESC"
		}
		neighborQ := fmt.Sprintf(`
			SELECT id, position FROM survey_questions
			WHERE phase = $1 AND branch IS NOT DISTINCT FROM $2 AND position %s $3
			ORDER BY position %s, id %s
			LIMIT 1`, cmp, order, order)

		var (
			neighborID  int64
			neighborPos int
		)
		err = tx.QueryRowContext(ctx, neighborQ, phase, branch, position).Scan(&neighborID, &neighborPos)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("move survey question: neighbor: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE survey_questions SET position = $1, updated_at = now() WHERE id = $2`, neighborPos, id); err != nil {
			return fmt.Errorf("move survey question: set self: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE survey_questions SET position = $1, updated_at = now() WHERE id = $2`, position, neighborID); err != nil {
			return fmt.Errorf("move survey question: set neighbor: %w", err)
		}
		return nil
	})
}

func (s *Store) SaveCompletedSurvey(ctx context.Context, userID int64, branch string, answers map[string]any) error {
	raw, err := marshalAnswers(answers)
	if err != nil {
		return err
	}
	const q = `
		INSERT INTO survey_responses (user_id, status, branch, step, answers, started_at, completed_at)
		VALUES ($1, 'completed', NULLIF($2, ''), NULL, $3, now(), now())
		ON CONFLICT (user_id) DO UPDATE
			SET status = 'completed',
			    branch = NULLIF($2, ''),
			    step = NULL,
			    answers = $3,
			    started_at = COALESCE(survey_responses.started_at, now()),
			    completed_at = now()`
	if _, err := s.db.ExecContext(ctx, q, userID, branch, raw); err != nil {
		return fmt.Errorf("save completed survey: %w", err)
	}
	return nil
}
