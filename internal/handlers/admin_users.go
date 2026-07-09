package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"therunish/internal/domain"
	"therunish/internal/storage"
	"therunish/internal/survey"
	"therunish/internal/usecase"
)

func redirectWithError(w http.ResponseWriter, r *http.Request, userID int64, msg string) {
	u := "/admin/users/" + strconv.FormatInt(userID, 10) + "?error=" + url.QueryEscape(msg)
	http.Redirect(w, r, u, http.StatusSeeOther)
}

func (a *App) AdminListUsersPage(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("q")

	users, err := a.adminUsers.List(r.Context(), search)
	if err != nil {
		a.logger.Error("admin list users", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Users  []storage.UserListItem
		Search string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Users:    users,
		Search:   search,
	}
	if err := a.renderer.Render(w, "admin_users", data); err != nil {
		a.logger.Error("render admin_users", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminUserDetailPage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	detail, err := a.adminUsers.Detail(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("admin user detail", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Target         domain.User
		Subscriptions  []domain.Subscription
		Services       []domain.Service
		Payments       []storage.PaymentWithDetails
		SurveyStatus   string
		SurveyQA       []survey.QA
		ActiveSubUntil *time.Time
		Error          string
	}{
		PageData:       PageData{BotUsername: a.cfg.BotUsername},
		Target:         detail.User,
		Subscriptions:  detail.Subscriptions,
		Services:       detail.Services,
		Payments:       detail.Payments,
		SurveyStatus:   detail.SurveyStatus,
		SurveyQA:       detail.SurveyQA,
		ActiveSubUntil: detail.ActiveSubUntil,
		Error:          r.URL.Query().Get("error"),
	}
	if err := a.renderer.Render(w, "admin_user_detail", data); err != nil {
		a.logger.Error("render admin_user_detail", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminSetEntryFeeSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	paid := r.FormValue("paid") == "on"
	if err := a.membership.SetEntryFee(r.Context(), id, paid); err != nil {
		a.logger.Error("set entry fee", "err", err, "user_id", id)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (a *App) AdminAddSubscriptionSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	serviceID, err := strconv.ParseInt(r.FormValue("service_id"), 10, 64)
	if err != nil || serviceID <= 0 {
		redirectWithError(w, r, id, "Выберите услугу")
		return
	}

	expiresAt, err := time.ParseInLocation("2006-01-02", r.FormValue("expires_at"), time.Local)
	if err != nil {
		redirectWithError(w, r, id, "Некорректная дата окончания")
		return
	}
	expiresAt = expiresAt.Add(24*time.Hour - time.Second)

	if err := a.membership.AddSubscription(r.Context(), id, serviceID, expiresAt); err != nil {
		a.logger.Error("admin create subscription", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (a *App) AdminEditSubscriptionPage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	subID, err := strconv.ParseInt(r.PathValue("subID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid sub id", http.StatusBadRequest)
		return
	}

	form, err := a.adminUsers.SubscriptionForm(r.Context(), id, subID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("admin get subscription", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		PageData
		Target   domain.User
		Sub      domain.Subscription
		Services []domain.Service
		Error    string
	}{
		PageData: PageData{BotUsername: a.cfg.BotUsername},
		Target:   form.User,
		Sub:      form.Sub,
		Services: form.Services,
		Error:    r.URL.Query().Get("error"),
	}
	if err := a.renderer.Render(w, "admin_subscription_form", data); err != nil {
		a.logger.Error("render admin_subscription_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminUpdateSubscriptionSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	subID, err := strconv.ParseInt(r.PathValue("subID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid sub id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	serviceID, err := strconv.ParseInt(r.FormValue("service_id"), 10, 64)
	if err != nil || serviceID <= 0 {
		u := "/admin/users/" + strconv.FormatInt(id, 10) +
			"/subscriptions/" + strconv.FormatInt(subID, 10) + "/edit?error=" +
			url.QueryEscape("Выберите услугу")
		http.Redirect(w, r, u, http.StatusSeeOther)
		return
	}

	status := domain.SubscriptionStatus(r.FormValue("status"))

	expiresAt, err := time.ParseInLocation("2006-01-02", r.FormValue("expires_at"), time.Local)
	if err != nil {
		u := "/admin/users/" + strconv.FormatInt(id, 10) +
			"/subscriptions/" + strconv.FormatInt(subID, 10) + "/edit?error=" +
			url.QueryEscape("Некорректная дата окончания")
		http.Redirect(w, r, u, http.StatusSeeOther)
		return
	}
	expiresAt = expiresAt.Add(24*time.Hour - time.Second)

	if err := a.membership.UpdateSubscription(r.Context(), subID, serviceID, status, expiresAt); err != nil {
		if errors.Is(err, usecase.ErrInvalidSubscriptionStatus) {
			u := "/admin/users/" + strconv.FormatInt(id, 10) +
				"/subscriptions/" + strconv.FormatInt(subID, 10) + "/edit?error=" +
				url.QueryEscape("Некорректный статус")
			http.Redirect(w, r, u, http.StatusSeeOther)
			return
		}
		a.logger.Error("admin update subscription", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (a *App) AdminExtendSubscriptionSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	subID, err := strconv.ParseInt(r.PathValue("subID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid sub id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	days, err := strconv.Atoi(r.FormValue("days"))
	if err != nil {
		redirectWithError(w, r, id, "Некорректное число дней")
		return
	}

	if err := a.membership.ExtendSubscription(r.Context(), subID, days); err != nil {
		if errors.Is(err, usecase.ErrInvalidExtensionDays) {
			redirectWithError(w, r, id, "Недопустимое число дней для продления")
			return
		}
		a.logger.Error("admin extend subscription", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (a *App) AdminDeleteSubscriptionSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	subID, err := strconv.ParseInt(r.PathValue("subID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid sub id", http.StatusBadRequest)
		return
	}

	if err := a.membership.DeleteSubscription(r.Context(), subID); err != nil {
		a.logger.Error("admin delete subscription", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}
