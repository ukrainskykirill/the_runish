package usecase

import (
	"context"
	"errors"
	"time"

	"therunish/internal/domain"
	"therunish/internal/storage"
)

var (
	ErrInvalidSubscriptionStatus = errors.New("invalid subscription status")
	ErrInvalidExtensionDays      = errors.New("invalid extension days")
)

type AdminMembership struct {
	store *storage.Store
}

func NewAdminMembership(store *storage.Store) *AdminMembership {
	return &AdminMembership{store: store}
}

func (m *AdminMembership) SetEntryFee(ctx context.Context, userID int64, paid bool) error {
	return m.store.SetEntryFeePaid(ctx, userID, paid)
}

func (m *AdminMembership) AddSubscription(ctx context.Context, userID, serviceID int64, expiresAt time.Time) error {
	return m.store.CreateSubscriptionAdmin(ctx, userID, serviceID, expiresAt)
}

func (m *AdminMembership) UpdateSubscription(ctx context.Context, subID, serviceID int64, status domain.SubscriptionStatus, expiresAt time.Time) error {
	if !validSubscriptionStatus(status) {
		return ErrInvalidSubscriptionStatus
	}
	return m.store.UpdateSubscription(ctx, subID, serviceID, status, expiresAt)
}

func (m *AdminMembership) ExtendSubscription(ctx context.Context, subID int64, days int) error {
	if !validExtensionDays(days) {
		return ErrInvalidExtensionDays
	}
	return m.store.ExtendSubscription(ctx, subID, days)
}

func (m *AdminMembership) DeleteSubscription(ctx context.Context, subID int64) error {
	return m.store.DeleteSubscriptionWithEntryFeeRevoke(ctx, subID)
}

func validSubscriptionStatus(status domain.SubscriptionStatus) bool {
	switch status {
	case domain.SubStatusActive, domain.SubStatusExpired, domain.SubStatusCancelled:
		return true
	default:
		return false
	}
}

func validExtensionDays(days int) bool {
	switch days {
	case 1, 7, 14, 30, 90, 180, 365:
		return true
	default:
		return false
	}
}
