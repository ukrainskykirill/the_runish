package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"therunish/internal/auth"
	"therunish/internal/domain"
	"therunish/internal/observability"
	"therunish/internal/storage"
)

// APIAuthTelegramWebApp logs a user in from the Telegram Mini App (Telegram.WebApp.initData).
// If the initData carries start_param (a site login nonce opened via startapp=<nonce>),
// it also confirms the matching /api/auth/telegram/{start,status,complete} deep-link flow,
// so opening the mini app from the site logs the site session in too.
func (a *App) APIAuthTelegramWebApp(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData   string `json:"init_data"`
		AllowWrite bool   `json:"allow_write"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.InitData == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	data, err := auth.ValidateWebAppInitData(body.InitData, a.cfg.BotToken)
	if err != nil {
		a.logger.Warn("validate webapp init data", "err", err)
		writeJSONError(w, http.StatusForbidden, "invalid_init_data")
		return
	}

	fullName := strings.TrimSpace(data.User.FirstName + " " + data.User.LastName)
	saved, err := a.store.UpsertUser(r.Context(), &domain.User{
		TelegramID: data.User.ID,
		Username:   data.User.Username,
		FullName:   fullName,
	})
	if err != nil {
		observability.Alert(r.Context(), a.logger, "Регистрация не удалась (вход через Mini App)", err, "tg_id", data.User.ID)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	if _, err := a.sessions.Create(w, saved.ID); err != nil {
		a.logger.Error("create session", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	if data.StartParam != "" {
		if err := a.store.ConfirmLoginRequest(r.Context(), data.StartParam, saved.ID); err != nil && !errors.Is(err, storage.ErrNotFound) {
			a.logger.Error("confirm login request from webapp", "err", err, "nonce", data.StartParam)
		}
	}

	if body.AllowWrite {
		if err := a.store.SetBotDialogOpen(r.Context(), saved.ID, true); err != nil {
			a.logger.Error("set bot dialog open", "err", err, "user_id", saved.ID)
		}
	}

	a.logger.Info("mini app login", "user_id", saved.ID, "tg_id", saved.TelegramID)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// devDefaultTelegramID is the fixed identity used by the /api/auth/dev bypass when the
// caller doesn't pick one, so repeated dev logins land on the same test user/session.
const devDefaultTelegramID = 900000001

// APIAuthDev logs a fake user in without going through Telegram at all, so the Mini App
// (/app) can be clicked through in a plain desktop browser during local development.
// Gated behind cfg.DevAuthEnabled (env DEV_AUTH_BYPASS=1) — off unless explicitly opted
// into, and never set that env var in a deployed environment.
func (a *App) APIAuthDev(w http.ResponseWriter, r *http.Request) {
	if !a.cfg.DevAuthEnabled {
		http.NotFound(w, r)
		return
	}

	var body struct {
		TelegramID int64  `json:"telegram_id"`
		FirstName  string `json:"first_name"`
		Username   string `json:"username"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	tgID := body.TelegramID
	if tgID == 0 {
		tgID = devDefaultTelegramID
	}
	firstName := body.FirstName
	if firstName == "" {
		firstName = "Dev"
	}

	saved, err := a.store.UpsertUser(r.Context(), &domain.User{
		TelegramID: tgID,
		Username:   body.Username,
		FullName:   firstName,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	if _, err := a.sessions.Create(w, saved.ID); err != nil {
		a.logger.Error("create session (dev)", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// APIMeEnableNotifications marks the user reachable for bot notifications
// (equivalent of the bot dialog being open), triggered by Telegram.WebApp.requestWriteAccess().
func (a *App) APIMeEnableNotifications(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "login_required")
		return
	}

	if err := a.store.SetBotDialogOpen(r.Context(), user.ID, true); err != nil {
		a.logger.Error("enable notifications", "err", err, "user_id", user.ID)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
