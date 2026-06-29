package domain

import "time"

type PaymentStatus string

const (
	PaymentStatusNew       PaymentStatus = "new"
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusConfirmed PaymentStatus = "confirmed"
	PaymentStatusRejected  PaymentStatus = "rejected"
	PaymentStatusRefunded  PaymentStatus = "refunded"
)

type Payment struct {
	ID             int64
	UserID         int64
	OrderID        int64
	AmountKop      int64
	Status         PaymentStatus
	Provider       string
	TBankPaymentID string
	TBankOrderID   string
	TBankStatus    string
	PaymentURL     string
	ErrorCode      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	PaidAt         *time.Time
}

// IsFinal возвращает true, если платёж в терминальном статусе (не pending/new).
func (p Payment) IsFinal() bool {
	switch p.Status {
	case PaymentStatusConfirmed, PaymentStatusRejected, PaymentStatusRefunded:
		return true
	}
	return false
}
