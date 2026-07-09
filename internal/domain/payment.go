package domain

import "time"

type PaymentStatus string

const (
	PaymentStatusNew       PaymentStatus = "new"
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusConfirmed PaymentStatus = "confirmed"
	PaymentStatusRejected  PaymentStatus = "rejected"
	PaymentStatusRefunded  PaymentStatus = "refunded"
	PaymentStatusCancelled PaymentStatus = "cancelled"
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
	RefundedKop    int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	PaidAt         *time.Time
}

func (p Payment) IsFinal() bool {
	switch p.Status {
	case PaymentStatusConfirmed, PaymentStatusRejected, PaymentStatusRefunded:
		return true
	}
	return false
}
