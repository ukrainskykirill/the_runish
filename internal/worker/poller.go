package worker

import (
	"context"
	"log/slog"
	"time"

	"therunish/internal/domain"
	"therunish/internal/payment"
	"therunish/internal/storage"
)

// PendingPaymentPoller — фоновый воркер, который периодически опрашивает
// T-Bank GetState для платежей, застрявших в pending.
// Подстраховывает webhook: если уведомление потерялось, воркер синхронизирует статус.
type PendingPaymentPoller struct {
	store    *storage.Store
	provider payment.PaymentProvider
	logger   *slog.Logger

	// Интервал опроса БД на наличие stale pending-платежей.
	interval time.Duration
	// Платёж считается stale, если updated_at старше этого threshold.
	staleAfter time.Duration
}

// NewPendingPaymentPoller создаёт воркер с дефолтными таймингами.
func NewPendingPaymentPoller(store *storage.Store, provider payment.PaymentProvider, logger *slog.Logger) *PendingPaymentPoller {
	return &PendingPaymentPoller{
		store:      store,
		provider:   provider,
		logger:     logger,
		interval:   2 * time.Minute,
		staleAfter: 5 * time.Minute,
	}
}

// Run запускает цикл опроса. Блокирует до отмены ctx.
func (p *PendingPaymentPoller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.logger.Info("pending payment poller started",
		"interval", p.interval.String(),
		"stale_after", p.staleAfter.String(),
	)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("pending payment poller stopped")
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

// tick — один цикл опроса: найти stale pending-платежи, опросить GetState.
func (p *PendingPaymentPoller) tick(ctx context.Context) {
	payments, err := p.store.ListPendingPaymentsOlderThan(ctx, p.staleAfter)
	if err != nil {
		p.logger.Error("poller: list pending payments", "err", err)
		return
	}
	if len(payments) == 0 {
		return
	}

	p.logger.Info("poller: checking stale pending payments", "count", len(payments))

	for i := range payments {
		pm := payments[i]
		if pm.TBankPaymentID == "" {
			// Нет PaymentId — нечего опрашивать (Init не завершился нормально).
			continue
		}
		p.checkOne(ctx, pm)
	}
}

func (p *PendingPaymentPoller) checkOne(ctx context.Context, pm domain.Payment) {
	status, err := p.provider.GetState(ctx, pm.TBankPaymentID)
	if err != nil {
		p.logger.Error("poller: getstate",
			"payment_id", pm.ID,
			"tbank_payment_id", pm.TBankPaymentID,
			"err", err,
		)
		return
	}

	mapped := payment.MapStatus(status)
	p.logger.Info("poller: getstate result",
		"payment_id", pm.ID,
		"tbank_status", status,
		"mapped", mapped,
	)

	switch mapped {
	case "confirmed":
		if err := p.store.ActivateSubscriptionTx(ctx, pm.TBankOrderID, pm.TBankPaymentID, status); err != nil {
			p.logger.Error("poller: activate subscription", "payment_id", pm.ID, "err", err)
		}
	case "rejected":
		if err := p.store.RejectPaymentTx(ctx, pm.TBankOrderID, pm.TBankPaymentID, status, ""); err != nil {
			p.logger.Error("poller: reject payment", "payment_id", pm.ID, "err", err)
		}
	}
	// pending/новые статусы — игнорируем, проверим на следующем тике.
}
