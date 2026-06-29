package domain

import "time"

type User struct {
	ID            int64     `json:"id"`
	TelegramID    int64     `json:"telegram_id"`
	Username      string    `json:"username"`
	FullName      string    `json:"full_name"`
	Phone         string    `json:"phone"`
	BotDialogOpen bool      `json:"bot_dialog_open"`
	IsAdmin       bool      `json:"is_admin"`
	EntryFeePaid  bool      `json:"entry_fee_paid"` // вступительный взнос оплачен (навсегда)
	CreatedAt     time.Time `json:"created_at"`
}
