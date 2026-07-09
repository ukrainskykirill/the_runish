package usecase

import (
	"context"
	"errors"
	"time"

	"therunish/internal/domain"
	"therunish/internal/storage"
	"therunish/internal/survey"
)

type AdminUsers struct {
	store *storage.Store
}

type AdminUserDetail struct {
	User           domain.User
	Subscriptions  []domain.Subscription
	Services       []domain.Service
	Payments       []storage.PaymentWithDetails
	SurveyStatus   string
	SurveyQA       []survey.QA
	ActiveSubUntil *time.Time
}

type AdminSubscriptionForm struct {
	User     domain.User
	Sub      domain.Subscription
	Services []domain.Service
}

func NewAdminUsers(store *storage.Store) *AdminUsers {
	return &AdminUsers{store: store}
}

func (u *AdminUsers) List(ctx context.Context, search string) ([]storage.UserListItem, error) {
	return u.store.ListUsers(ctx, search)
}

func (u *AdminUsers) Detail(ctx context.Context, userID int64) (AdminUserDetail, error) {
	user, err := u.store.GetUserByID(ctx, userID)
	if err != nil {
		return AdminUserDetail{}, err
	}
	subs, err := u.store.ListSubsByUser(ctx, userID)
	if err != nil {
		return AdminUserDetail{}, err
	}
	services, err := u.store.ListServices(ctx, true)
	if err != nil {
		return AdminUserDetail{}, err
	}
	payments, err := u.store.ListPaymentsByUser(ctx, userID, 20)
	if err != nil {
		return AdminUserDetail{}, err
	}

	var surveyStatus string
	var surveyQA []survey.QA
	if sv, err := u.store.GetSurvey(ctx, userID); err == nil {
		surveyStatus = surveyStatusLabel(sv.Status)
		if sv.Status == domain.SurveyCompleted || sv.Status == domain.SurveyInProgress {
			questions, err := u.store.ListSurveyQuestions(ctx, false)
			if err != nil {
				return AdminUserDetail{}, err
			}
			surveyQA = survey.Build(questions).RenderAnswers(sv.Branch, sv.Answers)
		}
	} else if !errors.Is(err, storage.ErrNotFound) {
		return AdminUserDetail{}, err
	}

	return AdminUserDetail{
		User:           user,
		Subscriptions:  subs,
		Services:       services,
		Payments:       payments,
		SurveyStatus:   surveyStatus,
		SurveyQA:       surveyQA,
		ActiveSubUntil: activeSubUntil(subs),
	}, nil
}

func (u *AdminUsers) SubscriptionForm(ctx context.Context, userID, subID int64) (AdminSubscriptionForm, error) {
	sub, err := u.store.GetSubscriptionByID(ctx, subID)
	if err != nil {
		return AdminSubscriptionForm{}, err
	}
	services, err := u.store.ListServices(ctx, true)
	if err != nil {
		return AdminSubscriptionForm{}, err
	}
	user, err := u.store.GetUserByID(ctx, userID)
	if err != nil {
		return AdminSubscriptionForm{}, err
	}
	return AdminSubscriptionForm{
		User:     user,
		Sub:      sub,
		Services: services,
	}, nil
}

func surveyStatusLabel(s domain.SurveyStatus) string {
	switch s {
	case domain.SurveyPending:
		return "не начата"
	case domain.SurveyInProgress:
		return "в процессе"
	case domain.SurveyCompleted:
		return "завершена"
	default:
		return string(s)
	}
}

func activeSubUntil(subs []domain.Subscription) *time.Time {
	var result *time.Time
	for i := range subs {
		if subs[i].Status == domain.SubStatusActive {
			if result == nil || subs[i].ExpiresAt.After(*result) {
				t := subs[i].ExpiresAt
				result = &t
			}
		}
	}
	return result
}
