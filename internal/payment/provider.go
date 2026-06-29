package payment

import "context"

// Receipt — фискальный чек 54-ФЗ для T-Bank (поле Receipt в /Init).
// Должен быть указан Email ИЛИ Phone. Если оба пустые — Receipt не отправляется.
type Receipt struct {
	Email    string        // email покупателя (можно оба)
	Phone    string        // телефон в формате +7XXXXXXXXXX (можно оба)
	Items    []ReceiptItem // позиции чека
	Taxation string        // система налогообложения (usn_income, osn, …)
}

// ReceiptItem — одна позиция чека.
type ReceiptItem struct {
	Name          string  // наименование услуги/товара (≤128 символов)
	Price         int64   // цена за единицу, копейки
	Quantity      float64 // количество (1.0 для услуги)
	Amount        int64   // итоговая сумма позиции = Price × Quantity, копейки
	Tax           string  // ставка НДС: none, vat0, vat10, vat20, vat110, vat118
	PaymentMethod string  // full_payment | full_prepayment | prepayment | partial_prepayment
	PaymentObject string  // service | commodity | ...
}

type InitParams struct {
	OrderID         string // наш tbank_order_id (уникальный)
	AmountKop       int64
	Description     string
	NotificationURL string
	SuccessURL      string
	FailURL         string
	Receipt         *Receipt // необязательно: чек для 54-ФЗ (нужен Email или Phone)
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
	// Refund инициирует возврат платежа (полный или частичный).
	// amountKop должен быть <= сумме исходного платежа. receipt — чек возврата
	// (54-ФЗ); nil, если фискализация не нужна.
	// Возврат асинхронен: фактическое завершение приходит вебхуком REFUNDED/PARTIAL_REFUNDED.
	Refund(ctx context.Context, paymentID string, amountKop int64, receipt *Receipt) error
}
