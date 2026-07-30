package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"therunish/internal/auth"
)

const panelSessionTTL = 12 * time.Hour

func (a *App) AdminLoginPage(w http.ResponseWriter, r *http.Request) {
	a.renderPanelLogin(w, "admin_login", "")
}

func (a *App) AdminLoginSubmit(w http.ResponseWriter, r *http.Request) {
	a.panelLoginSubmit(w, r, panelLoginConfig{
		Template: "admin_login",
		Login:    a.cfg.AdminLogin,
		Password: a.cfg.AdminPassword,
		Role:     auth.RoleAdmin,
		Cookie:   auth.AdminCookieName,
		Redirect: "/admin/services",
	})
}

func (a *App) AdminLogout(w http.ResponseWriter, r *http.Request) {
	a.panelLogout(w, r, auth.AdminCookieName, "/admin/login")
}

type panelLoginConfig struct {
	Template string
	Login    string
	Password string
	Role     string
	Cookie   string
	Redirect string
}

func (a *App) panelLoginSubmit(w http.ResponseWriter, r *http.Request, cfg panelLoginConfig) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	loginOK := subtle.ConstantTimeCompare([]byte(r.FormValue("login")), []byte(cfg.Login)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(r.FormValue("password")), []byte(cfg.Password)) == 1

	if !loginOK || !passOK {
		w.WriteHeader(http.StatusUnauthorized)
		a.renderPanelLogin(w, cfg.Template, "Неверный логин или пароль")
		return
	}

	token, err := generateAdminToken()
	if err != nil {
		a.logger.Error("generate panel token", "err", err, "role", cfg.Role)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(panelSessionTTL)
	if err := a.store.CreateAdminSession(r.Context(), token, cfg.Role, expiresAt); err != nil {
		a.logger.Error("create panel session", "err", err, "role", cfg.Role)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cfg.Cookie,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   a.secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, cfg.Redirect, http.StatusSeeOther)
}

func (a *App) panelLogout(w http.ResponseWriter, r *http.Request, cookieName, loginPath string) {
	c, err := r.Cookie(cookieName)
	if err == nil && c.Value != "" {
		_ = a.store.DeleteAdminSession(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.secureCookies(),
	})
	http.Redirect(w, r, loginPath, http.StatusSeeOther)
}

func (a *App) renderPanelLogin(w http.ResponseWriter, template, errMsg string) {
	data := struct {
		PageData
		Error string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Error:    errMsg,
	}
	_ = a.renderer.Render(w, template, data)
}

func (a *App) secureCookies() bool {
	return strings.HasPrefix(a.cfg.BaseURL, "https://")
}

func generateAdminToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
