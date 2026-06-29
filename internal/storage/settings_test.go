package storage

import "testing"

func TestParseReminderDays(t *testing.T) {
	days, err := ParseReminderDays("14, 7,3, 1,7")
	if err != nil {
		t.Fatalf("ParseReminderDays returned error: %v", err)
	}
	want := []int{14, 7, 3, 1}
	if len(days) != len(want) {
		t.Fatalf("len(days) = %d, want %d", len(days), len(want))
	}
	for i := range want {
		if days[i] != want[i] {
			t.Fatalf("days[%d] = %d, want %d", i, days[i], want[i])
		}
	}
}

func TestParseReminderDaysRejectsInvalidInput(t *testing.T) {
	cases := []string{"", "7,-1", "abc", "366"}
	for _, tc := range cases {
		if _, err := ParseReminderDays(tc); err == nil {
			t.Fatalf("ParseReminderDays(%q) returned nil error", tc)
		}
	}
}
