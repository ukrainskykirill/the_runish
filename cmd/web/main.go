package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"therunish/internal/auth"
	"therunish/internal/botworker"
	"therunish/internal/buildinfo"
	"therunish/internal/config"
	"therunish/internal/handlers"
	"therunish/internal/observability"
	"therunish/internal/payment"
	"therunish/internal/render"
	"therunish/internal/session"
	"therunish/internal/storage"
	"therunish/internal/telegram"
	"therunish/internal/worker"
)

func main() {
	boot := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	boot.Info("BUILD " + buildinfo.Time)
	config.DumpEnv(boot) // TODO: временная отладка — убрать после диагностики

	cfg, err := config.Load()
	if err != nil {
		boot.Error("config load failed: " + err.Error())
		os.Exit(1)
	}

	logger, flush := observability.Setup(cfg)
	defer flush()
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	store, err := storage.New(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		logger.Error("storage init", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.Migrate(ctx, "migrations"); err != nil {
		logger.Error("migrate", "err", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")

	renderer, err := render.New(
		os.DirFS("web/templates"),
		[]string{"login_layout.html"},
		[]string{"admin_layout.html"},
		[]string{
			"admin_login.html",
			"coach_login.html",
			"admin_dashboard.html",
			"admin_services.html",
			"admin_service_form.html",
			"admin_payments.html",
			"admin_news.html",
			"admin_news_form.html",
			"admin_merch.html",
			"admin_merch_form.html",
			"admin_plans.html",
			"admin_plan_new.html",
			"admin_plan_form.html",
			"admin_trainings.html",
			"admin_training_form.html",
			"admin_training_registrations.html",
			"admin_settings.html",
			"admin_users.html",
			"admin_user_detail.html",
			"admin_subscription_form.html",
			"admin_survey.html",
			"admin_survey_form.html",
		},
	)
	if err != nil {
		logger.Error("parse templates", "err", err)
		os.Exit(1)
	}

	secure := strings.HasPrefix(cfg.BaseURL, "https://")
	sessions := session.NewManager(store, cfg.SessionTTL, secure)

	mw := auth.NewMiddleware(sessions, store)

	var provider payment.PaymentProvider
	if cfg.PaymentProvider == "mock" {
		provider = payment.NewMock(cfg.BaseURL + "/payment/mock")
	} else {
		provider = payment.NewTBank(cfg.TBankTerminalKey, cfg.TBankPassword, cfg.TBankAPIBase)
	}

	if cfg.RunWorker {
		go runEmbeddedWorker(ctx, store, provider, cfg, logger)
	} else {
		poller := worker.NewPendingPaymentPoller(store, provider, logger)
		go poller.Run(ctx)
	}

	app := handlers.NewApp(handlers.Deps{
		Cfg:      cfg,
		Store:    store,
		Sessions: sessions,
		MW:       mw,
		Renderer: renderer,
		Provider: provider,
		Logger:   logger,
	})

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      observability.Recover(logger)(app.Routes()),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("server starting", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "err", err)
	}
	logger.Info("server stopped")
}

const workerLeaderLockKey int64 = 728154001

func runEmbeddedWorker(ctx context.Context, store *storage.Store, provider payment.PaymentProvider, cfg config.Config, logger *slog.Logger) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	standby := false
	for {
		conn, err := store.TryAdvisoryLock(ctx, workerLeaderLockKey)
		switch {
		case err != nil:
			logger.Error("worker: acquire leader lock", "err", err)
		case conn != nil:
			logger.Info("embedded worker started (leader, RUN_WORKER=1)")
			bot := telegram.New(cfg.BotToken)
			botworker.New(store, provider, bot, cfg, logger).Run(ctx)
			_ = conn.Close()
			return
		default:
			if !standby {
				logger.Info("embedded worker on standby (another instance holds the leader lock)")
				standby = true
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
