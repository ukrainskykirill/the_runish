package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"therunish/internal/auth"
	"therunish/internal/domain"
	"therunish/internal/storage"
	"therunish/internal/telegram"
)

var monthsRuGen = [...]string{
	"января", "февраля", "марта", "апреля", "мая", "июня",
	"июля", "августа", "сентября", "октября", "ноября", "декабря",
}

func formatDateRu(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return fmt.Sprintf("%d %s", t.Day(), monthsRuGen[t.Month()-1])
}

func renderMsgTemplate(tmpl string, vars map[string]string) string {
	pairs := make([]string, 0, len(vars)*2)
	for k, v := range vars {
		pairs = append(pairs, "{"+k+"}", v)
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}

func (a *App) APITrainingsUpcoming(w http.ResponseWriter, r *http.Request) {
	var userID int64
	if u := auth.UserFromContext(r.Context()); u != nil {
		userID = u.ID
	}

	occ, err := a.trainings.Upcoming(r.Context(), userID)
	if err != nil {
		a.logger.Error("list upcoming sessions", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if occ == nil {
		occ = []domain.TrainingOccurrence{}
	}
	writeJSON(w, http.StatusOK, struct {
		Occurrences []domain.TrainingOccurrence `json:"occurrences"`
	}{Occurrences: occ})
}

func (a *App) APITrainingRegister(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "login_required")
		return
	}

	var body struct {
		TrainingID  int64  `json:"training_id"`
		SessionDate string `json:"session_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TrainingID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	date, err := time.ParseInLocation("2006-01-02", body.SessionDate, time.UTC)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_date")
		return
	}

	reg, err := a.trainings.Register(r.Context(), user.ID, body.TrainingID, date)
	if err != nil {
		if status, code, ok := trainingErrCode(err); ok {
			writeJSONError(w, status, code)
			return
		}
		a.logger.Error("register for training", "err", err, "user_id", user.ID, "training_id", body.TrainingID)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	go a.notifyTrainingSignedUp(*user, reg)
	writeJSON(w, http.StatusOK, reg)
}

func (a *App) notifyTrainingSignedUp(user domain.User, reg domain.TrainingRegistration) {
	if a.cfg.BotToken == "" || user.TelegramID == 0 || !user.BotDialogOpen {
		return
	}
	ctx := context.Background()
	tmpl := a.store.MessageTemplate(ctx, storage.SettingTmplTrainingSignedUp)
	text := renderMsgTemplate(tmpl, map[string]string{
		"title": reg.Title,
		"date":  formatDateRu(reg.SessionDate),
		"time":  reg.StartTime,
		"place": reg.Place,
	})
	bot := telegram.New(a.cfg.BotToken)
	if err := bot.SendMessage(ctx, user.TelegramID, text); err != nil {
		var apiErr *telegram.APIError
		if errors.As(err, &apiErr) && apiErr.IsForbidden() {
			_ = a.store.SetBotDialogOpen(ctx, user.ID, false)
		} else {
			a.logger.Error("send training signed-up notice", "err", err, "user_id", user.ID)
		}
	}
}

func (a *App) APITrainingCancel(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "login_required")
		return
	}

	var body struct {
		RegistrationID int64 `json:"registration_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RegistrationID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	if err := a.trainings.Cancel(r.Context(), user.ID, body.RegistrationID); err != nil {
		if status, code, ok := trainingErrCode(err); ok {
			writeJSONError(w, status, code)
			return
		}
		a.logger.Error("cancel registration", "err", err, "user_id", user.ID, "reg_id", body.RegistrationID)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func trainingErrCode(err error) (int, string, bool) {
	switch {
	case errors.Is(err, domain.ErrTrainingNoEntryFee):
		return http.StatusForbidden, "entry_fee_required", true
	case errors.Is(err, domain.ErrTrainingNoAccess):
		return http.StatusForbidden, "access_required", true
	case errors.Is(err, domain.ErrTrainingSessionFull):
		return http.StatusConflict, "session_full", true
	case errors.Is(err, domain.ErrTrainingAlreadyRegistered):
		return http.StatusConflict, "already_registered", true
	case errors.Is(err, domain.ErrTrainingSessionCancelled):
		return http.StatusConflict, "session_cancelled", true
	case errors.Is(err, domain.ErrTrainingPastSession):
		return http.StatusConflict, "past_session", true
	case errors.Is(err, storage.ErrNotFound):
		return http.StatusNotFound, "not_found", true
	}
	return 0, "", false
}
