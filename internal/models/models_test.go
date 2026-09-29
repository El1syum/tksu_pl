package models

import "testing"

func TestParseAmount(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int64
		ok    bool
	}{{"0,01", 1, true}, {" 125.5 ", 12550, true}, {"999999999.99", 99999999999, true}, {"001.20", 120, true}, {"0", 0, false}, {"-1", 0, false}, {"NaN", 0, false}, {"1e3", 0, false}, {"2.123", 0, false}, {"1.", 0, false}, {".1", 0, false}, {"1000000000", 0, false}, {"1;DROP TABLE users", 0, false}} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseAmount(tc.input)
			if (err == nil) != tc.ok || got != tc.want {
				t.Fatalf("got %d %v; want %d valid=%v", got, err, tc.want, tc.ok)
			}
		})
	}
}
func TestDates(t *testing.T) {
	for _, date := range []string{"2024-02-29", "2026-09-30", "1900-01-01", "9999-12-31"} {
		if !ValidDate(date) {
			t.Errorf("valid date rejected: %s", date)
		}
	}
	for _, date := range []string{"2025-02-29", "2026-9-30", "0000-01-01", "2026-04-31", ""} {
		if ValidDate(date) {
			t.Errorf("invalid date accepted: %s", date)
		}
	}
}
