package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"therunish/internal/auth"
	"therunish/internal/domain"
	"therunish/internal/observability"
	"therunish/internal/storage"
)

const loginRequestTTL = 10 * time.Minute

func (a *App) APIAuthTelegramStart(w http.ResponseWriter, r *http.Request) {
	nonce := ""
	if a.cfg.BotUsername != "" {
		n, err := generateNonce()
		if err != nil {
			a.logger.Error("generate login nonce", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		if err := a.store.CreateLoginRequest(r.Context(), n, time.Now().Add(loginRequestTTL)); err != nil {
			a.logger.Error("create login request", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		nonce = n
	}

	writeJSON(w, http.StatusOK, struct {
		BotUsername string `json:"bot_username"`
		Nonce       string `json:"nonce"`
	}{BotUsername: a.cfg.BotUsername, Nonce: nonce})
}

func (a *App) APIAuthTelegramCallback(w http.ResponseWriter, r *http.Request) {
	data, err := auth.ParseTelegramAuth(r.URL.Query())
	if err != nil {
		a.logger.Warn("parse telegram auth", "err", err)
		http.Error(w, "Invalid auth data", http.StatusBadRequest)
		return
	}

	if err := auth.ValidateTelegramAuth(data, a.cfg.BotToken); err != nil {
		a.logger.Warn("validate telegram auth", "err", err, "tg_id", data.ID)
		http.Error(w, "Auth validation failed", http.StatusForbidden)
		return
	}

	fullName := strings.TrimSpace(data.FirstName + " " + data.LastName)
	u := &domain.User{
		TelegramID: data.ID,
		Username:   data.Username,
		FullName:   fullName,
	}
	saved, err := a.store.UpsertUser(r.Context(), u)
	if err != nil {
		observability.Alert(r.Context(), a.logger, "Регистрация не удалась (вход через Telegram на сайте)", err, "tg_id", data.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if _, err := a.sessions.Create(w, saved.ID); err != nil {
		a.logger.Error("create session", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	a.logger.Info("telegram login", "user_id", saved.ID, "tg_id", saved.TelegramID, "username", saved.Username)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) APIAuthTelegramStatus(w http.ResponseWriter, r *http.Request) {
	nonce := r.URL.Query().Get("nonce")
	if nonce == "" {
		writeJSONError(w, http.StatusBadRequest, "missing_nonce")
		return
	}

	status, _, err := a.store.GetLoginRequestStatus(r.Context(), nonce)
	if errors.Is(err, storage.ErrNotFound) {
		status = "expired"
	} else if err != nil {
		a.logger.Error("get login request status", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (a *App) APIAuthTelegramComplete(w http.ResponseWriter, r *http.Request) {
	nonce := r.URL.Query().Get("nonce")
	if nonce == "" {
		writeJSONError(w, http.StatusBadRequest, "missing_nonce")
		return
	}

	status, userID, err := a.store.GetLoginRequestStatus(r.Context(), nonce)
	if err != nil || status != "confirmed" || userID == 0 {
		writeJSONError(w, http.StatusBadRequest, "not_confirmed")
		return
	}

	if _, err := a.sessions.Create(w, userID); err != nil {
		a.logger.Error("create session", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	_ = a.store.DeleteLoginRequest(r.Context(), nonce)

	a.logger.Info("telegram deep-link login", "user_id", userID)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) APIAuthLogout(w http.ResponseWriter, r *http.Request) {
	a.sessions.Destroy(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
