package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/El1syum/tksu_pl/internal/models"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestPersistenceIsolationAndAggregation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expenses.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	u1, err := s.Users.Create(ctx, "first@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := s.Users.Create(ctx, "second@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []models.Expense{{UserID: u1, CategoryID: 1, Amount: 12345, Description: "Food", Date: "2026-01-01"}, {UserID: u1, CategoryID: 1, Amount: 100, Description: "Food2", Date: "2026-01-31"}, {UserID: u1, CategoryID: 2, Amount: 999, Description: "Bus", Date: "2026-02-01"}, {UserID: u2, CategoryID: 1, Amount: 99999, Description: "Private", Date: "2026-01-01"}} {
		if _, err := s.Expenses.Create(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Users.Create(ctx, "FIRST@example.com", "hash"); !errors.Is(err, ErrEmailExists) {
		t.Fatalf("duplicate email: %v", err)
	}
	if err := s.DB.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	cases := []struct {
		f      models.Filter
		count  int
		amount int64
	}{{models.Filter{}, 3, 13444}, {models.Filter{CategoryID: 1}, 2, 12445}, {models.Filter{From: "2026-01-31", To: "2026-02-01"}, 2, 1099}, {models.Filter{CategoryID: 1, From: "2026-01-31", To: "2026-01-31"}, 1, 100}, {models.Filter{From: "2027-01-01"}, 0, 0}, {models.Filter{To: "2026-01-01"}, 1, 12345}}
	for _, tc := range cases {
		all, err := s.Expenses.GetAll(ctx, u1, tc.f, 0, 0)
		if err != nil || len(all) != tc.count {
			t.Fatalf("filter %#v: %v count %d", tc.f, err, len(all))
		}
		totals, err := s.Expenses.Totals(ctx, u1, tc.f)
		if err != nil || totals.Amount != tc.amount || totals.Count != tc.count {
			t.Fatalf("totals %#v %v", totals, err)
		}
	}
	if _, err := s.Expenses.GetByID(ctx, u2, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("foreign record readable")
	}
	if err := s.Expenses.Delete(ctx, u2, 1); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("foreign record deletable")
	}
	if err := s.Expenses.Update(ctx, models.Expense{ID: 1, UserID: u2, CategoryID: 1, Amount: 100, Description: "stolen", Date: "2026-01-01"}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("foreign record editable")
	}
	cats, err := s.Expenses.ByCategory(ctx, u1, models.Filter{})
	if err != nil || len(cats) != 2 || cats[0].Amount != 12445 {
		t.Fatalf("categories %v %v", cats, err)
	}
	months, err := s.Expenses.ByMonth(ctx, u1, 2026)
	if err != nil || len(months) != 12 || months[0].Amount != 12445 || months[1].Amount != 999 || months[11].Amount != 0 {
		t.Fatalf("months %v %v", months, err)
	}
	if _, err := s.Expenses.Create(ctx, models.Expense{UserID: u1, CategoryID: 99, Amount: 1, Description: "bad FK", Date: "2026-01-01"}); err == nil {
		t.Fatal("foreign key not enforced")
	}
	if err := s.Sessions.Create(ctx, "expired", u1, time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Sessions.User(ctx, "expired"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("expired session accepted")
	}
	if err := s.Sessions.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestConcurrentWrites(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	ctx := context.Background()
	uid, err := s.Users.Create(ctx, "parallel@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Expenses.Create(ctx, models.Expense{UserID: uid, CategoryID: 1, Amount: 1, Description: "Concurrent", Date: "2026-01-01"})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	totals, err := s.Expenses.Totals(ctx, uid, models.Filter{})
	if err != nil || totals.Count != 20 || totals.Amount != 20 {
		t.Fatalf("%+v %v", totals, err)
	}
}
