package payment

import (
	"context"
	"fmt"
)

// MockProvider — заглушка для локальной разработки и тестов.
// Init сразу возвращает фейковый URL на внутреннюю страницу-эмулятор.
type MockProvider struct {
	BaseURL string // например http://localhost:8080/payment/mock
}

func NewMock(baseURL string) *MockProvider {
	return &MockProvider{BaseURL: baseURL}
}

func (m *MockProvider) Init(_ context.Context, p InitParams) (InitResult, error) {
	return InitResult{
		PaymentID:  "mock-" + p.OrderID,
		PaymentURL: fmt.Sprintf("%s/%s", m.BaseURL, p.OrderID),
		Status:     "NEW",
	}, nil
}

func (m *MockProvider) GetState(_ context.Context, _ string) (string, error) {
	return "CONFIRMED", nil
}

func (m *MockProvider) Cancel(_ context.Context, _ string) error {
	return nil
}

func (m *MockProvider) Refund(_ context.Context, _ string, _ int64, _ *Receipt) error {
	return nil
}
