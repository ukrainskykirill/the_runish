// Package botworker — фоновые задачи клуба: Telegram-бот (поллинг, логин, анкета,
// телефон), напоминания о подписке, авто-истечение подписок и GetState-подстраховка
// зависших платежей. Может запускаться как отдельный процесс (cmd/worker) или
// горутиной внутри web (RUN_WORKER=1) — в обоих случаях это один и тот же код.
package botworker

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"therunish/internal/auth"
	"therunish/internal/config"
	"therunish/internal/domain"
	"therunish/internal/payment"
	"therunish/internal/storage"
	"therunish/internal/telegram"
)

// Worker — фоновый воркер.
type Worker struct {
	store    *storage.Store
	provider payment.PaymentProvider
	bot      *telegram.Client
	cfg      config.Config
	logger   *slog.Logger
}

// New создаёт воркер с заданными зависимостями.
func New(store *storage.Store, provider payment.PaymentProvider, bot *telegram.Client, cfg config.Config, logger *slog.Logger) *Worker {
	return &Worker{store: store, provider: provider, bot: bot, cfg: cfg, logger: logger}
}

// Run запускает три тикера: напоминания (раз в сутки), истечение (раз в час),
// GetState-подстраховка (раз в 10 минут). Плюс — поллинг апдейтов от Telegram-бота.
func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("worker started")

	reminderTicker := time.NewTicker(24 * time.Hour)
	defer reminderTicker.Stop()

	expireTicker := time.NewTicker(1 * time.Hour)
	defer expireTicker.Stop()

	getStateTicker := time.NewTicker(10 * time.Minute)
	defer getStateTicker.Stop()

	if w.cfg.BotToken != "" {
		go w.pollTelegramUpdates(ctx)
	}

	// Сразу прогоняем при старте.
	w.runReminders(ctx)
	w.runExpire(ctx)
	w.runGetState(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("worker stopped")
			return
		case <-reminderTicker.C:
			w.runReminders(ctx)
		case <-expireTicker.C:
			w.runExpire(ctx)
		case <-getStateTicker.C:
			w.runGetState(ctx)
		}
	}
}

// runReminders — отправка напоминаний за настроенные периоды до истечения подписки.
func (w *Worker) runReminders(ctx context.Context) {
	days, err := w.store.SubscriptionReminderDays(ctx)
	if err != nil {
		w.logger.Error("load reminder days", "err", err)
		days = []int{7, 3, 1}
	}

	subs, err := w.store.ListReminderCandidates(ctx, days)
	if err != nil {
		w.logger.Error("list reminder candidates", "err", err)
		return
	}
	if len(subs) == 0 {
		return
	}

	w.logger.Info("processing reminders", "count", len(subs))

	for _, sub := range subs {
		// Получаем telegram_id юзера.
		user, err := w.store.GetUserByID(ctx, sub.UserID)
		if err != nil {
			w.logger.Error("get user for reminder", "err", err, "user_id", sub.UserID)
			continue
		}

		// Получаем название услуги.
		svc, err := w.store.GetService(ctx, sub.ServiceID)
		if err != nil {
			w.logger.Error("get service for reminder", "err", err, "service_id", sub.ServiceID)
			continue
		}

		text := telegram.ReminderText(svc.Title, sub.DaysBefore, w.cfg.BaseURL+"/runners")
		if err := w.bot.SendMessage(ctx, user.TelegramID, text); err != nil {
			var apiErr *telegram.APIError
			if errors.As(err, &apiErr) && apiErr.IsForbidden() {
				// Юзер заблокировал бота — снимаем флаг.
				_ = w.store.SetBotDialogOpen(ctx, user.ID, false)
				w.logger.Info("bot blocked by user", "user_id", user.ID)
			} else {
				w.logger.Error("send reminder", "err", err, "user_id", user.ID)
			}
			continue
		}

		if err := w.store.MarkReminded(ctx, sub.ID, sub.DaysBefore); err != nil {
			w.logger.Error("mark reminded", "err", err, "sub_id", sub.ID)
		}
	}
}

// runExpire — батч-перевод просроченных подписок в expired.
func (w *Worker) runExpire(ctx context.Context) {
	n, err := w.store.ExpireOverdue(ctx)
	if err != nil {
		w.logger.Error("expire overdue", "err", err)
		return
	}
	if n > 0 {
		w.logger.Info("expired subscriptions", "count", n)
	}
}

