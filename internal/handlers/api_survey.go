package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"therunish/internal/auth"
	"therunish/internal/domain"
	"therunish/internal/storage"
	"therunish/internal/survey"
)

func (a *App) APISurveyGet(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	questions, err := a.store.ListSurveyQuestions(r.Context(), true)
	if err != nil {
		a.logger.Error("api survey: list questions", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if questions == nil {
		questions = []domain.SurveyQuestion{}
	}

	status := string(domain.SurveyPending)
	branch := ""
	answers := map[string]any{}
	if sv, err := a.store.GetSurvey(r.Context(), user.ID); err == nil {
		status = string(sv.Status)
		branch = sv.Branch
		if sv.Answers != nil {
			answers = sv.Answers
		}
	} else if !errors.Is(err, storage.ErrNotFound) {
		a.logger.Error("api survey: get response", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, struct {
		Status    string                  `json:"status"`
		Branch    string                  `json:"branch"`
		Answers   map[string]any          `json:"answers"`
		Greeting  string                  `json:"greeting"`
		Final     string                  `json:"final"`
		Questions []domain.SurveyQuestion `json:"questions"`
	}{
		Status:    status,
		Branch:    branch,
		Answers:   answers,
		Greeting:  survey.Greeting(),
		Final:     survey.Final(),
		Questions: questions,
	})
}

func (a *App) APISurveySubmit(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Answers map[string]any `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	questions, err := a.store.ListSurveyQuestions(r.Context(), true)
	if err != nil {
		a.logger.Error("api survey submit: list questions", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	tmpl := survey.Build(questions)

	clean := map[string]any{}
	branch := ""
	for _, q := range questions {
		v, ok := body.Answers[q.Key]
		if !ok {
			continue
		}
		clean[q.Key] = normalizeAnswer(q, v)
		if q.IsSelector {
			branch = branchForLabel(q, clean[q.Key])
		}
	}

	if !surveyComplete(tmpl, branch, clean) {
		writeJSONError(w, http.StatusBadRequest, "incomplete")
		return
	}

	if err := a.store.SaveCompletedSurvey(r.Context(), user.ID, branch, clean); err != nil {
		a.logger.Error("api survey submit: save", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": string(domain.SurveyCompleted)})
}

func (a *App) APISurveyProgress(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body struct {
		Answers map[string]any `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	questions, err := a.store.ListSurveyQuestions(r.Context(), true)
	if err != nil {
		a.logger.Error("api survey progress: list questions", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	clean := map[string]any{}
	branch := ""
	for _, q := range questions {
		v, ok := body.Answers[q.Key]
		if !ok {
			continue
		}
		clean[q.Key] = normalizeAnswer(q, v)
		if q.IsSelector {
			branch = branchForLabel(q, clean[q.Key])
		}
	}

	if err := a.store.SaveSurvey(r.Context(), user.ID, domain.SurveyInProgress, branch, "", clean, nil); err != nil {
		a.logger.Error("api survey progress: save", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": string(domain.SurveyInProgress)})
}

func normalizeAnswer(q domain.SurveyQuestion, v any) any {
	if q.Kind == domain.SurveyKindMulti {
		out := []string{}
		if arr, ok := v.([]any); ok {
			for _, x := range arr {
				if s, ok := x.(string); ok && s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func branchForLabel(q domain.SurveyQuestion, v any) string {
	label, _ := v.(string)
	for _, o := range q.Options {
		if o.Label == label {
			return o.Branch
		}
	}
	return ""
}

func surveyComplete(tmpl *survey.Template, branch string, answers map[string]any) bool {
	seq := tmpl.Sequence(branch)
	if len(seq) == 0 {
		return false
	}
	for _, key := range seq {
		v, ok := answers[key]
		if !ok {
			return false
		}
		switch val := v.(type) {
		case string:
			if val == "" {
				return false
			}
		case []string:
			if len(val) == 0 {
				return false
			}
		}
	}
	return true
}
