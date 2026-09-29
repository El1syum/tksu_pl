package repository

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/El1syum/tksu_pl/migrations"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
)

type Store struct {
	DB        *sql.DB
	Expenses  *ExpenseRepository
	Users     *UserRepository
	Sessions  *SessionRepository
	Budgets   *BudgetRepository
	Recurring *RecurringRepository
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*Store, error) { db.Close(); return nil, err }
	if _, err = db.Exec("PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;"); err != nil {
		return fail(err)
	}
	if err = migrate(db); err != nil {
		return fail(err)
	}
	return &Store{DB: db, Expenses: &ExpenseRepository{db}, Users: &UserRepository{db}, Sessions: &SessionRepository{db}, Budgets: &BudgetRepository{db}, Recurring: &RecurringRepository{db}}, nil
}
func migrate(db *sql.DB) error {
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY)"); err != nil {
		return err
	}
	files, err := migrations.Files.ReadDir(".")
	if err != nil {
		return err
	}
	for _, f := range files {
		if filepath.Ext(f.Name()) != ".sql" {
			continue
		}
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE name=?", f.Name()).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		script, err := migrations.Files.ReadFile(f.Name())
		if err != nil {
			return err
		}
		tx, err := db.BeginTx(context.Background(), nil)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(string(script)); err == nil {
			_, err = tx.Exec("INSERT INTO schema_migrations(name) VALUES(?)", f.Name())
		}
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", f.Name(), err)
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
