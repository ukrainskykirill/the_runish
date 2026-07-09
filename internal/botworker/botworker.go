package botworker

import (
	"context"
	"log/slog"
	"time"

	"therunish/internal/config"
	"therunish/internal/payment"
	"therunish/internal/storage"
	"therunish/internal/telegram"
)

type Worker struct {
	store    *storage.Store
	provider payment.PaymentProvider
	bot      *telegram.Client
	cfg      config.Config
	logger   *slog.Logger
}

func New(store *storage.Store, provider payment.PaymentProvider, bot *telegram.Client, cfg config.Config, logger *slog.Logger) *Worker {
	return &Worker{store: store, provider: provider, bot: bot, cfg: cfg, logger: logger}
}

func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("worker started")

	reminderTicker := time.NewTicker(1 * time.Hour)
	defer reminderTicker.Stop()

	expireTicker := time.NewTicker(1 * time.Hour)
	defer expireTicker.Stop()

	getStateTicker := time.NewTicker(10 * time.Minute)
	defer getStateTicker.Stop()

	trainingReminderTicker := time.NewTicker(1 * time.Hour)
	defer trainingReminderTicker.Stop()

	if w.cfg.BotToken != "" {
		go w.pollTelegramUpdates(ctx)
	}

	w.runReminders(ctx)
	w.runExpire(ctx)
	w.runGetState(ctx)
	w.runTrainingReminders(ctx)

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
		case <-trainingReminderTicker.C:
			w.runTrainingReminders(ctx)
		}
	}
}
