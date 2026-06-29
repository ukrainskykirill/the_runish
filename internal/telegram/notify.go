package telegram

import "fmt"

// ReminderText формирует текст напоминания о продлении подписки.
func ReminderText(serviceTitle string, daysLeft int, runnersURL string) string {
	// Склонение слова "день".
	var dayWord string
	switch daysLeft {
	case 1:
		dayWord = "день"
	case 2, 3, 4:
		dayWord = "дня"
	default:
		dayWord = "дней"
	}

	return fmt.Sprintf(
		"⏳ Ваша подписка «%s» истекает через %d %s.\n"+
			"Чтобы продлить — перейдите по ссылке: %s",
		serviceTitle, daysLeft, dayWord, runnersURL,
	)
}
