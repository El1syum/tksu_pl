package repository

import (
	"context"
	"database/sql"
	"github.com/El1syum/tksu_pl/internal/models"
	"time"
)

type RecurringRepository struct{ db *sql.DB }

const recurringSelect = `SELECT r.id,r.user_id,r.category_id,r.amount,r.description,r.frequency,r.anchor_date,r.next_date,r.end_date,r.enabled,c.name FROM recurring_expenses r JOIN categories c ON c.id=r.category_id`

func scanRecurring(s interface{ Scan(...any) error }) (r models.Recurring, err error) {
	err = s.Scan(&r.ID, &r.UserID, &r.CategoryID, &r.Amount, &r.Description, &r.Frequency, &r.AnchorDate, &r.NextDate, &r.EndDate, &r.Enabled, &r.Category)
	return
}
func (r *RecurringRepository) List(ctx context.Context, userID int64) ([]models.Recurring, error) {
	rows, err := r.db.QueryContext(ctx, recurringSelect+" WHERE r.user_id=? ORDER BY r.enabled DESC,r.next_date,r.id", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Recurring{}
	for rows.Next() {
		v, err := scanRecurring(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *RecurringRepository) Get(ctx context.Context, userID, id int64) (models.Recurring, error) {
	return scanRecurring(r.db.QueryRowContext(ctx, recurringSelect+" WHERE r.user_id=? AND r.id=?", userID, id))
}
func (r *RecurringRepository) Create(ctx context.Context, v models.Recurring) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO recurring_expenses(user_id,category_id,amount,description,frequency,anchor_date,next_date,end_date,enabled) VALUES(?,?,?,?,?,?,?,?,1)`, v.UserID, v.CategoryID, v.Amount, v.Description, v.Frequency, v.AnchorDate, v.NextDate, v.EndDate)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
func (r *RecurringRepository) Update(ctx context.Context, v models.Recurring) error {
	res, err := r.db.ExecContext(ctx, `UPDATE recurring_expenses SET category_id=?,amount=?,description=?,frequency=?,anchor_date=?,next_date=?,end_date=? WHERE id=? AND user_id=?`, v.CategoryID, v.Amount, v.Description, v.Frequency, v.AnchorDate, v.NextDate, v.EndDate, v.ID, v.UserID)
	return affected(res, err)
}
func (r *RecurringRepository) Delete(ctx context.Context, userID, id int64) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM recurring_expenses WHERE user_id=? AND id=?", userID, id)
	return affected(res, err)
}

// Paused dates are skipped on resume; already created expenses are never changed.
func (r *RecurringRepository) SetEnabled(ctx context.Context, userID, id int64, enabled bool, today string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	v, err := scanRecurring(tx.QueryRowContext(ctx, recurringSelect+" WHERE r.user_id=? AND r.id=?", userID, id))
	if err != nil {
		return err
	}
	if enabled && !v.Enabled {
		if next := v.ResumeDate(today); next != "" {
			v.NextDate = next
		} else {
			enabled = false
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE recurring_expenses SET enabled=?,next_date=? WHERE id=? AND user_id=?", enabled, v.NextDate, id, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// GenerateDue atomically claims dates and creates expenses. A batch is bounded to
// keep startup and cancellation fast; the next tick continues from next_date.
// userID=0 processes all users, otherwise only the selected account.
func (r *RecurringRepository) GenerateDue(ctx context.Context, now time.Time, userID int64) (int, error) {
	today := now.Format("2006-01-02")
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, recurringSelect+" WHERE r.enabled=1 AND r.next_date<=? AND (?=0 OR r.user_id=?) ORDER BY r.next_date,r.id LIMIT 1000", today, userID, userID)
	if err != nil {
		return 0, err
	}
	rules := []models.Recurring{}
	for rows.Next() {
		v, err := scanRecurring(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		rules = append(rules, v)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	generated, processed := 0, 0
	for _, v := range rules {
		for v.Enabled && v.NextDate <= today && processed < 1000 {
			if v.EndDate != "" && v.NextDate > v.EndDate {
				v.Enabled = false
				break
			}
			res, err := tx.ExecContext(ctx, "INSERT INTO recurring_occurrences(recurring_id,scheduled_date) VALUES(?,?) ON CONFLICT DO NOTHING", v.ID, v.NextDate)
			if err != nil {
				return 0, err
			}
			claimed, err := res.RowsAffected()
			if err != nil {
				return 0, err
			}
			if claimed > 0 {
				if _, err = tx.ExecContext(ctx, "INSERT INTO expenses(user_id,category_id,amount,description,date,recurring_id) VALUES(?,?,?,?,?,?)", v.UserID, v.CategoryID, v.Amount, v.Description, v.NextDate, v.ID); err != nil {
					return 0, err
				}
				generated++
			}
			processed++
			next := models.NextRecurringDate(v.NextDate, v.AnchorDate, v.Frequency)
			if next == "" {
				v.Enabled = false
				break
			}
			v.NextDate = next
		}
		if v.EndDate != "" && v.NextDate > v.EndDate {
			v.Enabled = false
		}
		if _, err = tx.ExecContext(ctx, "UPDATE recurring_expenses SET next_date=?,enabled=? WHERE id=?", v.NextDate, v.Enabled, v.ID); err != nil {
			return 0, err
		}
		if processed >= 1000 {
			break
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return generated, nil
}
