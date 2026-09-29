package repository

import (
	"context"
	"database/sql"
	"github.com/El1syum/tksu_pl/internal/models"
)

type BudgetRepository struct{ db *sql.DB }

func (r *BudgetRepository) Set(ctx context.Context, userID, categoryID int64, month string, amount int64) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO budgets(user_id,category_id,month,amount) VALUES(?,?,?,?) ON CONFLICT(user_id,category_id,month) DO UPDATE SET amount=excluded.amount`, userID, categoryID, month, amount)
	return err
}
func (r *BudgetRepository) Delete(ctx context.Context, userID, categoryID int64, month string) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM budgets WHERE user_id=? AND category_id=? AND month=?", userID, categoryID, month)
	return affected(res, err)
}
func (r *BudgetRepository) List(ctx context.Context, userID int64, month string) ([]models.Budget, error) {
	from, to := models.MonthBounds(month)
	rows, err := r.db.QueryContext(ctx, `SELECT b.category_id,b.amount,b.month,c.name,COALESCE(SUM(e.amount),0)
 FROM budgets b JOIN categories c ON c.id=b.category_id
 LEFT JOIN expenses e ON e.user_id=b.user_id AND e.category_id=b.category_id AND e.date>=? AND e.date<=?
 WHERE b.user_id=? AND b.month=? GROUP BY b.category_id,b.amount,b.month,c.name ORDER BY b.category_id`, from, to, userID, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Budget{}
	for rows.Next() {
		var b models.Budget
		if err := rows.Scan(&b.CategoryID, &b.Amount, &b.Month, &b.Category, &b.Spent); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
