package botworker

import (
	"context"
	"errors"
	"strings"
	"time"

	"therunish/internal/auth"
	"therunish/internal/domain"
	"therunish/internal/observability"
	"therunish/internal/storage"
	"therunish/internal/telegram"
)

func (w *Worker) pollTelegramUpdates(ctx context.Context) {
	if err := w.bot.DeleteWebhook(ctx); err != nil {
		w.logger.Error("delete telegram webhook", "err", err)
	}

	if err := w.bot.SetMyCommands(ctx, []telegram.BotCommand{
		{Command: "start", Description: "Подключить уведомления и вход на сайт"},
		{Command: "phone", Description: "Поделиться номером телефона (для оплаты и чека)"},
	}); err != nil {
		w.logger.Warn("set bot commands", "err", err)
	}

	if w.cfg.BaseURL != "" {
		if err := w.bot.SetChatMenuButton(ctx, w.cfg.BaseURL+"/app", "Открыть клуб"); err != nil {
			w.logger.Warn("set chat menu button", "err", err)
		}
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

func (w *Worker) handleTelegramUpdate(ctx context.Context, u telegram.Update) {
	if u.Message == nil {
		return
	}

	if u.Message.Contact != nil {
		w.handleContact(ctx, u)
		return
	}

	text := strings.TrimSpace(u.Message.Text)

	if text == "/phone" || strings.HasPrefix(text, "/phone@") {
		w.handlePhoneCommand(ctx, u.Message.From.ID, u.Message.Chat.ID)
		return
	}

	if !strings.HasPrefix(text, "/start") {
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
		observability.Alert(ctx, w.logger, "Регистрация не удалась (/start в боте)", err, "tg_id", from.ID)
		return
	}

	if err := w.store.SetBotDialogOpen(ctx, user.ID, true); err != nil {
		w.logger.Error("set bot dialog open", "err", err, "user_id", user.ID)
	}

	if nonce == "phone" {
		w.askForPhone(ctx, user, u.Message.Chat.ID)
		return
	}

	if nonce != "" {
		err := w.store.ConfirmLoginRequest(ctx, nonce, user.ID)
		if errors.Is(err, storage.ErrNotFound) {
			return
		}
		if err != nil {
			w.logger.Error("confirm login request", "err", err, "nonce", nonce)
			return
		}
		reply := renderTemplate(w.store.MessageTemplate(ctx, storage.SettingTmplLoginDone), map[string]string{"site": w.cfg.BaseURL})
		if err := w.bot.SendMessage(ctx, u.Message.Chat.ID, reply); err != nil {
			w.logger.Warn("send telegram reply", "err", err, "chat_id", u.Message.Chat.ID)
		}
		w.logger.Info("telegram login via bot", "user_id", user.ID, "tg_id", user.TelegramID)
		return
	}

	reply := renderTemplate(w.store.MessageTemplate(ctx, storage.SettingTmplWelcome), map[string]string{"site": w.cfg.BaseURL})
	if err := w.bot.SendMessage(ctx, u.Message.Chat.ID, reply); err != nil {
		w.logger.Warn("send telegram reply", "err", err, "chat_id", u.Message.Chat.ID)
	}
	w.logger.Info("telegram /start handled", "user_id", user.ID, "tg_id", user.TelegramID)

	if user.Phone == "" {
		w.askForPhone(ctx, user, u.Message.Chat.ID)
	}
}

func (w *Worker) handlePhoneCommand(ctx context.Context, tgID, chatID int64) {
	user, err := w.store.GetUserByTelegramID(ctx, tgID)
	if errors.Is(err, storage.ErrNotFound) {
		_ = w.bot.SendMessage(ctx, chatID, w.store.MessageTemplate(ctx, storage.SettingTmplPhoneNeedStart))
		return
	}
	if err != nil {
		w.logger.Error("phone cmd: get user", "err", err, "tg_id", tgID)
		return
	}
	w.askForPhone(ctx, user, chatID)
}

func (w *Worker) askForPhone(ctx context.Context, user domain.User, chatID int64) {
	if user.Phone != "" {
		text := renderTemplate(w.store.MessageTemplate(ctx, storage.SettingTmplPhoneHave), map[string]string{"phone": user.Phone})
		_ = w.bot.SendMessageRemoveKeyboard(ctx, chatID, text)
		return
	}
	if err := w.bot.SendMessageWithContactButton(ctx, chatID,
		w.store.MessageTemplate(ctx, storage.SettingTmplPhoneAsk)); err != nil {
		w.logger.Warn("send contact button", "err", err, "chat_id", chatID)
	}
}

func (w *Worker) handleContact(ctx context.Context, u telegram.Update) {
	c := u.Message.Contact
	from := u.Message.From
	chatID := u.Message.Chat.ID

	if c.UserID != 0 && c.UserID != from.ID {
		_ = w.bot.SendMessage(ctx, chatID, w.store.MessageTemplate(ctx, storage.SettingTmplPhoneWrongUser))
		return
	}
	phone, ok := auth.NormalizeRuPhone(c.PhoneNumber)
	if !ok {
		_ = w.bot.SendMessage(ctx, chatID, w.store.MessageTemplate(ctx, storage.SettingTmplPhoneBad))
		return
	}

	// UpsertUser (а не GetUserByTelegramID) гарантирует наличие строки пользователя:
	// если её почему-то нет, создаём, а не выходим молча. Телефон сохраняем всегда —
	// повторный «поделиться» каждый раз перезаписывает номер.
	fullName := strings.TrimSpace(from.FirstName + " " + from.LastName)
	user, err := w.store.UpsertUser(ctx, &domain.User{
		TelegramID: from.ID,
		Username:   from.Username,
		FullName:   fullName,
	})
	if err != nil {
		observability.Alert(ctx, w.logger, "Не удалось сохранить телефон (contact в боте)", err, "tg_id", from.ID)
		return
	}
	if err := w.store.SetUserPhone(ctx, user.ID, phone); err != nil {
		observability.Alert(ctx, w.logger, "Не удалось сохранить телефон (contact в боте)", err, "user_id", user.ID)
		return
	}
	_ = w.bot.SendMessageRemoveKeyboard(ctx, chatID, w.store.MessageTemplate(ctx, storage.SettingTmplPhoneSaved))
	w.logger.Info("phone saved via contact", "user_id", user.ID)
}
