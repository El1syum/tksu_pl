package repository

import (
	"context"
	"database/sql"
	"errors"
	"github.com/El1syum/tksu_pl/internal/models"
	"modernc.org/sqlite"
	"time"
)

var ErrEmailExists = errors.New("email already exists")

type UserRepository struct{ db *sql.DB }

func (r *UserRepository) Create(ctx context.Context, email, hash string) (int64, error) {
	res, err := r.db.ExecContext(ctx, "INSERT INTO users(email,password_hash) VALUES(?,?)", email, hash)
	if err != nil {
		var se *sqlite.Error
		if errors.As(err, &se) && se.Code() == 2067 {
			return 0, ErrEmailExists
		}
		return 0, err
	}
	return res.LastInsertId()
}
func (r *UserRepository) ByEmail(ctx context.Context, email string) (u models.User, err error) {
	err = r.db.QueryRowContext(ctx, "SELECT id,email,password_hash FROM users WHERE email=?", email).Scan(&u.ID, &u.Email, &u.PasswordHash)
	return
}

type SessionRepository struct{ db *sql.DB }

func (r *SessionRepository) Create(ctx context.Context, id string, userID int64, expires time.Time) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO sessions(session_id,user_id,expires_at) VALUES(?,?,?)", id, userID, expires.Unix())
	return err
}
func (r *SessionRepository) User(ctx context.Context, id string) (u models.User, err error) {
	err = r.db.QueryRowContext(ctx, "SELECT u.id,u.email FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.session_id=? AND s.expires_at>?", id, time.Now().Unix()).Scan(&u.ID, &u.Email)
	return
}
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE session_id=?", id)
	return err
}
func (r *SessionRepository) Cleanup(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at<=?", time.Now().Unix())
	return err
}
