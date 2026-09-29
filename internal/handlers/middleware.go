package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"github.com/El1syum/tksu_pl/internal/models"
	"net/http"
	"strings"
	"time"
)

type contextKey uint8

const (
	userKey contextKey = iota
	csrfKey
)

func currentUser(r *http.Request) *models.User {
	u, _ := r.Context().Value(userKey).(*models.User)
	return u
}
func (h *Handler) digest(s string) string {
	mac := hmac.New(sha256.New, []byte(h.config.SessionSecret))
	mac.Write([]byte(s))
	return hex.EncodeToString(mac.Sum(nil))
}
func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func (h *Handler) cookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: age, HttpOnly: true, Secure: h.config.CookieSecure, SameSite: http.SameSiteLaxMode})
}
func (h *Handler) session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("session"); err == nil && len(c.Value) == 64 {
			u, err := h.store.Sessions.User(r.Context(), h.digest("session:"+c.Value))
			if err == nil {
				r = r.WithContext(context.WithValue(r.Context(), userKey, &u))
			} else if !errors.Is(err, sql.ErrNoRows) {
				h.internal(w, r, err)
				return
			} else {
				h.cookie(w, "session", "", -1)
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r) == nil {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				h.json(w, 401, map[string]string{"error": "Войдите в аккаунт"})
			} else {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (h *Handler) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The diagnostic route rejects every non-GET method with 405, even without a session or form token.
		if r.URL.Path == "/ping" {
			next.ServeHTTP(w, r)
			return
		}
		var token string
		if c, err := r.Cookie("csrf"); err == nil {
			parts := strings.Split(c.Value, ".")
			if len(parts) == 2 && len(parts[0]) == 64 && hmac.Equal([]byte(parts[1]), []byte(h.digest("csrf:"+parts[0]))) {
				token = c.Value
			}
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			if origin := r.Header.Get("Origin"); origin != "" && origin != h.config.PublicURL {
				h.fail(w, r, 403)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
			if err := r.ParseForm(); err != nil {
				var max *http.MaxBytesError
				if errors.As(err, &max) {
					h.fail(w, r, 413)
				} else {
					h.fail(w, r, 400)
				}
				return
			}
			if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(r.PostForm.Get("csrf"))) != 1 {
				h.fail(w, r, 403)
				return
			}
		}
		if token == "" {
			v := randomToken()
			token = v + "." + h.digest("csrf:"+v)
			h.cookie(w, "csrf", token, 7*24*3600)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), csrfKey, token)))
	})
}
func (h *Handler) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if h.config.CookieSecure {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if strings.HasPrefix(r.URL.Path, "/static/") {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		} else {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (w *responseRecorder) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
		w.ResponseWriter.WriteHeader(code)
	}
}
func (w *responseRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *responseRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (h *Handler) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = 200
		}
		h.log.Info("http request", "method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", time.Since(start).Milliseconds())
	})
}
func (h *Handler) recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				h.log.Error("handler panic", "path", r.URL.Path, "panic", v)
				h.fail(w, r, 500)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
