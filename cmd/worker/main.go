package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"therunish/internal/botworker"
	"therunish/internal/config"
	"therunish/internal/payment"
	"therunish/internal/storage"
	"therunish/internal/telegram"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load", "err", err)
		os.Exit(1)
	}

	store, err := storage.New(context.Background(), cfg.DatabaseURL, logger)
	if err != nil {
		logger.Error("storage init", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	var provider payment.PaymentProvider
	if cfg.PaymentProvider == "mock" {
		provider = payment.NewMock(cfg.BaseURL + "/payment/mock")
	} else {
		provider = payment.NewTBank(cfg.TBankTerminalKey, cfg.TBankPassword, cfg.TBankAPIBase)
	}

	bot := telegram.New(cfg.BotToken)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	botworker.New(store, provider, bot, cfg, logger).Run(ctx)
}
