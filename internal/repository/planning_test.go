package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/El1syum/tksu_pl/internal/models"
	"github.com/El1syum/tksu_pl/migrations"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func planningStore(t *testing.T) (*Store, int64, int64) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "planning.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Close() })
	a, err := s.Users.Create(context.Background(), "a@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Users.Create(context.Background(), "b@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	return s, a, b
}
func date(s string) time.Time { t, _ := time.Parse("2006-01-02", s); return t }
func rule(user int64, frequency, start, end string) models.Recurring {
	return models.Recurring{UserID: user, CategoryID: 1, Amount: 100, Description: "Subscription", Frequency: frequency, AnchorDate: start, NextDate: start, EndDate: end, Enabled: true}
}
func TestBudgetMonthsAndUsers(t *testing.T) {
	s, a, b := planningStore(t)
	ctx := context.Background()
	for _, e := range []models.Expense{{UserID: a, CategoryID: 1, Amount: 10100, Description: "Over", Date: "2026-09-01"}, {UserID: a, CategoryID: 1, Amount: 90000, Description: "Other month", Date: "2026-08-31"}, {UserID: b, CategoryID: 1, Amount: 100000, Description: "Private", Date: "2026-09-01"}} {
		if _, err := s.Expenses.Create(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Budgets.Set(ctx, a, 1, "2026-09", 10000); err != nil {
		t.Fatal(err)
	}
	list, err := s.Budgets.List(ctx, a, "2026-09")
	if err != nil || len(list) != 1 || list[0].Spent != 10100 || !list[0].Over() || list[0].OverBy() != 100 || list[0].Progress() != 10000 || list[0].Remaining() != 0 {
		t.Fatalf("%+v %v", list, err)
	}
	other, err := s.Budgets.List(ctx, b, "2026-09")
	if err != nil || len(other) != 0 {
		t.Fatal("budget isolation", other, err)
	}
	other, err = s.Budgets.List(ctx, a, "2026-10")
	if err != nil || len(other) != 0 {
		t.Fatal("monthly isolation", other, err)
	}
	if err = s.Budgets.Delete(ctx, b, 1, "2026-09"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("foreign delete", err)
	}
	if err = s.Budgets.Set(ctx, a, 1, "2026-09", 20000); err != nil {
		t.Fatal(err)
	}
	list, err = s.Budgets.List(ctx, a, "2026-09")
	if err != nil || len(list) != 1 || list[0].Reached() || list[0].Remaining() != 9900 {
		t.Fatal("upsert", list, err)
	}
	if err = s.Budgets.Delete(ctx, a, 1, "2026-09"); err != nil {
		t.Fatal(err)
	}
	totals, err := s.Expenses.Totals(ctx, a, models.Filter{})
	if err != nil || totals.Count != 2 {
		t.Fatal("deleting budget changed expenses")
	}
}
func TestRecurringIdempotenceAndOwnership(t *testing.T) {
	s, a, b := planningStore(t)
	ctx := context.Background()
	id, err := s.Recurring.Create(ctx, rule(a, "monthly", "2024-01-31", "2024-03-31"))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := s.Recurring.GenerateDue(ctx, date("2024-04-01"), b); err != nil || n != 0 {
		t.Fatal("foreign generation", n, err)
	}
	if n, err := s.Recurring.GenerateDue(ctx, date("2024-04-01"), 0); err != nil || n != 3 {
		t.Fatal(n, err)
	}
	items, err := s.Expenses.GetAll(ctx, a, models.Filter{}, 0, 0)
	if err != nil || len(items) != 3 || items[0].Date != "2024-03-31" || items[1].Date != "2024-02-29" || items[2].Date != "2024-01-31" || items[0].RecurringID != id {
		t.Fatal(items, err)
	}
	if n, err := s.Recurring.GenerateDue(ctx, date("2024-04-01"), 0); err != nil || n != 0 {
		t.Fatal("duplicate generation", n, err)
	}
	if _, err = s.Recurring.Get(ctx, b, id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("foreign read", err)
	}
	if err = s.Recurring.Delete(ctx, b, id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("foreign delete", err)
	}
	if err = s.Recurring.SetEnabled(ctx, b, id, false, "2024-04-01"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("foreign pause", err)
	}
	foreign := rule(b, "daily", "2024-01-01", "")
	foreign.ID = id
	if err = s.Recurring.Update(ctx, foreign); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("foreign edit", err)
	}
	if err = s.Expenses.Delete(ctx, a, items[2].ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("UPDATE recurring_expenses SET next_date='2024-01-31',enabled=1 WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Recurring.GenerateDue(ctx, date("2024-04-01"), 0); err != nil || n != 0 {
		t.Fatal("deleted occurrence was recreated", n, err)
	}
	if err = s.Recurring.Delete(ctx, a, id); err != nil {
		t.Fatal(err)
	}
	items, err = s.Expenses.GetAll(ctx, a, models.Filter{}, 0, 0)
	if err != nil || len(items) != 2 || items[0].RecurringID != 0 {
		t.Fatal("rule deletion changed history", items, err)
	}
}
func TestRecurringPauseResumeAndUpdate(t *testing.T) {
	s, a, _ := planningStore(t)
	ctx := context.Background()
	id, err := s.Recurring.Create(ctx, rule(a, "daily", "2026-01-01", ""))
	if err != nil {
		t.Fatal(err)
	}
	// Repeating resume on an already active rule must not skip overdue dates.
	if err = s.Recurring.SetEnabled(ctx, a, id, true, "2026-01-03"); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Recurring.GenerateDue(ctx, date("2026-01-03"), 0); err != nil || n != 3 {
		t.Fatal(n, err)
	}
	if err = s.Recurring.SetEnabled(ctx, a, id, false, "2026-01-03"); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Recurring.GenerateDue(ctx, date("2026-01-10"), 0); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if err = s.Recurring.SetEnabled(ctx, a, id, true, "2026-01-10"); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Recurring.GenerateDue(ctx, date("2026-01-10"), 0); err != nil || n != 1 {
		t.Fatal("pause backfilled", n, err)
	}
	v, err := s.Recurring.Get(ctx, a, id)
	if err != nil {
		t.Fatal(err)
	}
	v.Amount = 250
	if err = s.Recurring.Update(ctx, v); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Recurring.GenerateDue(ctx, date("2026-01-11"), 0); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	totals, err := s.Expenses.Totals(ctx, a, models.Filter{})
	if err != nil || totals.Amount != 650 || totals.Count != 5 {
		t.Fatal(totals, err)
	}
}
func TestRecurringRollbackAndConcurrentWorkers(t *testing.T) {
	s, a, _ := planningStore(t)
	ctx := context.Background()
	id, err := s.Recurring.Create(ctx, rule(a, "daily", "2026-01-01", ""))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.Exec("CREATE TRIGGER fail_generated BEFORE INSERT ON expenses WHEN NEW.recurring_id IS NOT NULL BEGIN SELECT RAISE(ABORT,'simulated failure'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Recurring.GenerateDue(ctx, date("2026-01-01"), 0); err == nil {
		t.Fatal("expected generation failure")
	}
	var claimed int
	if err = s.DB.QueryRow("SELECT COUNT(*) FROM recurring_occurrences").Scan(&claimed); err != nil || claimed != 0 {
		t.Fatal("claim not rolled back", claimed, err)
	}
	v, err := s.Recurring.Get(ctx, a, id)
	if err != nil || v.NextDate != "2026-01-01" {
		t.Fatal(v, err)
	}
	if _, err = s.DB.Exec("DROP TRIGGER fail_generated"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Recurring.GenerateDue(ctx, date("2026-01-06"), 0); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	totals, err := s.Expenses.Totals(ctx, a, models.Filter{})
	if err != nil || totals.Count != 6 || totals.Amount != 600 {
		t.Fatal("concurrent duplicate", totals, err)
	}
}
func TestUpgradeFromOriginalSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := migrations.Files.ReadFile("001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(initial)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TABLE schema_migrations(name TEXT PRIMARY KEY); INSERT INTO schema_migrations VALUES('001_initial.sql'); INSERT INTO users(id,email,password_hash) VALUES(1,'old@example.com','hash'); INSERT INTO expenses(user_id,category_id,amount,description,date) VALUES(1,1,12345,'Existing record','2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	items, err := s.Expenses.GetAll(ctx, 1, models.Filter{}, 0, 0)
	if err != nil || len(items) != 1 || items[0].Amount != 12345 || items[0].RecurringID != 0 {
		t.Fatal("migration lost data", items, err)
	}
	_, err = s.Recurring.Create(ctx, rule(1, "weekly", "2026-01-01", ""))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := s.Recurring.GenerateDue(ctx, date("2026-01-01"), 0); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	s.DB.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	if n, err := s.Recurring.GenerateDue(ctx, date("2026-01-01"), 0); err != nil || n != 0 {
		t.Fatal("restart duplicate", n, err)
	}
}
