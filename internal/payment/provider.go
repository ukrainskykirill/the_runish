package payment

import "context"

type Receipt struct {
	Email    string
	Phone    string
	Items    []ReceiptItem
	Taxation string
}

type ReceiptItem struct {
	Name          string
	Price         int64
	Quantity      float64
	Amount        int64
	Tax           string
	PaymentMethod string
	PaymentObject string
}

type InitParams struct {
	OrderID         string
	AmountKop       int64
	Description     string
	NotificationURL string
	SuccessURL      string
	FailURL         string
	Receipt         *Receipt
}

type InitResult struct {
	PaymentID  string
	PaymentURL string
	Status     string
}

type PaymentProvider interface {
	Init(ctx context.Context, p InitParams) (InitResult, error)
	GetState(ctx context.Context, paymentID string) (string, error)
	Cancel(ctx context.Context, paymentID string) error
	Refund(ctx context.Context, paymentID string, amountKop int64, receipt *Receipt) error
}
