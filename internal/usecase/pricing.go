package usecase

import (
	"context"
	"log/slog"

	"therunish/internal/domain"
	"therunish/internal/storage"
)

type Pricing struct {
	store  *storage.Store
	logger *slog.Logger
}

const First30PromoLimit = 30

type PricingContext struct {
	PromoActive  bool
	hasActiveSub bool
	entryPaid    bool
}

func NewPricing(store *storage.Store, logger *slog.Logger) *Pricing {
	return &Pricing{store: store, logger: logger}
}

func (p *Pricing) Context(ctx context.Context, user *domain.User) PricingContext {
	var pc PricingContext

	promo, err := p.store.First30PromoEnabled(ctx)
	if err != nil {
		p.logger.Error("promo enabled", "err", err)
	}
	if promo {
		n, err := p.store.CountEntryFeeReserved(ctx)
		if err != nil {
			p.logger.Error("count entry fee reserved", "err", err)
		} else if n < First30PromoLimit {
			pc.PromoActive = true
		}
	}

	if user != nil {
		pc.entryPaid = user.EntryFeePaid
		subs, err := p.store.ListActiveSubsByUser(ctx, user.ID)
		if err != nil {
			p.logger.Error("list active subs", "err", err)
		}
		pc.hasActiveSub = len(subs) > 0
	}

	return pc
}

func (pc PricingContext) ApplyService(svc *domain.Service) {
	price := svc.PriceKop
	if pc.PromoActive && svc.PromoPriceKop != nil {
		price = *svc.PromoPriceKop
	}
	if pc.hasActiveSub && svc.PriceWithSubKop != nil {
		price = *svc.PriceWithSubKop
	}

	svc.EffectivePriceKop = price
	svc.Locked = svc.Kind == domain.KindSubscription && !pc.entryPaid
	svc.Owned = (svc.Kind.GrantsEntry() && pc.entryPaid) ||
		(svc.Kind == domain.KindSubscription && pc.hasActiveSub)
}
