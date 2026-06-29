package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"therunish/internal/storage"
)

const (
	CookieName            = "runish_session"
	tokenBytes            = 32 // 256 бит
	contextKeyUser ctxKey = "user_id"
)

type ctxKey string

// Manager управляет сессиями: создание, чтение из cookie.
type Manager struct {
	store  *storage.Store
	ttl    time.Duration
	secure bool // Secure flag для cookie (true только на HTTPS)
}

func NewManager(store *storage.Store, ttl time.Duration, secure bool) *Manager {
	return &Manager{store: store, ttl: ttl, secure: secure}
}

// Create создаёт новую сессию и ставит cookie в ответ.
func (m *Manager) Create(w http.ResponseWriter, userID int64) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	ctx := context.Background()
	expiresAt := time.Now().Add(m.ttl)
	if err := m.store.CreateSession(ctx, token, userID, expiresAt); err != nil {
		return "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return token, nil
}

// UserFromRequest извлекает userID из cookie. Если сессии нет — возвращает 0, nil.
func (m *Manager) UserFromRequest(r *http.Request) (int64, error) {
	token := m.TokenFromRequest(r)
	if token == "" {
		return 0, nil
	}

	userID, err := m.store.GetSession(r.Context(), token)
	if errors.Is(err, storage.ErrNotFound) {
		return 0, nil
	}
	return userID, err
}

// SessionID возвращает токен сессии из cookie (пустая строка если нет).
func (m *Manager) SessionID(r *http.Request) string {
	return m.TokenFromRequest(r)
}

// Destroy удаляет сессию и сбрасывает cookie.
func (m *Manager) Destroy(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(CookieName)
	if err == nil && c.Value != "" {
		_ = m.store.DeleteSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// TokenFromRequest — вспомогательный метод для middleware.
func (m *Manager) TokenFromRequest(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return ""
	}
	return c.Value
}

func generateToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
