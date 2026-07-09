package domain

import "errors"

var (
	ErrTrainingNoEntryFee        = errors.New("entry fee not paid")
	ErrTrainingNoAccess          = errors.New("no active access for this training")
	ErrTrainingSessionFull       = errors.New("session is full")
	ErrTrainingAlreadyRegistered = errors.New("already registered")
	ErrTrainingSessionCancelled  = errors.New("session cancelled")
	ErrTrainingPastSession       = errors.New("session already started")
)
