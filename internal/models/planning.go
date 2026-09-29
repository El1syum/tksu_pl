package models

import "time"

type Budget struct {
	CategoryID, Amount, Spent int64
	Category, Month           string
}

func (b Budget) Over() bool       { return b.Spent > b.Amount }
func (b Budget) Reached() bool    { return b.Spent >= b.Amount }
func (b Budget) Remaining() int64 { return max(0, b.Amount-b.Spent) }
func (b Budget) OverBy() int64    { return max(0, b.Spent-b.Amount) }
func (b Budget) Progress() int64  { return min(b.Spent, b.Amount) }

type Recurring struct {
	ID, UserID, CategoryID, Amount                                  int64
	Description, Category, Frequency, AnchorDate, NextDate, EndDate string
	Enabled                                                         bool
}

// ResumeDate skips paused dates while preserving the original calendar anchor.
// An empty result means that the schedule has no remaining occurrence.
func (r Recurring) ResumeDate(today string) string {
	date := r.NextDate
	for date != "" && date < today {
		date = NextRecurringDate(date, r.AnchorDate, r.Frequency)
	}
	if r.EndDate != "" && date > r.EndDate {
		return ""
	}
	return date
}

func (r Recurring) Finished() bool {
	return !r.Enabled && r.ResumeDate(time.Now().Format("2006-01-02")) == ""
}

func FrequencyLabel(f string) string {
	return map[string]string{"daily": "Каждый день", "weekly": "Каждую неделю", "monthly": "Каждый месяц", "yearly": "Каждый год"}[f]
}
func ValidMonth(s string) bool { return len(s) == 7 && ValidDate(s+"-01") }
func MonthBounds(s string) (string, string) {
	t, _ := time.Parse("2006-01", s)
	return s + "-01", time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

// Month-end/leap-day schedules preserve the original day after shorter months.
func NextRecurringDate(current, anchor, frequency string) string {
	c, err := time.Parse("2006-01-02", current)
	if err != nil {
		return ""
	}
	a, err := time.Parse("2006-01-02", anchor)
	if err != nil {
		return ""
	}
	var next time.Time
	switch frequency {
	case "daily":
		next = c.AddDate(0, 0, 1)
	case "weekly":
		next = c.AddDate(0, 0, 7)
	case "monthly":
		first := time.Date(c.Year(), c.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		last := first.AddDate(0, 1, -1).Day()
		next = time.Date(first.Year(), first.Month(), min(a.Day(), last), 0, 0, 0, 0, time.UTC)
	case "yearly":
		last := time.Date(c.Year()+1, a.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		next = time.Date(c.Year()+1, a.Month(), min(a.Day(), last), 0, 0, 0, 0, time.UTC)
	default:
		return ""
	}
	if next.Year() > 9999 || !next.After(c) {
		return ""
	}
	return next.Format("2006-01-02")
}
