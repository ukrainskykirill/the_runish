package worker

import (
	"context"
	"log/slog"
	"time"

	"therunish/internal/domain"
	"therunish/internal/observability"
	"therunish/internal/payment"
	"therunish/internal/storage"
)

type PendingPaymentPoller struct {
	store    *storage.Store
	provider payment.PaymentProvider
	logger   *slog.Logger

	interval   time.Duration
	staleAfter time.Duration
}

func NewPendingPaymentPoller(store *storage.Store, provider payment.PaymentProvider, logger *slog.Logger) *PendingPaymentPoller {
	return &PendingPaymentPoller{
		store:      store,
		provider:   provider,
		logger:     logger,
		interval:   2 * time.Minute,
		staleAfter: 5 * time.Minute,
	}
}

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
			continue
		}
		p.checkOne(ctx, pm)
	}
}

func (p *PendingPaymentPoller) checkOne(ctx context.Context, pm domain.Payment) {
	status, err := p.provider.GetState(ctx, pm.TBankPaymentID)
	if err != nil {
		observability.Alert(ctx, p.logger, "Оплата: не удалось проверить статус платежа (GetState)", err,
			"payment_id", pm.ID, "tbank_payment_id", pm.TBankPaymentID)
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
			observability.Alert(ctx, p.logger, "Оплата: не удалось активировать подписку (поллер)", err, "payment_id", pm.ID)
		}
	case "rejected":
		if err := p.store.RejectPaymentTx(ctx, pm.TBankOrderID, pm.TBankPaymentID, status, ""); err != nil {
			observability.Alert(ctx, p.logger, "Оплата: не удалось обработать отклонённый платёж (поллер)", err, "payment_id", pm.ID)
		}
	}
}
