package usecase

import (
	"context"
	"errors"

	"therunish/internal/domain"
	"therunish/internal/storage"
)

var (
	ErrNoSession                 = errors.New("no session")
	ErrInvalidService            = errors.New("invalid service")
	ErrServiceInactive           = errors.New("service inactive")
	ErrSubscriptionAlreadyInCart = errors.New("subscription already in cart")
	ErrCartEmpty                 = errors.New("cart empty")
	ErrCartInvalid               = errors.New("cart invalid")
	ErrMultipleSubscriptions     = errors.New("multiple subscriptions")
	ErrEntryFeeRequired          = errors.New("entry fee required")
	ErrAlreadyOwned              = errors.New("already owned")
	ErrPhoneRequired             = errors.New("phone required")
	ErrPaymentInitFailed         = errors.New("payment init failed")
)

type CartLine struct {
	ServiceID int64  `json:"service_id"`
	Title     string `json:"title"`
	PriceKop  int64  `json:"price_kop"`
	Qty       int    `json:"qty"`
	Subtotal  int64  `json:"subtotal"`
}

type CartResponse struct {
	Lines []CartLine `json:"lines"`
	Total int64      `json:"total"`
	Count int        `json:"count"`
}

type Cart struct {
	store   *storage.Store
	pricing *Pricing
}

func NewCart(store *storage.Store, pricing *Pricing) *Cart {
	return &Cart{store: store, pricing: pricing}
}

func (c *Cart) View(ctx context.Context, user *domain.User) (CartResponse, error) {
	resp := CartResponse{Lines: []CartLine{}}
	if user == nil {
		return resp, nil
	}

	cart, err := c.store.GetCart(ctx, user.ID)
	if err != nil {
		return resp, err
	}
	if len(cart) == 0 {
		return resp, nil
	}

	services, err := c.servicesByCart(ctx, cart)
	if err != nil {
		return resp, err
	}
	pc := c.pricing.Context(ctx, user)

	for _, it := range cart {
		svc, ok := services[it.ServiceID]
		if !ok {
			continue
		}
		pc.ApplyService(&svc)
		subtotal := svc.EffectivePriceKop * int64(it.Qty)
		resp.Lines = append(resp.Lines, CartLine{
			ServiceID: svc.ID,
			Title:     svc.Title,
			PriceKop:  svc.EffectivePriceKop,
			Qty:       it.Qty,
			Subtotal:  subtotal,
		})
		resp.Total += subtotal
		resp.Count += it.Qty
	}

	return resp, nil
}

func (c *Cart) Add(ctx context.Context, serviceID int64, user *domain.User) (CartResponse, error) {
	if user == nil {
		return CartResponse{}, ErrNoSession
	}

	newSvc, err := c.store.GetService(ctx, serviceID)
	if errors.Is(err, storage.ErrNotFound) {
		return CartResponse{}, ErrInvalidService
	}
	if err != nil {
		return CartResponse{}, err
	}
	if !newSvc.IsActive {
		return CartResponse{}, ErrServiceInactive
	}

	if newSvc.Kind.GrantsSubscription() {
		cart, err := c.store.GetCart(ctx, user.ID)
		if err != nil {
			return CartResponse{}, err
		}
		for _, it := range cart {
			if it.ServiceID == newSvc.ID {
				continue
			}
			existing, err := c.store.GetService(ctx, it.ServiceID)
			if err != nil {
				continue
			}
			if existing.Kind.GrantsSubscription() {
				return CartResponse{}, ErrSubscriptionAlreadyInCart
			}
		}
	}

	if err := c.store.AddToCart(ctx, user.ID, serviceID); err != nil {
		return CartResponse{}, err
	}
	return c.View(ctx, user)
}

func (c *Cart) Remove(ctx context.Context, serviceID int64, user *domain.User) (CartResponse, error) {
	if user != nil {
		if err := c.store.RemoveFromCart(ctx, user.ID, serviceID); err != nil {
			return CartResponse{}, err
		}
	}
	return c.View(ctx, user)
}

func (c *Cart) checkoutItems(ctx context.Context, user *domain.User) ([]domain.OrderItem, int64, error) {
	cart, _ := c.store.GetCart(ctx, user.ID)
	if len(cart) == 0 {
		return nil, 0, ErrCartEmpty
	}

	services, err := c.servicesByCart(ctx, cart)
	if err != nil {
		return nil, 0, err
	}

	pc := c.pricing.Context(ctx, user)
	var total int64
	var subCount int
	items := make([]domain.OrderItem, 0, len(cart))

	for _, it := range cart {
		svc, ok := services[it.ServiceID]
		if !ok {
			continue
		}

		qty := it.Qty
		if qty > 1 {
			qty = 1
		}
		if svc.Kind.GrantsSubscription() {
			subCount += qty
			if subCount > 1 {
				return nil, 0, ErrMultipleSubscriptions
			}
		}

		pc.ApplyService(&svc)
		if svc.Locked {
			return nil, 0, ErrEntryFeeRequired
		}
		if svc.Owned {
			return nil, 0, ErrAlreadyOwned
		}

		items = append(items, domain.OrderItem{
			ServiceID: svc.ID,
			Title:     svc.Title,
			PriceKop:  svc.EffectivePriceKop,
			Qty:       qty,
		})
		total += svc.EffectivePriceKop * int64(qty)
	}

	if len(items) == 0 {
		return nil, 0, ErrCartInvalid
	}
	return items, total, nil
}

func (c *Cart) servicesByCart(ctx context.Context, cart []storage.CartItem) (map[int64]domain.Service, error) {
	ids := make([]int64, len(cart))
	for i, it := range cart {
		ids[i] = it.ServiceID
	}
	return c.store.ListServicesByIDs(ctx, ids)
}