// runGetState — подстраховка для зависших pending-платежей.
func (w *Worker) runGetState(ctx context.Context) {
	// Платежи в статусе pending, не обновлённые последние 30 минут.
	payments, err := w.store.ListPendingPaymentsOlderThan(ctx, 30*time.Minute)
	if err != nil {
		w.logger.Error("list pending payments", "err", err)
		return
	}
	if len(payments) == 0 {
		return
	}

	w.logger.Info("checking stuck payments", "count", len(payments))

	for _, p := range payments {
		if p.TBankPaymentID == "" {
			continue
		}

		status, err := w.provider.GetState(ctx, p.TBankPaymentID)
		if err != nil {
			w.logger.Error("getstate", "err", err, "payment_id", p.ID)
			continue
		}

		mapped := payment.MapStatus(status)
		switch mapped {
		case "confirmed":
			if err := w.store.ActivateSubscriptionTx(ctx, p.TBankOrderID, p.TBankPaymentID, status); err != nil {
				w.logger.Error("activate from getstate", "err", err, "order_id", p.TBankOrderID)
			}
			w.logger.Info("payment confirmed via getstate", "order_id", p.TBankOrderID)
		case "rejected":
			if err := w.store.RejectPaymentTx(ctx, p.TBankOrderID, p.TBankPaymentID, status, p.ErrorCode); err != nil {
				w.logger.Error("reject from getstate", "err", err, "order_id", p.TBankOrderID)
			}
		}
	}
}

// pollTelegramUpdates — короткий поллинг getUpdates. Обрабатывает /start <nonce>
// от Telegram Login deep-link фолбэка (см. AuthTelegram).
func (w *Worker) pollTelegramUpdates(ctx context.Context) {
	// На случай, если ранее был установлен webhook — getUpdates с ним конфликтует.
	if err := w.bot.DeleteWebhook(ctx); err != nil {
		w.logger.Error("delete telegram webhook", "err", err)
	}

	// Меню команд бота (кнопка «/» в клиенте).
	if err := w.bot.SetMyCommands(ctx, []telegram.BotCommand{
		{Command: "start", Description: "Подключить уведомления и вход на сайт"},
		{Command: "phone", Description: "Поделиться номером телефона (для оплаты и чека)"},
	}); err != nil {
		w.logger.Warn("set bot commands", "err", err)
	}

	var offset int64
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			updates, err := w.bot.GetUpdates(ctx, offset)
			if err != nil {
				w.logger.Error("get telegram updates", "err", err)
				continue
			}
			for _, u := range updates {
				offset = u.UpdateID + 1
				switch {
				case u.CallbackQuery != nil:
					w.handleCallbackQuery(ctx, u)
				case u.Message != nil:
					w.handleTelegramUpdate(ctx, u)
				}
			}
		}
	}
}

// handleTelegramUpdate обрабатывает /start <nonce>: создаёт/обновляет юзера,
// помечает диалог с ботом открытым и подтверждает nonce для deep-link логина.
func (w *Worker) handleTelegramUpdate(ctx context.Context, u telegram.Update) {
	if u.Message == nil {
		return
	}

	// Пользователь поделился телефоном (кнопка request_contact).
	if u.Message.Contact != nil {
		w.handleContact(ctx, u)
		return
	}

	text := strings.TrimSpace(u.Message.Text)

	// Команда /phone — запросить номер телефона (для записи и чека при оплате).
	if text == "/phone" || strings.HasPrefix(text, "/phone@") {
		w.handlePhoneCommand(ctx, u.Message.From.ID, u.Message.Chat.ID)
		return
	}

	if !strings.HasPrefix(text, "/start") {
		// Не команда — возможно, это ответ на текстовый шаг онбординг-анкеты.
		w.handleSurveyText(ctx, u.Message.From.ID, u.Message.Chat.ID, text)
		return
	}
	nonce := strings.TrimSpace(strings.TrimPrefix(text, "/start"))

	from := u.Message.From
	fullName := strings.TrimSpace(from.FirstName + " " + from.LastName)
	user, err := w.store.UpsertUser(ctx, &domain.User{
		TelegramID: from.ID,
		Username:   from.Username,
		FullName:   fullName,
	})
	if err != nil {
		w.logger.Error("upsert user from telegram", "err", err, "tg_id", from.ID)
		return
	}

	if err := w.store.SetBotDialogOpen(ctx, user.ID, true); err != nil {
		w.logger.Error("set bot dialog open", "err", err, "user_id", user.ID)
	}

	// Диплинк с сайта именно за телефоном (?start=phone): сразу просим контакт,
	// без сообщения про «возвращайтесь на сайт».
	if nonce == "phone" {
		w.askForPhone(ctx, user, u.Message.Chat.ID)
		return
	}

	reply := "✅ Готово! Возвращайтесь на сайт — вход произойдёт автоматически."
	if nonce != "" {
		if err := w.store.ConfirmLoginRequest(ctx, nonce, user.ID); err != nil && !errors.Is(err, storage.ErrNotFound) {
			w.logger.Error("confirm login request", "err", err, "nonce", nonce)
		}
	} else {
		reply = "✅ Вы подключили уведомления The Runish."
	}

	if err := w.bot.SendMessage(ctx, u.Message.Chat.ID, reply); err != nil {
		w.logger.Warn("send telegram reply", "err", err, "chat_id", u.Message.Chat.ID)
	}

	w.logger.Info("telegram /start handled", "user_id", user.ID, "tg_id", user.TelegramID)

	// Новому юзеру — онбординг-анкета (один раз). Существующих и завершивших не трогает.
	w.maybeStartOrContinueSurvey(ctx, user.ID, u.Message.Chat.ID)

	// Нет телефона — просим поделиться (нужен для записи и чека при оплате).
	if user.Phone == "" {
		w.askForPhone(ctx, user, u.Message.Chat.ID)
	}
}

