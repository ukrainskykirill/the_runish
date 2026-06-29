package auth

import "testing"

func TestNormalizeRuPhone(t *testing.T) {
	ok := map[string]string{
		"89991234567":       "+79991234567",
		"79991234567":       "+79991234567",
		"+79991234567":      "+79991234567",
		"+7 (999) 123-45-67": "+79991234567",
		"8 999 123 45 67":   "+79991234567",
	}
	for in, want := range ok {
		got, valid := NormalizeRuPhone(in)
		if !valid || got != want {
			t.Errorf("NormalizeRuPhone(%q) = %q,%v; want %q,true", in, got, valid, want)
		}
	}

	bad := []string{"", "12345", "9991234567", "+1 202 555 0100", "phone", "799912345678"}
	for _, in := range bad {
		if got, valid := NormalizeRuPhone(in); valid {
			t.Errorf("NormalizeRuPhone(%q) = %q,true; want invalid", in, got)
		}
	}
}
