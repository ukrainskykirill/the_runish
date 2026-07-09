package storage

import "time"

type PaymentWithDetails struct {
	ID           int64
	OrderID      int64
	AmountKop    int64
	Status       string
	Provider     string
	TBankOrderID string
	PaymentURL   string
	ErrorCode    string
	CreatedAt    time.Time
	PaidAt       *time.Time
	UserName     string
	UserTg       string
	ContactName  string
	ContactPhone string
}

type DailyPaymentStat struct {
	Date      time.Time
	UserCount int
	TotalKop  int64
	PayCount  int
	UserNames []string
}

type PaymentPeriodStats struct {
	TotalKop    int64
	PayCount    int
	UniqueUser  int
	MaxDailyKop int64
	Daily       []DailyPaymentStat
}

type PaymentDashboard struct {
	Month PaymentPeriodStats
	Week  PaymentPeriodStats
}