// handlePhoneCommand обрабатывает команду /phone: просит поделиться номером, а если
// телефон уже сохранён — сообщает об этом. Для незнакомого юзера предлагает /start.
func (w *Worker) handlePhoneCommand(ctx context.Context, tgID, chatID int64) {
	user, err := w.store.GetUserByTelegramID(ctx, tgID)
	if errors.Is(err, storage.ErrNotFound) {
		_ = w.bot.SendMessage(ctx, chatID, "Сначала нажми /start, чтобы подключиться, — потом сможешь поделиться номером.")
		return
	}
	if err != nil {
		w.logger.Error("phone cmd: get user", "err", err, "tg_id", tgID)
		return
	}
	w.askForPhone(ctx, user, chatID)
}

// askForPhone отправляет кнопку «Поделиться телефоном». Если телефон уже есть —
// просто подтверждает это (и убирает клавиатуру).
func (w *Worker) askForPhone(ctx context.Context, user domain.User, chatID int64) {
	if user.Phone != "" {
		_ = w.bot.SendMessageRemoveKeyboard(ctx, chatID, "✅ Телефон уже сохранён: "+user.Phone+". Можно оплачивать на сайте.")
		return
	}
	if err := w.bot.SendMessageWithContactButton(ctx, chatID,
		"📱 Чтобы записываться и оплачивать, поделись номером телефона — он нужен для чека."); err != nil {
		w.logger.Warn("send contact button", "err", err, "chat_id", chatID)
	}
}

// handleContact сохраняет телефон, которым поделился пользователь (кнопка request_contact).
// Берём только СВОЙ контакт (чужой номер залить нельзя).
func (w *Worker) handleContact(ctx context.Context, u telegram.Update) {
	c := u.Message.Contact
	from := u.Message.From
	chatID := u.Message.Chat.ID

	if c.UserID != 0 && c.UserID != from.ID {
		_ = w.bot.SendMessage(ctx, chatID, "Поделитесь, пожалуйста, своим номером (кнопкой ниже).")
		return
	}
	phone, ok := auth.NormalizeRuPhone(c.PhoneNumber)
	if !ok {
		_ = w.bot.SendMessage(ctx, chatID, "Не удалось распознать номер — нужен российский телефон.")
		return
	}

	user, err := w.store.GetUserByTelegramID(ctx, from.ID)
	if err != nil {
		w.logger.Error("contact: get user", "err", err, "tg_id", from.ID)
		return
	}
	if err := w.store.SetUserPhone(ctx, user.ID, phone); err != nil {
		w.logger.Error("contact: set phone", "err", err, "user_id", user.ID)
		return
	}
	_ = w.bot.SendMessageRemoveKeyboard(ctx, chatID, "✅ Спасибо! Телефон сохранён — теперь можно оплачивать на сайте.")
	w.logger.Info("phone saved via contact", "user_id", user.ID)
}
