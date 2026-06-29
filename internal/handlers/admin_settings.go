package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"therunish/internal/storage"
	"therunish/internal/telegram"
)

// AdminSettingsPage — настройки клуба (GET /admin/settings).
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
	reminderDays, err := a.store.SubscriptionReminderDays(r.Context())
	if err != nil {
		a.logger.Error("settings: reminder days", "err", err)
		reminderDays = []int{7, 3, 1}
	}

	data := struct {
		PageData
		PromoEnabled     bool
		PaidCount        int
		PromoActive      bool
		ReminderDays     string
		Saved            bool
		NotifySent       string
		NotifyFailed     string
		NotificationText string
	}{
		PageData:     PageData{BotUsername: a.cfg.BotUsername},
		PromoEnabled: enabled,
		PaidCount:    count,
		PromoActive:  enabled && count < 30,
		ReminderDays: storage.FormatReminderDays(reminderDays),
		Saved:        r.URL.Query().Get("saved") == "1",
		NotifySent:   r.URL.Query().Get("notify_sent"),
		NotifyFailed: r.URL.Query().Get("notify_failed"),
	}
	if err := a.renderer.Render(w, "admin_settings", data); err != nil {
		a.logger.Error("render admin_settings", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// AdminSettingsSubmit — сохранение настроек (POST /admin/settings).
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
	http.Redirect(w, r, "/admin/settings?saved=promo", http.StatusSeeOther)
}

// AdminReminderSettingsSubmit — сохранение периодов напоминаний о подписке.
func (a *App) AdminReminderSettingsSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	reminderDays, err := storage.ParseReminderDays(r.FormValue("subscription_reminder_days"))
	if err != nil {
		http.Error(w, "Некорректные периоды напоминаний", http.StatusBadRequest)
		return
	}
	if err := a.store.SetSetting(r.Context(), storage.SettingSubscriptionReminderDays, storage.FormatReminderDays(reminderDays)); err != nil {
		a.logger.Error("settings: save reminder days", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/settings?saved=reminders", http.StatusSeeOther)
}

// AdminSendNotificationSubmit — ручная Telegram-рассылка из админки.
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

	bot := telegram.New(a.cfg.BotToken)
	var sent, failed int
	for _, u := range users {
		if err := bot.SendMessage(r.Context(), u.telegramID, text); err != nil {
			var apiErr *telegram.APIError
			if errors.As(err, &apiErr) && apiErr.IsForbidden() {
				_ = a.store.SetBotDialogOpen(r.Context(), u.id, false)
			}
			a.logger.Warn("notifications: send failed", "err", err, "user_id", u.id)
			failed++
			continue
		}
		sent++
	}

	q := url.Values{}
	q.Set("notify_sent", strconv.Itoa(sent))
	q.Set("notify_failed", strconv.Itoa(failed))
	http.Redirect(w, r, "/admin/settings?"+q.Encode(), http.StatusSeeOther)
}

type notificationRecipient struct {
	id         int64
	telegramID int64
}
