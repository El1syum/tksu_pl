package models

import "testing"

func TestRecurringCalendar(t *testing.T) {
	for _, tc := range []struct{ date, anchor, frequency, want string }{
		{"2026-01-31", "2026-01-31", "monthly", "2026-02-28"},
		{"2026-02-28", "2026-01-31", "monthly", "2026-03-31"},
		{"2024-01-31", "2024-01-31", "monthly", "2024-02-29"},
		{"2024-02-29", "2024-02-29", "yearly", "2025-02-28"},
		{"2027-02-28", "2024-02-29", "yearly", "2028-02-29"},
		{"2026-12-29", "2026-12-29", "weekly", "2027-01-05"},
		{"2026-12-31", "2026-12-31", "daily", "2027-01-01"},
		{"9999-12-31", "9999-12-31", "daily", ""},
		{"2026-01-01", "bad", "monthly", ""}, {"2026-01-01", "2026-01-01", "invalid", ""},
	} {
		if got := NextRecurringDate(tc.date, tc.anchor, tc.frequency); got != tc.want {
			t.Errorf("%+v got %s", tc, got)
		}
	}
	if !ValidMonth("2026-09") || ValidMonth("2026-13") || ValidMonth("2026-9") || ValidMonth("0000-01") {
		t.Fatal("invalid month validation")
	}
	from, to := MonthBounds("2024-02")
	if from != "2024-02-01" || to != "2024-02-29" {
		t.Fatal(from, to)
	}
}

func TestResumeDateRespectsEndAndAnchor(t *testing.T) {
	r := Recurring{NextDate: "2026-01-31", AnchorDate: "2026-01-31", Frequency: "monthly"}
	if got := r.ResumeDate("2026-03-01"); got != "2026-03-31" {
		t.Fatal(got)
	}
	r.EndDate = "2026-03-30"
	if got := r.ResumeDate("2026-03-01"); got != "" {
		t.Fatal("resumed beyond end date", got)
	}
}
