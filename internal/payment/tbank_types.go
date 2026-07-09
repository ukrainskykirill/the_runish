package payment

import "strings"

type tbankInitResponse struct {
	Success    bool   `json:"Success"`
	ErrorCode  string `json:"ErrorCode"`
	Message    string `json:"Message"`
	PaymentID  string `json:"PaymentId"`
	PaymentURL string `json:"PaymentURL"`
	Status     string `json:"Status"`
	OrderID    string `json:"OrderId"`
}

type tbankStateResponse struct {
	Success   bool   `json:"Success"`
	ErrorCode string `json:"ErrorCode"`
	Message   string `json:"Message"`
	PaymentID string `json:"PaymentId"`
	Status    string `json:"Status"`
	OrderID   string `json:"OrderId"`
}

type Notification struct {
	TerminalKey string `json:"TerminalKey"`
	OrderID     string `json:"OrderId"`
	Success     bool   `json:"Success"`
	Status      string `json:"Status"`
	PaymentID   flexID `json:"PaymentId"`
	ErrorCode   string `json:"ErrorCode"`
	Amount      int64  `json:"Amount"`
}

type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	*f = flexID(strings.Trim(string(b), `"`))
	return nil
}

func (f flexID) String() string { return string(f) }
