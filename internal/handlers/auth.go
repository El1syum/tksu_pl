package handlers

import (
	"database/sql"
	"errors"
	"github.com/El1syum/tksu_pl/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"
)

type attempt struct {
	count   int
	expires time.Time
}
type loginLimiter struct {
	mu    sync.Mutex
	items map[string]attempt
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{items: map[string]attempt{}} }
func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, a := range l.items {
		if now.After(a.expires) {
			delete(l.items, k)
		}
	}
	a := l.items[key]
	if a.count >= 10 {
		return false
	}
	if a.count == 0 {
		if len(l.items) >= 5000 {
			return false
		}
		a.expires = now.Add(15 * time.Minute)
	}
	a.count++
	l.items[key] = a
	return true
}
func (l *loginLimiter) clear(key string) { l.mu.Lock(); defer l.mu.Unlock(); delete(l.items, key) }
func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/expenses", 303)
		return
	}
	p := Page{Title: "Вход"}
	if r.URL.Query().Get("registered") == "1" {
		p.Notice = "Аккаунт создан. Войдите, чтобы добавить первую трату."
	}
	h.render(w, r, "auth", 200, p)
}
func (h *Handler) registerPage(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/expenses", 303)
		return
	}
	h.render(w, r, "auth", 200, Page{Title: "Регистрация", Register: true})
}
func authForm(r *http.Request) (Form, string) {
	return Form{Email: strings.ToLower(strings.TrimSpace(r.PostForm.Get("email")))}, r.PostForm.Get("password")
}
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	form, password := authForm(r)
	p := Page{Title: "Регистрация", Register: true, Form: form, Errors: map[string]string{}}
	addr, err := mail.ParseAddress(form.Email)
	if err != nil || addr.Address != form.Email || len(form.Email) > 254 || !strings.Contains(form.Email, ".") {
		p.Errors["email"] = "Введите корректный email, например name@example.com"
	}
	if len(password) < 10 || len(password) > 72 {
		p.Errors["password"] = "Пароль: от 10 до 72 байт (для латиницы — символов)"
	}
	if r.PostForm.Get("password_confirm") != password {
		p.Errors["password_confirm"] = "Пароли не совпадают"
	}
	if len(p.Errors) > 0 {
		h.render(w, r, "auth", 422, p)
		return
	}
	if !h.limiter.allow("register:" + form.Email) {
		h.fail(w, r, 429)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	if _, err = h.store.Users.Create(r.Context(), form.Email, string(hash)); errors.Is(err, repository.ErrEmailExists) {
		p.Errors["email"] = "Этот email уже зарегистрирован. Войдите в аккаунт."
		h.render(w, r, "auth", 422, p)
		return
	} else if err != nil {
		h.internal(w, r, err)
		return
	}
	http.Redirect(w, r, "/login?registered=1", 303)
}

// A valid dummy hash makes unknown accounts take roughly the same time as a wrong password.
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	form, password := authForm(r)
	p := Page{Title: "Вход", Form: form}
	if !h.limiter.allow("login:" + form.Email) {
		w.Header().Set("Retry-After", "900")
		h.fail(w, r, 429)
		return
	}
	u, err := h.store.Users.ByEmail(r.Context(), form.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		h.internal(w, r, err)
		return
	}
	hash := u.PasswordHash
	if err != nil {
		hash = dummyHash
	}
	compareErr := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil || compareErr != nil || len(password) > 72 {
		p.Error = "Неверный email или пароль"
		h.render(w, r, "auth", 422, p)
		return
	}
	token := randomToken()
	if err = h.store.Sessions.Create(r.Context(), h.digest("session:"+token), u.ID, time.Now().Add(7*24*time.Hour)); err != nil {
		h.internal(w, r, err)
		return
	}
	if old, e := r.Cookie("session"); e == nil {
		if e = h.store.Sessions.Delete(r.Context(), h.digest("session:"+old.Value)); e != nil {
			h.log.Error("old session cleanup failed", "error", e)
		}
	}
	h.limiter.clear("login:" + form.Email)
	h.cookie(w, "session", token, 7*24*3600)
	h.cookie(w, "csrf", "", -1)
	http.Redirect(w, r, "/expenses", 303)
}
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session"); err == nil {
		if err = h.store.Sessions.Delete(r.Context(), h.digest("session:"+c.Value)); err != nil {
			h.internal(w, r, err)
			return
		}
	}
	h.cookie(w, "session", "", -1)
	h.cookie(w, "csrf", "", -1)
	http.Redirect(w, r, "/login", 303)
}
