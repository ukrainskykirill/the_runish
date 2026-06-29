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
)

// redirectWithError редиректит на карточку юзера с сообщением об ошибке в query.
func redirectWithError(w http.ResponseWriter, r *http.Request, userID int64, msg string) {
	u := "/admin/users/" + strconv.FormatInt(userID, 10) + "?error=" + url.QueryEscape(msg)
	http.Redirect(w, r, u, http.StatusSeeOther)
}

// AdminListUsersPage — список юзеров с поиском (GET /admin/users?q=...).
func (a *App) AdminListUsersPage(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("q")

	users, err := a.store.ListUsers(r.Context(), search)
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

// surveyStatusLabel — человекочитаемый статус анкеты для админки.
func surveyStatusLabel(s domain.SurveyStatus) string {
	switch s {
	case domain.SurveyPending:
		return "не начата"
	case domain.SurveyInProgress:
		return "в процессе"
	case domain.SurveyCompleted:
		return "завершена"
	default:
		return string(s)
	}
}

// AdminUserDetailPage — карточка юзера: подписки + форма ручного добавления (GET /admin/users/{id}).
func (a *App) AdminUserDetailPage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	user, err := a.store.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("admin get user", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	subs, err := a.store.ListSubsByUser(r.Context(), id)
	if err != nil {
		a.logger.Error("admin list subs", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	services, err := a.store.ListServices(r.Context(), true)
	if err != nil {
		a.logger.Error("admin list services", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	payments, err := a.store.ListPaymentsByUser(r.Context(), id, 20)
	if err != nil {
		a.logger.Error("admin list user payments", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Онбординг-анкета (может отсутствовать у юзеров, зарегистрированных до фичи).
	var (
		surveyStatus string
		surveyQA     []survey.QA
	)
	if sv, err := a.store.GetSurvey(r.Context(), id); err == nil {
		surveyStatus = surveyStatusLabel(sv.Status)
		if sv.Status == domain.SurveyCompleted || sv.Status == domain.SurveyInProgress {
			surveyQA = survey.RenderAnswers(sv.Branch, sv.Answers)
		}
	} else if !errors.Is(err, storage.ErrNotFound) {
		a.logger.Error("admin get survey", "err", err)
	}

	// Активная подписка (если есть) — для блока «Статус участника».
	var activeSubUntil *time.Time
	for i := range subs {
		if subs[i].Status == domain.SubStatusActive {
			if activeSubUntil == nil || subs[i].ExpiresAt.After(*activeSubUntil) {
				t := subs[i].ExpiresAt
				activeSubUntil = &t
			}
		}
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
		Target:         user,
		Subscriptions:  subs,
		Services:       services,
		Payments:       payments,
		SurveyStatus:   surveyStatus,
		SurveyQA:       surveyQA,
		ActiveSubUntil: activeSubUntil,
		Error:          r.URL.Query().Get("error"),
	}
	if err := a.renderer.Render(w, "admin_user_detail", data); err != nil {
		a.logger.Error("render admin_user_detail", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminSetEntryFeeSubmit — ручное управление вступительным взносом
// (POST /admin/users/{id}/entry-fee, поле paid=on|off).
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
	if err := a.store.SetEntryFeePaid(r.Context(), id, paid); err != nil {
		a.logger.Error("set entry fee", "err", err, "user_id", id)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// AdminAddSubscriptionSubmit — ручное добавление подписки юзеру (POST /admin/users/{id}/subscriptions).
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
	// Подписка действует до конца указанного дня.
	expiresAt = expiresAt.Add(24*time.Hour - time.Second)

	if err := a.store.CreateSubscriptionAdmin(r.Context(), id, serviceID, expiresAt); err != nil {
		a.logger.Error("admin create subscription", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// AdminEditSubscriptionPage — форма редактирования подписки (GET /admin/users/{id}/subscriptions/{subID}/edit).
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

	sub, err := a.store.GetSubscriptionByID(r.Context(), subID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("admin get subscription", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	services, err := a.store.ListServices(r.Context(), true)
	if err != nil {
		a.logger.Error("admin list services", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Пользователь нужен для шапки формы (хлебные крошки).
	user, err := a.store.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("admin get user", "err", err)
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
		Target:   user,
		Sub:      sub,
		Services: services,
		Error:    r.URL.Query().Get("error"),
	}
	if err := a.renderer.Render(w, "admin_subscription_form", data); err != nil {
		a.logger.Error("render admin_subscription_form", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminUpdateSubscriptionSubmit — сохранить изменения подписки (POST /admin/users/{id}/subscriptions/{subID}).
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
	switch status {
	case domain.SubStatusActive, domain.SubStatusExpired, domain.SubStatusCancelled:
	default:
		u := "/admin/users/" + strconv.FormatInt(id, 10) +
			"/subscriptions/" + strconv.FormatInt(subID, 10) + "/edit?error=" +
			url.QueryEscape("Некорректный статус")
		http.Redirect(w, r, u, http.StatusSeeOther)
		return
	}

	expiresAt, err := time.ParseInLocation("2006-01-02", r.FormValue("expires_at"), time.Local)
	if err != nil {
		u := "/admin/users/" + strconv.FormatInt(id, 10) +
			"/subscriptions/" + strconv.FormatInt(subID, 10) + "/edit?error=" +
			url.QueryEscape("Некорректная дата окончания")
		http.Redirect(w, r, u, http.StatusSeeOther)
		return
	}
	// Подписка действует до конца указанного дня.
	expiresAt = expiresAt.Add(24*time.Hour - time.Second)

	if err := a.store.UpdateSubscription(r.Context(), subID, serviceID, status, expiresAt); err != nil {
		a.logger.Error("admin update subscription", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// AdminExtendSubscriptionSubmit — быстрое продление подписки (POST /admin/users/{id}/subscriptions/{subID}/extend).
// Параметр days валидируется из белого списка: 1, 7, 30, 90, 180, 365.
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

	// Белый список разрешённых значений продления.
	switch days {
	case 1, 7, 14, 30, 90, 180, 365:
	default:
		redirectWithError(w, r, id, "Недопустимое число дней для продления")
		return
	}

	if err := a.store.ExtendSubscription(r.Context(), subID, days); err != nil {
		a.logger.Error("admin extend subscription", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// AdminDeleteSubscriptionSubmit — удаление подписки (POST /admin/users/{id}/subscriptions/{subID}/delete).
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

	// Если удаляемая подписка выдана бандлом — снимаем и вступительный взнос
	// (бандл включал взнос: убираем его целиком).
	if sub, err := a.store.GetSubscriptionByID(r.Context(), subID); err == nil {
		if svc, err := a.store.GetService(r.Context(), sub.ServiceID); err == nil && svc.Kind == domain.KindBundle {
			if err := a.store.SetEntryFeePaid(r.Context(), sub.UserID, false); err != nil {
				a.logger.Error("revoke entry fee on bundle sub delete", "err", err, "user_id", sub.UserID)
			}
		}
	}

	if err := a.store.DeleteSubscription(r.Context(), subID); err != nil {
		a.logger.Error("admin delete subscription", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}
