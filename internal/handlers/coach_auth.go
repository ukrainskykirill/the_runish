package handlers

import (
	"net/http"

	"therunish/internal/auth"
)

func (a *App) CoachLoginPage(w http.ResponseWriter, r *http.Request) {
	a.renderPanelLogin(w, "coach_login", "")
}

func (a *App) CoachLoginSubmit(w http.ResponseWriter, r *http.Request) {
	a.panelLoginSubmit(w, r, panelLoginConfig{
		Template: "coach_login",
		Login:    a.cfg.CoachLogin,
		Password: a.cfg.CoachPassword,
		Role:     auth.RoleCoach,
		Cookie:   auth.CoachCookieName,
		Redirect: "/coach/plans",
	})
}

func (a *App) CoachLogout(w http.ResponseWriter, r *http.Request) {
	a.panelLogout(w, r, auth.CoachCookieName, "/coach/login")
}
