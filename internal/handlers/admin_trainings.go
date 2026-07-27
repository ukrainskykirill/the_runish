package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"therunish/internal/domain"
	"therunish/internal/storage"
)

var timeHHMM = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

func (a *App) AdminListTrainingsPage(w http.ResponseWriter, r *http.Request) {
	trainings, err := a.store.ListTrainings(r.Context(), true)
	if err != nil {
		a.logger.Error("admin list trainings", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Trainings []domain.Training
	}{
		PageData:  PageData{BotUsername: a.cfg.BotUsername},
		Trainings: trainings,
	}
	if err := a.renderer.Render(w, "admin_trainings", data); err != nil {
		a.logger.Error("render admin_trainings", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminCreateTrainingPage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		PageData
		Training *domain.Training
		Error    string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
	}
	if err := a.renderer.Render(w, "admin_training_form", data); err != nil {
		a.logger.Error("render admin_training_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminEditTrainingPage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	t, err := a.store.GetTraining(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("get training", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Training *domain.Training
		Error    string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Training: &t,
	}
	if err := a.renderer.Render(w, "admin_training_form", data); err != nil {
		a.logger.Error("render admin_training_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminCreateTrainingSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	t, errMsg := parseTrainingForm(r)
	if errMsg != "" {
		a.renderTrainingFormError(w, t, errMsg)
		return
	}

	if _, err := a.store.CreateTraining(r.Context(), t); err != nil {
		a.logger.Error("create training", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/trainings", http.StatusSeeOther)
}

func (a *App) AdminUpdateTrainingSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	t, errMsg := parseTrainingForm(r)
	if errMsg != "" {
		t.ID = id
		a.renderTrainingFormError(w, t, errMsg)
		return
	}

	t.ID = id
	if err := a.store.UpdateTraining(r.Context(), t); err != nil {
		a.logger.Error("update training", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/trainings", http.StatusSeeOther)
}

func (a *App) AdminDeleteTrainingSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := a.store.DeleteTraining(r.Context(), id); err != nil {
		a.logger.Error("delete training", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/trainings", http.StatusSeeOther)
}

func (a *App) renderTrainingFormError(w http.ResponseWriter, t *domain.Training, errMsg string) {
	data := struct {
		PageData
		Training *domain.Training
		Error    string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Training: t,
		Error:    errMsg,
	}
	w.WriteHeader(http.StatusBadRequest)
	_ = a.renderer.Render(w, "admin_training_form", data)
}

func parseTrainingForm(r *http.Request) (*domain.Training, string) {
	title := r.FormValue("title")
	description := strings.TrimSpace(r.FormValue("description"))
	place := r.FormValue("place")
	startTime := r.FormValue("start_time")
	trainingDate := strings.TrimSpace(r.FormValue("training_date"))
	kind := r.FormValue("kind")
	sortOrder, _ := strconv.Atoi(r.FormValue("sort_order"))
	isActive := r.FormValue("is_active") == "on"

	if title == "" {
		return nil, "Название обязательно"
	}
	if place == "" {
		return nil, "Место обязательно"
	}
	if kind == "" {
		kind = "regular"
	}
	if kind != "regular" && kind != "sunday_runish" {
		return nil, "Некорректный тип тренировки"
	}
	d, err := time.Parse("2006-01-02", trainingDate)
	if err != nil {
		return nil, "Укажите дату тренировки"
	}
	now := time.Now()
	if d.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)) {
		return nil, "Дата не может быть в прошлом"
	}
	if !timeHHMM.MatchString(startTime) {
		return nil, "Укажите время в формате ЧЧ:ММ"
	}

	var capacity *int
	if cs := r.FormValue("capacity"); cs != "" {
		c, err := strconv.Atoi(cs)
		if err != nil || c <= 0 {
			return nil, "Лимит мест должен быть положительным числом"
		}
		capacity = &c
	}

	var placeURL *string
	if pu := strings.TrimSpace(r.FormValue("place_url")); pu != "" {
		placeURL = &pu
	}

	return &domain.Training{
		Title:        title,
		Description:  description,
		Kind:         kind,
		TrainingDate: trainingDate,
		StartTime:    startTime,
		Place:        place,
		PlaceURL:     placeURL,
		IsActive:     isActive,
		SortOrder:    sortOrder,
		Capacity:     capacity,
	}, ""
}
