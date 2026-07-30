package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"therunish/internal/auth"
	"therunish/internal/domain"
	"therunish/internal/storage"
)

type planResponse struct {
	WeekStart string                `json:"week_start"`
	Label     string                `json:"label"`
	Weeks     []string              `json:"weeks"`
	Groups    []domain.PlanGroup    `json:"groups"`
	Materials []domain.PlanMaterial `json:"materials"`
}

// APIPlan отдаёт недельный план тренировок. Доступен только пользователям
// с активной подпиской: план — часть платной подписки.
func (a *App) APIPlan(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	subs, err := a.store.ListActiveSubsByUser(r.Context(), user.ID)
	if err != nil {
		a.logger.Error("plan: list active subs", "err", err, "user_id", user.ID)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if len(subs) == 0 {
		writeJSONError(w, http.StatusForbidden, "subscription_required")
		return
	}

	weeks, err := a.store.ListPublishedWeeks(r.Context())
	if err != nil {
		a.logger.Error("plan: list weeks", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var plan domain.TrainingPlan
	if week := strings.TrimSpace(r.URL.Query().Get("week")); week != "" {
		plan, err = a.store.GetPublishedPlanByWeek(r.Context(), week)
	} else {
		// По умолчанию — текущая неделя, а если её ещё не опубликовали, последняя доступная.
		plan, err = a.store.GetLatestPublishedPlan(r.Context(), mondayOf(time.Now()).Format(planDateLayout))
		if errors.Is(err, storage.ErrNotFound) && len(weeks) > 0 {
			plan, err = a.store.GetPublishedPlanByWeek(r.Context(), weeks[0])
		}
	}
	if errors.Is(err, storage.ErrNotFound) {
		writeJSON(w, http.StatusOK, planResponse{Weeks: weeks, Groups: []domain.PlanGroup{}, Materials: []domain.PlanMaterial{}})
		return
	}
	if err != nil {
		a.logger.Error("plan: get plan", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if plan.Groups == nil {
		plan.Groups = []domain.PlanGroup{}
	}
	if plan.Materials == nil {
		plan.Materials = []domain.PlanMaterial{}
	}

	writeJSON(w, http.StatusOK, planResponse{
		WeekStart: plan.WeekStart,
		Label:     weekLabel(plan.WeekStart),
		Weeks:     weeks,
		Groups:    plan.Groups,
		Materials: plan.Materials,
	})
}
