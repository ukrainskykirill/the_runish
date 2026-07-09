package domain

import "time"

type ServiceKind string

const (
	KindSubscription ServiceKind = "subscription"
	KindTraining     ServiceKind = "training"
	KindFree         ServiceKind = "free"
	KindEntry        ServiceKind = "entry"
	KindBundle       ServiceKind = "bundle"
)

func (k ServiceKind) IsValid() bool {
	switch k {
	case KindSubscription, KindTraining, KindFree, KindEntry, KindBundle:
		return true
	}
	return false
}

func (k ServiceKind) GrantsEntry() bool { return k == KindEntry || k == KindBundle }

func (k ServiceKind) GrantsSubscription() bool { return k == KindSubscription || k == KindBundle }

type Service struct {
	ID              int64       `json:"id"`
	Kind            ServiceKind `json:"kind"`
	Title           string      `json:"title"`
	Description     string      `json:"description"`
	PriceKop        int64       `json:"price_kop"`
	DurationDays    *int        `json:"duration_days,omitempty"`
	SortOrder       int         `json:"sort_order"`
	IsActive        bool        `json:"is_active"`
	PriceWithSubKop *int64      `json:"price_with_sub_kop,omitempty"`
	PromoPriceKop   *int64      `json:"promo_price_kop,omitempty"`
	TrainingsQuota  *int        `json:"trainings_quota,omitempty"`

	EffectivePriceKop int64     `json:"effective_price_kop"`
	Locked            bool      `json:"locked"`
	Owned             bool      `json:"owned"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
