package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"therunish/internal/domain"
	"therunish/internal/storage"
)

var timeHHMM = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// trainingGroup — тренировки одного месяца для списка в админке.
type trainingGroup struct {
	Label     string
	Trainings []domain.Training
}

var monthsNom = []string{
	"Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
	"Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь",
}

// trainingPeriodRange — диапазон [from, to) по пресету периода.
func trainingPeriodRange(period string, now time.Time) (string, string) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	const layout = "2006-01-02"
	switch period {
	case "week":
		monday := today.AddDate(0, 0, -(int(today.Weekday())+6)%7)
		return monday.Format(layout), monday.AddDate(0, 0, 7).Format(layout)
	case "month":
		first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
		return first.Format(layout), first.AddDate(0, 1, 0).Format(layout)
	case "upcoming":
		return today.Format(layout), ""
	case "past":
		return "", today.Format(layout)
	default:
		return "", ""
	}
}

func (a *App) AdminListTrainingsPage(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "all"
	}
	from, to := trainingPeriodRange(period, time.Now())
	if period == "custom" {
		from = strings.TrimSpace(r.URL.Query().Get("from"))
		to = strings.TrimSpace(r.URL.Query().Get("to"))
		if to != "" {
			// «по» включительно: сдвигаем верхнюю границу на день вперёд.
			if d, err := time.Parse("2006-01-02", to); err == nil {
				to = d.AddDate(0, 0, 1).Format("2006-01-02")
			}
		}
	}

	all, err := a.store.ListTrainings(r.Context(), true)
	if err != nil {
		a.logger.Error("admin list trainings", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	counts, err := a.store.CountActiveRegistrationsByTraining(r.Context())
	if err != nil {
		a.logger.Error("admin count registrations", "err", err)
		counts = map[int64]int{}
	}

	var filtered []domain.Training
	totalRegs := 0
	for _, t := range all {
		if from != "" && t.TrainingDate < from {
			continue
		}
		if to != "" && t.TrainingDate >= to {
			continue
		}
		filtered = append(filtered, t)
		totalRegs += counts[t.ID]
	}

	// Группировка по месяцу (тренировки уже отсортированы по дате).
	var groups []trainingGroup
	for _, t := range filtered {
		label := t.TrainingDate
		if d, err := time.Parse("2006-01-02", t.TrainingDate); err == nil {
			label = fmt.Sprintf("%s %d", monthsNom[int(d.Month())-1], d.Year())
		}
		if len(groups) > 0 && groups[len(groups)-1].Label == label {
			groups[len(groups)-1].Trainings = append(groups[len(groups)-1].Trainings, t)
			continue
		}
		groups = append(groups, trainingGroup{Label: label, Trainings: []domain.Training{t}})
	}

	data := struct {
		PageData
		Groups    []trainingGroup
		RegCounts map[int64]int
		Period    string
		From      string
		To        string
		Total     int
		TotalRegs int
		Today     string
	}{
		PageData:  PageData{BotUsername: a.cfg.BotUsername},
		Groups:    groups,
		RegCounts: counts,
		Period:    period,
		From:      r.URL.Query().Get("from"),
		To:        r.URL.Query().Get("to"),
		Total:     len(filtered),
		TotalRegs: totalRegs,
		Today:     time.Now().Format("2006-01-02"),
	}
	if err := a.renderer.Render(w, "admin_trainings", data); err != nil {
		a.logger.Error("render admin_trainings", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminTrainingRegistrationsPage(w http.ResponseWriter, r *http.Request) {
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

	regs, err := a.store.ListTrainingRegistrationsAdmin(r.Context(), id)
	if err != nil {
		a.logger.Error("admin list training registrations", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	activeCount := 0
	for _, reg := range regs {
		if reg.Status == "active" {
			activeCount++
		}
	}

	data := struct {
		PageData
		Training      domain.Training
		Registrations []storage.AdminTrainingReg
		ActiveCount   int
	}{
		PageData:      PageData{BotUsername: a.cfg.BotUsername},
		Training:      t,
		Registrations: regs,
		ActiveCount:   activeCount,
	}
	if err := a.renderer.Render(w, "admin_training_registrations", data); err != nil {
		a.logger.Error("render admin_training_registrations", "err", err)
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
