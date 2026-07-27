package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = $1`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return v, nil
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	const q = `
		INSERT INTO app_settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`
	if _, err := s.db.ExecContext(ctx, q, key, value); err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

const SettingFirst30Promo = "first30_promo_enabled"

const SettingSubscriptionReminderDays = "subscription_reminder_days"

const (
	SettingSubscriptionReminderHours = "subscription_reminder_hours"
	SettingTrainingReminderHours     = "training_reminder_hours"
	SettingTrainingWindowDays        = "training_window_days"
	SettingQuietFrom                 = "notify_quiet_from"
	SettingQuietTo                   = "notify_quiet_to"
)

const (
	SettingTmplSubReminder      = "tmpl_subscription_reminder"
	SettingTmplTrainingReminder = "tmpl_training_reminder"
	SettingTmplTrainingSignedUp = "tmpl_training_signed_up"
	SettingTmplWelcome          = "tmpl_bot_welcome"
	SettingTmplLoginDone        = "tmpl_bot_login_done"
	SettingTmplPhoneAsk         = "tmpl_bot_phone_ask"
	SettingTmplPhoneHave        = "tmpl_bot_phone_have"
	SettingTmplPhoneNeedStart   = "tmpl_bot_phone_need_start"
	SettingTmplPhoneWrongUser   = "tmpl_bot_phone_wrong_user"
	SettingTmplPhoneBad         = "tmpl_bot_phone_bad"
	SettingTmplPhoneSaved       = "tmpl_bot_phone_saved"
)

type MessageTemplateDef struct {
	Key          string
	Label        string
	Default      string
	Placeholders string
}

var messageTemplates = []MessageTemplateDef{
	{SettingTmplSubReminder, "Напоминание об окончании подписки", "⏳ Подписка «{title}» истекает через {left}.\nЧтобы продлить — перейдите по ссылке: {url}", "{title} {left} {hours} {url}"},
	{SettingTmplTrainingReminder, "Напоминание о тренировке", "🏃 Напоминание о тренировке!\n\n«{title}»\n📅 {date} в {time}\n📍 {place}\n\nРасписание и отмена записи: {url}", "{title} {date} {time} {place} {url}"},
	{SettingTmplTrainingSignedUp, "Запись на тренировку подтверждена", "✅ Вы зарегистрированы на тренировку «{title}»\n📅 {date} в {time}\n📍 {place}", "{title} {date} {time} {place}"},
	{SettingTmplWelcome, "Приветствие (/start без входа)", "✅ Вы подключили уведомления The Runish.", "{site}"},
	{SettingTmplLoginDone, "Вход выполнен (/start со ссылкой)", "✅ Готово! Возвращайтесь на сайт — вход произойдёт автоматически.", "{site}"},
	{SettingTmplPhoneAsk, "Запрос номера телефона", "📱 Чтобы записываться и оплачивать, поделись номером телефона — он нужен для чека.", "—"},
	{SettingTmplPhoneHave, "Номер уже сохранён", "✅ Телефон уже сохранён: {phone}. Можно оплачивать на сайте.", "{phone}"},
	{SettingTmplPhoneNeedStart, "Запрос номера без /start", "Сначала нажми /start, чтобы подключиться, — потом сможешь поделиться номером.", "—"},
	{SettingTmplPhoneWrongUser, "Поделились чужим контактом", "Поделитесь, пожалуйста, своим номером (кнопкой ниже).", "—"},
	{SettingTmplPhoneBad, "Номер не распознан", "Не удалось распознать номер — нужен российский телефон.", "—"},
	{SettingTmplPhoneSaved, "Номер сохранён", "✅ Спасибо! Телефон сохранён — теперь можно оплачивать на сайте.", "—"},
}

var botTemplateDefaults = func() map[string]string {
	m := make(map[string]string, len(messageTemplates))
	for _, t := range messageTemplates {
		m[t.Key] = t.Default
	}
	return m
}()

func MessageTemplates() []MessageTemplateDef { return messageTemplates }

func (s *Store) MessageTemplate(ctx context.Context, key string) string {
	def := botTemplateDefaults[key]
	v, err := s.GetSetting(ctx, key)
	if err != nil || strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func (s *Store) SubscriptionReminderHours(ctx context.Context) ([]int, error) {
	v, err := s.GetSetting(ctx, SettingSubscriptionReminderHours)
	if errors.Is(err, ErrNotFound) {
		return []int{168, 72, 24}, nil
	}
	if err != nil {
		return nil, err
	}
	return ParseReminderHours(v)
}

func (s *Store) TrainingReminderHours(ctx context.Context) (int, error) {
	v, err := s.GetSetting(ctx, SettingTrainingReminderHours)
	if errors.Is(err, ErrNotFound) {
		return 24, nil
	}
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 1 || n > 8760 {
		return 24, nil
	}
	return n, nil
}

func (s *Store) TrainingWindowDays(ctx context.Context) int {
	v, err := s.GetSetting(ctx, SettingTrainingWindowDays)
	if err != nil {
		return 28
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 1 || n > 365 {
		return 28
	}
	return n
}

func (s *Store) QuietWindow(ctx context.Context) (from, to int) {
	from = parseHourSetting(ctx, s, SettingQuietFrom, 22)
	to = parseHourSetting(ctx, s, SettingQuietTo, 9)
	return from, to
}

func parseHourSetting(ctx context.Context, s *Store, key string, def int) int {
	v, err := s.GetSetting(ctx, key)
	if err != nil {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 || n > 23 {
		return def
	}
	return n
}

func ParseReminderHours(raw string) ([]int, error) {
	seen := make(map[int]bool)
	var result []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > 8760 {
			return nil, fmt.Errorf("invalid reminder hour %q", part)
		}
		if !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("empty reminder hours")
	}
	sort.Sort(sort.Reverse(sort.IntSlice(result)))
	return result, nil
}

func ParseHourOfDay(raw string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 || n > 23 {
		return 0, fmt.Errorf("invalid hour %q", raw)
	}
	return n, nil
}

func (s *Store) First30PromoEnabled(ctx context.Context) (bool, error) {
	v, err := s.GetSetting(ctx, SettingFirst30Promo)
	if errors.Is(err, ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return v == "true", nil
}

func (s *Store) SubscriptionReminderDays(ctx context.Context) ([]int, error) {
	v, err := s.GetSetting(ctx, SettingSubscriptionReminderDays)
	if errors.Is(err, ErrNotFound) {
		return []int{7, 3, 1}, nil
	}
	if err != nil {
		return nil, err
	}
	return ParseReminderDays(v)
}

func ParseReminderDays(raw string) ([]int, error) {
	seen := make(map[int]bool)
	var result []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n > 365 {
			return nil, fmt.Errorf("invalid reminder day %q", part)
		}
		if !seen[n] {
			seen[n] = true
			result = append(result, n)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("empty reminder days")
	}
	sort.Sort(sort.Reverse(sort.IntSlice(result)))
	return result, nil
}

func FormatReminderDays(days []int) string {
	parts := make([]string, 0, len(days))
	for _, day := range days {
		parts = append(parts, strconv.Itoa(day))
	}
	return strings.Join(parts, ",")
}
