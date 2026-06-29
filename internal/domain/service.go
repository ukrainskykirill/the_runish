package domain

import "time"

// ServiceKind — тип услуги.
type ServiceKind string

const (
	KindSubscription ServiceKind = "subscription" // подписка (требует оплаченного взноса)
	KindTraining     ServiceKind = "training"     // разовая/индивидуальная услуга
	KindFree         ServiceKind = "free"         // бесплатная (пробная)
	KindEntry        ServiceKind = "entry"        // вступительный взнос
	KindBundle       ServiceKind = "bundle"       // взнос + первый месяц подписки
)

// IsValid проверяет, что kind из допустимого множества.
func (k ServiceKind) IsValid() bool {
	switch k {
	case KindSubscription, KindTraining, KindFree, KindEntry, KindBundle:
		return true
	}
	return false
}

// GrantsEntry — продукт даёт вступительный взнос (взнос или бандл).
func (k ServiceKind) GrantsEntry() bool { return k == KindEntry || k == KindBundle }

// GrantsSubscription — продукт активирует/продлевает подписку (подписка или бандл).
func (k ServiceKind) GrantsSubscription() bool { return k == KindSubscription || k == KindBundle }

// Service — позиция каталога.
type Service struct {
	ID           int64       `json:"id"`
	Kind         ServiceKind `json:"kind"`
	Title        string      `json:"title"`
	Description  string      `json:"description"`
	PriceKop     int64       `json:"price_kop"`
	DurationDays *int        `json:"duration_days,omitempty"`
	SortOrder    int         `json:"sort_order"`
	IsActive     bool        `json:"is_active"`
	// Скидки: цена «при подписке» и льготная цена «первых 30». nil = не применяется.
	PriceWithSubKop *int64 `json:"price_with_sub_kop,omitempty"`
	PromoPriceKop   *int64 `json:"promo_price_kop,omitempty"`
	// Вычисляемые поля для API (зависят от текущего юзера и акции). Не из БД.
	EffectivePriceKop int64 `json:"effective_price_kop"`
	Locked            bool  `json:"locked"` // подписка недоступна: не оплачен взнос
	Owned             bool  `json:"owned"`  // взнос/бандл уже оплачен — повторно нельзя
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
