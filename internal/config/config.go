package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr string
	BaseURL  string // без слэша на конце

	DatabaseURL string

	SessionTTL time.Duration

	BotToken    string
	BotUsername string

	TBankTerminalKey string
	TBankPassword    string
	TBankAPIBase     string

	// 54-ФЗ: фискальные параметры чека.
	TBankTaxation      string // система налогообложения: usn_income, osn, …
	TBankTax           string // ставка НДС: none, vat0, vat10, vat20
	TBankPaymentObject string // тип предмета расчёта: service, commodity, …
	TBankPaymentMethod string // способ расчёта: full_payment, full_prepayment

	AdminToken string

	// Админ-логин/пароль (без Telegram).
	AdminLogin    string
	AdminPassword string

	// "tbank" | "mock"
	PaymentProvider string

	// RUN_WORKER=1 — крутить фоновый воркер (бот, напоминания, истечение, GetState)
	// горутиной прямо в web-процессе (один процесс на один core, напр. на Amvera).
	// ВАЖНО: при RUN_WORKER нельзя поднимать >1 реплики web — задвоится поллинг бота
	// и напоминания.
	RunWorker bool

	LogLevel slog.Level
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:         getenv("HTTP_ADDR", ":8080"),
		BaseURL:          strings.TrimRight(getenv("BASE_URL", "http://localhost:8080"), "/"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		BotToken:         os.Getenv("BOT_TOKEN"),
		BotUsername:      os.Getenv("BOT_USERNAME"),
		TBankTerminalKey: os.Getenv("TBANK_TERMINAL_KEY"),
		TBankPassword:    os.Getenv("TBANK_PASSWORD"),
		TBankAPIBase:     getenv("TBANK_API_BASE", "https://securepay.tinkoff.ru/v2"),

		// 54-ФЗ: дефолты — УСН доход, без НДС, оплата услуг.
		TBankTaxation:      getenv("TBANK_TAXATION", "usn_income"),
		TBankTax:           getenv("TBANK_TAX", "none"),
		TBankPaymentObject: getenv("TBANK_PAYMENT_OBJECT", "service"),
		TBankPaymentMethod: getenv("TBANK_PAYMENT_METHOD", "full_payment"),
		AdminToken:         os.Getenv("ADMIN_TOKEN"),
		AdminLogin:         getenv("ADMIN_LOGIN", "test"),
		AdminPassword:      getenv("ADMIN_PASSWORD", "1111"),
		PaymentProvider:    getenv("PAYMENT_PROVIDER", "mock"),
		RunWorker:          getenv("RUN_WORKER", "") == "1",
	}

	ttl, err := time.ParseDuration(getenv("SESSION_TTL", "720h"))
	if err != nil {
		return Config{}, fmt.Errorf("parse SESSION_TTL: %w", err)
	}
	cfg.SessionTTL = ttl

	cfg.LogLevel = slog.LevelInfo
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		var lv slog.Level
		if err := lv.UnmarshalText([]byte(v)); err == nil {
			cfg.LogLevel = lv
		}
	}

	// TODO(prod): вынести BotToken/BotUsername в обязательные после заведения боевого бота.

	required := map[string]string{
		"DATABASE_URL": cfg.DatabaseURL,
	}
	if cfg.PaymentProvider == "tbank" {
		required["TBANK_TERMINAL_KEY"] = cfg.TBankTerminalKey
		required["TBANK_PASSWORD"] = cfg.TBankPassword
	}
	var missing []string
	for name, val := range required {
		if val == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Config{}, errors.New("missing required env vars: " + strings.Join(missing, ", "))
	}

	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
