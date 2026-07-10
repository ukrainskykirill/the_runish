package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr string
	BaseURL  string

	DatabaseURL string

	SessionTTL time.Duration

	BotToken    string
	BotUsername string

	VKClientID     string
	VKClientSecret string

	TBankTerminalKey string
	TBankPassword    string
	TBankAPIBase     string

	TBankTaxation      string
	TBankTax           string
	TBankPaymentObject string
	TBankPaymentMethod string

	AdminToken string

	AdminLogin    string
	AdminPassword string

	PaymentProvider string

	RunWorker bool

	SentryDSN         string
	SentryEnvironment string
	AlertBotToken     string
	AlertChatID       int64

	LogLevel slog.Level
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:         getenv("HTTP_ADDR", ":8080"),
		BaseURL:          strings.TrimRight(getenv("BASE_URL", "http://localhost:8080"), "/"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		BotToken:         os.Getenv("BOT_TOKEN"),
		BotUsername:      os.Getenv("BOT_USERNAME"),
		VKClientID:       os.Getenv("VK_CLIENT_ID"),
		VKClientSecret:   os.Getenv("VK_CLIENT_SECRET"),
		TBankTerminalKey: os.Getenv("TBANK_TERMINAL_KEY"),
		TBankPassword:    os.Getenv("TBANK_PASSWORD"),
		TBankAPIBase:     getenv("TBANK_API_BASE", "https://securepay.tinkoff.ru/v2"),

		TBankTaxation:      getenv("TBANK_TAXATION", "usn_income"),
		TBankTax:           getenv("TBANK_TAX", "none"),
		TBankPaymentObject: getenv("TBANK_PAYMENT_OBJECT", "service"),
		TBankPaymentMethod: getenv("TBANK_PAYMENT_METHOD", "full_payment"),
		AdminToken:         os.Getenv("ADMIN_TOKEN"),
		AdminLogin:         getenv("ADMIN_LOGIN", "test"),
		AdminPassword:      getenv("ADMIN_PASSWORD", "1111"),
		PaymentProvider:    getenv("PAYMENT_PROVIDER", "mock"),
		RunWorker:          getenv("RUN_WORKER", "") == "1",
		SentryDSN:          os.Getenv("SENTRY_DSN"),
		SentryEnvironment:  getenv("SENTRY_ENVIRONMENT", "production"),
		AlertBotToken:      os.Getenv("ALERT_BOT_TOKEN"),
	}

	if v := os.Getenv("ALERT_CHAT_ID"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("parse ALERT_CHAT_ID: %w", err)
		}
		cfg.AlertChatID = id
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

func (c Config) VKEnabled() bool {
	return c.VKClientID != "" && c.VKClientSecret != ""
}

func (c Config) VKRedirectURI() string {
	return c.BaseURL + "/api/auth/vk/callback"
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// DumpEnv — временная отладка: печатает все известные переменные окружения в лог.
// Секреты маскируются, у DATABASE_URL прячется только пароль. Убрать после диагностики.
func DumpEnv(logger *slog.Logger) {
	plain := []string{
		"HTTP_ADDR", "BASE_URL", "BOT_USERNAME", "VK_CLIENT_ID", "PAYMENT_PROVIDER", "RUN_WORKER",
		"SESSION_TTL", "TBANK_API_BASE", "TBANK_TAXATION", "TBANK_TAX",
		"TBANK_PAYMENT_OBJECT", "TBANK_PAYMENT_METHOD", "ADMIN_LOGIN",
		"SENTRY_ENVIRONMENT", "ALERT_CHAT_ID", "LOG_LEVEL",
	}
	secret := []string{
		"TBANK_TERMINAL_KEY", "TBANK_PASSWORD", "BOT_TOKEN", "ADMIN_TOKEN",
		"ADMIN_PASSWORD", "SENTRY_DSN", "ALERT_BOT_TOKEN", "VK_CLIENT_SECRET",
	}

	// Всё пишем прямо в текст сообщения (msg), т.к. просмотрщик логов Amvera
	// показывает только msg, а structured-поля slog не отображает.
	for _, k := range plain {
		v := os.Getenv(k)
		if v == "" {
			v = "(empty)"
		}
		logger.Info(fmt.Sprintf("ENVDUMP %s=%s", k, v))
	}
	for _, k := range secret {
		logger.Info(fmt.Sprintf("ENVDUMP %s=%s", k, maskSecret(os.Getenv(k))))
	}
	if v := os.Getenv("DATABASE_URL"); v == "" {
		logger.Info("ENVDUMP DATABASE_URL=MISSING")
	} else {
		logger.Info(fmt.Sprintf("ENVDUMP DATABASE_URL=%s (len=%d)", redactDBURL(v), len(v)))
	}
}

func maskSecret(v string) string {
	if v == "" {
		return "MISSING"
	}
	prefix := v
	if len(prefix) > 4 {
		prefix = prefix[:4]
	}
	return fmt.Sprintf("set len=%d prefix=%s…", len(v), prefix)
}

var dbURLPassRe = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9+.-]*://[^:/@]+:)[^@]*(@)`)

func redactDBURL(v string) string {
	return dbURLPassRe.ReplaceAllString(v, "${1}***${2}")
}
