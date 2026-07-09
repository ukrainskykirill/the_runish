package usecase

import (
	"context"
	"time"

	"therunish/internal/domain"
	"therunish/internal/storage"
)

type Training struct {
	store *storage.Store
}

func NewTraining(store *storage.Store) *Training {
	return &Training{store: store}
}

func (t *Training) Upcoming(ctx context.Context, userID int64) ([]domain.TrainingOccurrence, error) {
	return t.store.ListUpcomingSessions(ctx, userID)
}

func (t *Training) Register(ctx context.Context, userID, trainingID int64, date time.Time) (domain.TrainingRegistration, error) {
	return t.store.RegisterForTrainingTx(ctx, userID, trainingID, date)
}

func (t *Training) Cancel(ctx context.Context, userID, registrationID int64) error {
	return t.store.CancelRegistrationTx(ctx, userID, registrationID)
}

func (t *Training) MyRegistrations(ctx context.Context, userID int64) ([]domain.TrainingRegistration, error) {
	return t.store.ListMyTrainingRegistrations(ctx, userID)
}
