package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"therunish/internal/storage"
	"therunish/internal/telegram"
)

type templateView struct {
	Key          string
	Label        string
	Placeholders string
	Value        string
}

func (a *App) AdminSettingsPage(w http.ResponseWriter, r *http.Request) {
	enabled, err := a.store.First30PromoEnabled(r.Context())
	if err != nil {
		a.logger.Error("settings: promo enabled", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	count, err := a.store.CountEntryFeePaid(r.Context())
	if err != nil {
		a.logger.Error("settings: count entry", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	subHours, err := a.store.SubscriptionReminderHours(r.Context())
	if err != nil {
		a.logger.Error("settings: reminder hours", "err", err)
		subHours = []int{168, 72, 24}
	}
	trainingHours, err := a.store.TrainingReminderHours(r.Context())
	if err != nil {
		trainingHours = 24
	}
	quietFrom, quietTo := a.store.QuietWindow(r.Context())

	var templates []templateView
	for _, t := range storage.MessageTemplates() {
		templates = append(templates, templateView{
			Key:          t.Key,
			Label:        t.Label,
			Placeholders: t.Placeholders,
			Value:        a.store.MessageTemplate(r.Context(), t.Key),
		})
	}

	data := struct {
		PageData
		PromoEnabled     bool
		PaidCount        int
		PromoActive      bool
		SubReminderHours string
		TrainingHours    int
		QuietFrom        int
		QuietTo          int
		Templates        []templateView
		Saved            bool
		NotifySent       string
		NotifyFailed     string
	}{
		PageData:         PageData{BotUsername: a.cfg.BotUsername},
		PromoEnabled:     enabled,
		PaidCount:        count,
		PromoActive:      enabled && count < 30,
		SubReminderHours: storage.FormatReminderDays(subHours),
		TrainingHours:    trainingHours,
		QuietFrom:        quietFrom,
		QuietTo:          quietTo,
		Templates:        templates,
		Saved:            r.URL.Query().Get("saved") == "1",
		NotifySent:       r.URL.Query().Get("notify_sent"),
		NotifyFailed:     r.URL.Query().Get("notify_failed"),
	}
	if err := a.renderer.Render(w, "admin_settings", data); err != nil {
		a.logger.Error("render admin_settings", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (a *App) AdminSettingsSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	value := "false"
	if r.FormValue("first30_promo_enabled") == "on" {
		value = "true"
	}
	if err := a.store.SetSetting(r.Context(), storage.SettingFirst30Promo, value); err != nil {
		a.logger.Error("settings: save", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/settings?saved=1", http.StatusSeeOther)
}

func (a *App) AdminReminderSettingsSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	subHours, err := storage.ParseReminderHours(r.FormValue("subscription_reminder_hours"))
	if err != nil {
		http.Error(w, "Некорректные часы напоминаний о подписке", http.StatusBadRequest)
		return
	}

	trainingHours, err := strconv.Atoi(strings.TrimSpace(r.FormValue("training_reminder_hours")))
	if err != nil || trainingHours < 1 || trainingHours > 8760 {
		http.Error(w, "Некорректные часы напоминания о тренировке", http.StatusBadRequest)
		return
	}

	quietFrom, err := storage.ParseHourOfDay(r.FormValue("notify_quiet_from"))
	if err != nil {
		http.Error(w, "Некорректное время начала тихого окна", http.StatusBadRequest)
		return
	}
	quietTo, err := storage.ParseHourOfDay(r.FormValue("notify_quiet_to"))
	if err != nil {
		http.Error(w, "Некорректное время конца тихого окна", http.StatusBadRequest)
		return
	}

	pairs := [][2]string{
		{storage.SettingSubscriptionReminderHours, storage.FormatReminderDays(subHours)},
		{storage.SettingTrainingReminderHours, strconv.Itoa(trainingHours)},
		{storage.SettingQuietFrom, strconv.Itoa(quietFrom)},
		{storage.SettingQuietTo, strconv.Itoa(quietTo)},
	}
	for _, p := range pairs {
		if err := a.store.SetSetting(r.Context(), p[0], p[1]); err != nil {
			a.logger.Error("settings: save reminder", "err", err, "key", p[0])
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/admin/settings?saved=1", http.StatusSeeOther)
}

func (a *App) AdminTemplatesSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	for _, t := range storage.MessageTemplates() {
		value := strings.TrimSpace(r.FormValue(t.Key))
		if err := a.store.SetSetting(r.Context(), t.Key, value); err != nil {
			a.logger.Error("settings: save template", "err", err, "key", t.Key)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/admin/settings?saved=1", http.StatusSeeOther)
}

func (a *App) AdminSendNotificationSubmit(w http.ResponseWriter, r *http.Request) {
	if a.cfg.BotToken == "" {
		http.Error(w, "Telegram-бот не настроен", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(r.FormValue("message"))
	if text == "" {
		http.Error(w, "Текст уведомления обязателен", http.StatusBadRequest)
		return
	}

	audience := r.FormValue("audience")
	var users []notificationRecipient
	switch audience {
	case "all", "active":
		list, err := a.store.ListNotificationUsers(r.Context(), audience)
		if err != nil {
			a.logger.Error("notifications: list users", "err", err, "audience", audience)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		for _, u := range list {
			users = append(users, notificationRecipient{id: u.ID, telegramID: u.TelegramID})
		}
	case "user":
		userID, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("user_id")), 10, 64)
		if err != nil || userID <= 0 {
			http.Error(w, "Некорректный ID пользователя", http.StatusBadRequest)
			return
		}
		u, err := a.store.GetNotificationUser(r.Context(), userID)
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, "Пользователь не найден или не открывал бота", http.StatusBadRequest)
			return
		}
		if err != nil {
			a.logger.Error("notifications: get user", "err", err, "user_id", userID)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		users = append(users, notificationRecipient{id: u.ID, telegramID: u.TelegramID})
	default:
		http.Error(w, "Некорректная аудитория", http.StatusBadRequest)
		return
	}

	sent, failed := a.broadcast(r.Context(), users, text)

	q := url.Values{}
	q.Set("notify_sent", strconv.Itoa(sent))
	q.Set("notify_failed", strconv.Itoa(failed))
	http.Redirect(w, r, "/admin/settings?"+q.Encode(), http.StatusSeeOther)
}

type notificationRecipient struct {
	id         int64
	telegramID int64
}

// broadcast рассылает текст списку получателей. Пользователи, заблокировавшие бота
// (403), помечаются как закрывшие диалог, чтобы не долбиться в них снова.
func (a *App) broadcast(ctx context.Context, users []notificationRecipient, text string) (sent, failed int) {
	bot := telegram.New(a.cfg.BotToken)
	for _, u := range users {
		if err := bot.SendMessage(ctx, u.telegramID, text); err != nil {
			var apiErr *telegram.APIError
			if errors.As(err, &apiErr) && apiErr.IsForbidden() {
				_ = a.store.SetBotDialogOpen(ctx, u.id, false)
			}
			a.logger.Warn("notifications: send failed", "err", err, "user_id", u.id)
			failed++
			continue
		}
		sent++
	}
	return sent, failed
}
