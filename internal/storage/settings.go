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

// GetSetting возвращает значение настройки. ErrNotFound — если ключа нет.
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

// SetSetting сохраняет значение настройки (upsert).
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	const q = `
		INSERT INTO app_settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`
	if _, err := s.db.ExecContext(ctx, q, key, value); err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// SettingFirst30Promo — ключ переключателя акции «первых 30».
const SettingFirst30Promo = "first30_promo_enabled"

// SettingSubscriptionReminderDays — за сколько дней до окончания подписки напоминать.
const SettingSubscriptionReminderDays = "subscription_reminder_days"

// First30PromoEnabled — включён ли переключатель акции в админке (по умолчанию true).
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

// SubscriptionReminderDays возвращает настроенные периоды напоминаний.
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

// ParseReminderDays разбирает строку вида "14,7,3,1" в уникальный список дней.
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

// FormatReminderDays сериализует периоды для сохранения в settings.
func FormatReminderDays(days []int) string {
	parts := make([]string, 0, len(days))
	for _, day := range days {
		parts = append(parts, strconv.Itoa(day))
	}
	return strings.Join(parts, ",")
}
