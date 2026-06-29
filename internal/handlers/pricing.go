package handlers

import (
	"context"

	"therunish/internal/domain"
)

// pricingCtx — контекст расчёта эффективной цены и доступности услуг.
// Единый источник правды: используется и каталогом, и checkout.
type pricingCtx struct {
	promoActive  bool // акция «первых 30» активна (переключатель + счётчик < 30)
	hasActiveSub bool // у текущего юзера есть активная подписка
	entryPaid    bool // текущий юзер оплатил вступительный взнос
}

// pricingContext собирает контекст для расчёта (юзер может быть nil — гость).
func (a *App) pricingContext(ctx context.Context, user *domain.User) pricingCtx {
	var pc pricingCtx

	promo, err := a.store.First30PromoEnabled(ctx)
	if err != nil {
		a.logger.Error("promo enabled", "err", err)
	}
	if promo {
		n, err := a.store.CountEntryFeePaid(ctx)
		if err != nil {
			a.logger.Error("count entry fee paid", "err", err)
		} else if n < 30 {
			pc.promoActive = true
		}
	}

	if user != nil {
		pc.entryPaid = user.EntryFeePaid
		subs, err := a.store.ListActiveSubsByUser(ctx, user.ID)
		if err != nil {
			a.logger.Error("list active subs", "err", err)
		}
		pc.hasActiveSub = len(subs) > 0
	}
	return pc
}

// apply проставляет услуге EffectivePriceKop, Locked, Owned.
func (pc pricingCtx) apply(svc *domain.Service) {
	price := svc.PriceKop
	if pc.promoActive && svc.PromoPriceKop != nil {
		price = *svc.PromoPriceKop
	}
	if pc.hasActiveSub && svc.PriceWithSubKop != nil {
		price = *svc.PriceWithSubKop
	}
	svc.EffectivePriceKop = price

	// Подписку нельзя купить без оплаченного взноса.
	svc.Locked = svc.Kind == domain.KindSubscription && !pc.entryPaid
	// «Уже есть»: взнос/бандл — после оплаты взноса; подписка — пока активна
	// (в т.ч. полученная из бандла), повторно покупать не нужно — продление автоматическое.
	svc.Owned = (svc.Kind.GrantsEntry() && pc.entryPaid) ||
		(svc.Kind == domain.KindSubscription && pc.hasActiveSub)
}
