package repository

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/El1syum/tksu_pl/internal/models"
)

type ExpenseRepository struct{ db *sql.DB }

const expenseSelect = `SELECT e.id,e.user_id,e.amount,e.description,e.date,e.category_id,c.name,c.color,c.icon,COALESCE(e.recurring_id,0) FROM expenses e JOIN categories c ON c.id=e.category_id`

func scanExpense(s interface{ Scan(...any) error }) (e models.Expense, err error) {
	err = s.Scan(&e.ID, &e.UserID, &e.Amount, &e.Description, &e.Date, &e.CategoryID, &e.Category, &e.Color, &e.Icon, &e.RecurringID)
	return
}
func where(userID int64, f models.Filter) (string, []any) {
	q := " WHERE e.user_id=?"
	args := []any{userID}
	if f.CategoryID > 0 {
		q += " AND e.category_id=?"
		args = append(args, f.CategoryID)
	}
	if f.From != "" {
		q += " AND e.date>=?"
		args = append(args, f.From)
	}
	if f.To != "" {
		q += " AND e.date<=?"
		args = append(args, f.To)
	}
	return q, args
}
func (r *ExpenseRepository) Create(ctx context.Context, e models.Expense) (int64, error) {
	res, err := r.db.ExecContext(ctx, "INSERT INTO expenses(user_id,amount,description,date,category_id) VALUES(?,?,?,?,?)", e.UserID, e.Amount, e.Description, e.Date, e.CategoryID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
func (r *ExpenseRepository) GetByID(ctx context.Context, userID, id int64) (models.Expense, error) {
	return scanExpense(r.db.QueryRowContext(ctx, expenseSelect+" WHERE e.user_id=? AND e.id=?", userID, id))
}
func (r *ExpenseRepository) GetAll(ctx context.Context, userID int64, f models.Filter, limit, offset int) ([]models.Expense, error) {
	q, args := where(userID, f)
	q = expenseSelect + q + " ORDER BY e.date DESC,e.id DESC"
	if limit > 0 {
		q += " LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Expense{}
	for rows.Next() {
		e, err := scanExpense(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (r *ExpenseRepository) Update(ctx context.Context, e models.Expense) error {
	res, err := r.db.ExecContext(ctx, "UPDATE expenses SET amount=?,description=?,date=?,category_id=? WHERE id=? AND user_id=?", e.Amount, e.Description, e.Date, e.CategoryID, e.ID, e.UserID)
	return affected(res, err)
}
func (r *ExpenseRepository) Delete(ctx context.Context, userID, id int64) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM expenses WHERE user_id=? AND id=?", userID, id)
	return affected(res, err)
}
func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return err
}
func (r *ExpenseRepository) Categories(ctx context.Context) ([]models.Category, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id,name,color,icon FROM categories ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Category{}
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Color, &c.Icon); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (r *ExpenseRepository) Totals(ctx context.Context, userID int64, f models.Filter) (t models.Totals, err error) {
	q, args := where(userID, f)
	err = r.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(e.amount),0),COUNT(*) FROM expenses e"+q, args...).Scan(&t.Amount, &t.Count)
	return
}
func (r *ExpenseRepository) ByCategory(ctx context.Context, userID int64, f models.Filter) ([]models.CategoryStat, error) {
	q, args := where(userID, f)
	rows, err := r.db.QueryContext(ctx, "SELECT c.name,c.color,SUM(e.amount),COUNT(*) FROM expenses e JOIN categories c ON c.id=e.category_id"+q+" GROUP BY c.id ORDER BY SUM(e.amount) DESC,c.id", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CategoryStat{}
	for rows.Next() {
		var s models.CategoryStat
		if err := rows.Scan(&s.Name, &s.Color, &s.Amount, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func (r *ExpenseRepository) ByMonth(ctx context.Context, userID int64, year int) ([]models.MonthStat, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT CAST(substr(date,6,2) AS INTEGER),SUM(amount) FROM expenses WHERE user_id=? AND date>=? AND date<=? GROUP BY substr(date,6,2)", userID, fmt.Sprintf("%04d-01-01", year), fmt.Sprintf("%04d-12-31", year))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.MonthStat, 12)
	for i := range out {
		out[i].Month = i + 1
	}
	for rows.Next() {
		var s models.MonthStat
		if err := rows.Scan(&s.Month, &s.Amount); err != nil {
			return nil, err
		}
		if s.Month >= 1 && s.Month <= 12 {
			out[s.Month-1] = s
		}
	}
	return out, rows.Err()
}
