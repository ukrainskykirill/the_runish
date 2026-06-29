package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"time"

	"therunish/internal/auth"
)

// AdminLoginPage — форма входа (GET /admin/login).
func (a *App) AdminLoginPage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		PageData
		Error string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
	}
	_ = a.renderer.Render(w, "admin_login", data)
}

// AdminLoginSubmit — проверка логина/пароля (POST /admin/login).
func (a *App) AdminLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	login := r.FormValue("login")
	password := r.FormValue("password")

	// Constant-time comparison.
	loginOK := subtle.ConstantTimeCompare([]byte(login), []byte(a.cfg.AdminLogin)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(a.cfg.AdminPassword)) == 1

	if !loginOK || !passOK {
		data := struct {
			PageData
			Error string
		}{
			PageData: PageData{BotUsername: a.cfg.BotUsername},
			Error:    "Неверный логин или пароль",
		}
		w.WriteHeader(http.StatusUnauthorized)
		_ = a.renderer.Render(w, "admin_login", data)
		return
	}

	// Создаём админ-сессию.
	token, err := generateAdminToken()
	if err != nil {
		a.logger.Error("generate admin token", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(12 * time.Hour)
	ctx := context.Background()
	if err := a.store.CreateAdminSession(ctx, token, expiresAt); err != nil {
		a.logger.Error("create admin session", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     auth.AdminCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/admin/services", http.StatusSeeOther)
}

// AdminLogout — выход (POST /admin/logout).
func (a *App) AdminLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(auth.AdminCookieName)
	if err == nil && c.Value != "" {
		_ = a.store.DeleteAdminSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.AdminCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func generateAdminToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
