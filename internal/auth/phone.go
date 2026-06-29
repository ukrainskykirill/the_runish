package auth

import "strings"

// NormalizeRuPhone приводит российский номер к формату +7XXXXXXXXXX.
// Принимает варианты с 8/7/+7, пробелами, скобками, дефисами. Возвращает ok=false,
// если после очистки получилось не 11 цифр или это не российский номер.
func NormalizeRuPhone(raw string) (string, bool) {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()

	// Ведущая 8 → 7 (8 (999)… → 7999…).
	if len(d) == 11 && d[0] == '8' {
		d = "7" + d[1:]
	}
	if len(d) != 11 || d[0] != '7' {
		return "", false
	}
	return "+" + d, true
}
