package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"therunish/internal/domain"
	"therunish/internal/storage"
)

type surveyGroup struct {
	Title     string
	Questions []domain.SurveyQuestion
}

func groupSurveyQuestions(questions []domain.SurveyQuestion) []surveyGroup {
	groups := []surveyGroup{
		{Title: "Вступление (всем)"},
		{Title: "Ветка: Новичок"},
		{Title: "Ветка: Любитель"},
		{Title: "Ветка: Регуляр"},
		{Title: "Завершение (всем)"},
	}
	idx := func(q domain.SurveyQuestion) int {
		switch q.Phase {
		case domain.SurveyPhaseIntro:
			return 0
		case domain.SurveyPhaseOutro:
			return 4
		case domain.SurveyPhaseBranch:
			switch q.Branch {
			case "novice":
				return 1
			case "casual":
				return 2
			case "regular":
				return 3
			}
		}
		return -1
	}
	for _, q := range questions {
		if i := idx(q); i >= 0 {
			groups[i].Questions = append(groups[i].Questions, q)
		}
	}
	return groups
}

func (a *App) AdminListSurveyPage(w http.ResponseWriter, r *http.Request) {
	questions, err := a.store.ListSurveyQuestions(r.Context(), false)
	if err != nil {
		a.logger.Error("admin list survey", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Groups []surveyGroup
		Total  int
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Groups:   groupSurveyQuestions(questions),
		Total:    len(questions),
	}
	if err := a.renderer.Render(w, "admin_survey", data); err != nil {
		a.logger.Error("render admin_survey", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

type surveyFormView struct {
	PageData
	Question *domain.SurveyQuestion
	Error    string
}

func (a *App) AdminCreateSurveyPage(w http.ResponseWriter, r *http.Request) {
	a.renderSurveyForm(w, &domain.SurveyQuestion{
		Phase:    domain.SurveyPhaseIntro,
		Kind:     domain.SurveyKindSingle,
		IsActive: true,
		Options:  []domain.SurveyOption{},
	}, "")
}

func (a *App) AdminEditSurveyPage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	q, err := a.store.GetSurveyQuestion(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("get survey question", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	a.renderSurveyForm(w, &q, "")
}

func (a *App) renderSurveyForm(w http.ResponseWriter, q *domain.SurveyQuestion, errMsg string) {
	data := surveyFormView{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Question: q,
		Error:    errMsg,
	}
	status := http.StatusOK
	if errMsg != "" {
		status = http.StatusBadRequest
	}
	w.WriteHeader(status)
	if err := a.renderer.Render(w, "admin_survey_form", data); err != nil {
		a.logger.Error("render admin_survey_form", "err", err)
	}
}

func (a *App) AdminCreateSurveySubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	q, errMsg := parseSurveyQuestionForm(r)
	if errMsg != "" {
		a.renderSurveyForm(w, q, errMsg)
		return
	}
	if _, err := a.store.CreateSurveyQuestion(r.Context(), q); err != nil {
		a.logger.Error("create survey question", "err", err)
		a.renderSurveyForm(w, q, "Не удалось сохранить вопрос (возможно, ключ уже занят)")
		return
	}
	http.Redirect(w, r, "/admin/survey", http.StatusSeeOther)
}

func (a *App) AdminUpdateSurveySubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	q, errMsg := parseSurveyQuestionForm(r)
	q.ID = id
	if errMsg != "" {
		a.renderSurveyForm(w, q, errMsg)
		return
	}
	if err := a.store.UpdateSurveyQuestion(r.Context(), q); err != nil {
		a.logger.Error("update survey question", "err", err)
		a.renderSurveyForm(w, q, "Не удалось сохранить вопрос (возможно, ключ уже занят)")
		return
	}
	http.Redirect(w, r, "/admin/survey", http.StatusSeeOther)
}

func (a *App) AdminDeleteSurveySubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := a.store.DeleteSurveyQuestion(r.Context(), id); err != nil && !errors.Is(err, storage.ErrNotFound) {
		a.logger.Error("delete survey question", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/survey", http.StatusSeeOther)
}

func (a *App) AdminMoveSurveySubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	dir := -1
	if r.FormValue("dir") == "down" {
		dir = 1
	}
	if err := a.store.MoveSurveyQuestion(r.Context(), id, dir); err != nil && !errors.Is(err, storage.ErrNotFound) {
		a.logger.Error("move survey question", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/survey", http.StatusSeeOther)
}

func parseSurveyQuestionForm(r *http.Request) (*domain.SurveyQuestion, string) {
	key := strings.TrimSpace(r.FormValue("key"))
	phase := r.FormValue("phase")
	branch := r.FormValue("branch")
	kind := r.FormValue("kind")
	label := strings.TrimSpace(r.FormValue("label"))
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	isSelector := r.FormValue("is_selector") == "on"
	isActive := r.FormValue("is_active") == "on"
	position, _ := strconv.Atoi(r.FormValue("position"))

	options := parseSurveyOptions(r)

	q := &domain.SurveyQuestion{
		Key:        key,
		Phase:      phase,
		Branch:     branch,
		Kind:       kind,
		Label:      label,
		Prompt:     prompt,
		Options:    options,
		IsSelector: isSelector,
		Position:   position,
		IsActive:   isActive,
	}

	if key == "" {
		return q, "Ключ обязателен"
	}
	switch phase {
	case domain.SurveyPhaseIntro, domain.SurveyPhaseOutro:
		q.Branch = ""
	case domain.SurveyPhaseBranch:
		if branch != "novice" && branch != "casual" && branch != "regular" {
			return q, "Для вопроса ветки выберите ветку"
		}
	default:
		return q, "Неверный этап"
	}
	if kind != domain.SurveyKindSingle && kind != domain.SurveyKindMulti && kind != domain.SurveyKindText {
		return q, "Неверный тип вопроса"
	}
	if label == "" {
		return q, "Короткая подпись обязательна"
	}
	if prompt == "" {
		return q, "Текст вопроса обязателен"
	}
	if (kind == domain.SurveyKindSingle || kind == domain.SurveyKindMulti) && len(options) == 0 {
		return q, "Добавьте хотя бы один вариант ответа"
	}
	if kind == domain.SurveyKindText {
		q.Options = []domain.SurveyOption{}
		q.IsSelector = false
	}
	if isSelector {
		if kind != domain.SurveyKindSingle {
			return q, "Вопрос-селектор должен быть с одиночным выбором"
		}
		for _, o := range q.Options {
			if o.Branch == "" {
				return q, "У каждого варианта селектора укажите целевую ветку"
			}
		}
	}
	return q, ""
}

func parseSurveyOptions(r *http.Request) []domain.SurveyOption {
	labels := r.Form["option_label"]
	branches := r.Form["option_branch"]
	out := make([]domain.SurveyOption, 0, len(labels))
	for i, l := range labels {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		opt := domain.SurveyOption{Label: l}
		if i < len(branches) {
			opt.Branch = strings.TrimSpace(branches[i])
		}
		out = append(out, opt)
	}
	return out
}
