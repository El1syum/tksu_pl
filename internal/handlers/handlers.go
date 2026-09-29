package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/El1syum/tksu_pl/internal/config"
	"github.com/El1syum/tksu_pl/internal/models"
	"github.com/El1syum/tksu_pl/internal/repository"
	"github.com/El1syum/tksu_pl/web"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	store     *repository.Store
	config    config.Config
	log       *slog.Logger
	templates map[string]*template.Template
	limiter   *loginLimiter
}
type Form struct{ Amount, Description, Date, CategoryID, Email string }
type Page struct {
	Title, Active, CSRF, Error, Notice, Action, FilterQuery, PrevURL, NextURL, From, To string
	User                                                                                *models.User
	Categories                                                                          []models.Category
	Expenses                                                                            []models.Expense
	CategoryID                                                                          int64
	Total, MonthTotal, Average                                                          int64
	Count, Page, Pages, Year, Status                                                    int
	Filtered, Edit, Register                                                            bool
	Form                                                                                Form
	Errors                                                                              map[string]string
	Expense                                                                             models.Expense
}

func New(store *repository.Store, c config.Config, logger *slog.Logger) (*Handler, error) {
	h := &Handler{store: store, config: c, log: logger, templates: map[string]*template.Template{}, limiter: newLoginLimiter()}
	funcs := template.FuncMap{"money": models.Money, "decimal": models.Decimal, "date": func(s string) string {
		t, e := time.Parse("2006-01-02", s)
		if e != nil {
			return s
		}
		return t.Format("02.01.2006")
	}, "str": func(n int64) string { return strconv.FormatInt(n, 10) }, "initial": func(s string) string {
		for _, r := range s {
			return strings.ToUpper(string(r))
		}
		return "?"
	}}
	for _, name := range []string{"auth", "expenses", "expense_form", "delete", "stats", "about", "404", "500", "error"} {
		t, err := template.New("layout").Funcs(funcs).ParseFS(web.Files, "templates/layout.html", "templates/"+name+".html")
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", name, err)
		}
		h.templates[name] = t
	}
	return h, nil
}
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r) != nil {
			http.Redirect(w, r, "/expenses", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
	})
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			h.fail(w, r, 405)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("pong"))
	})
	mux.HandleFunc("GET /about", func(w http.ResponseWriter, r *http.Request) {
		h.render(w, r, "about", 200, Page{Title: "О проекте", Active: "about"})
	})
	mux.HandleFunc("GET /register", h.registerPage)
	mux.HandleFunc("POST /register", h.register)
	mux.HandleFunc("GET /login", h.loginPage)
	mux.HandleFunc("POST /login", h.login)
	mux.Handle("POST /logout", h.requireAuth(http.HandlerFunc(h.logout)))
	for route, fn := range map[string]http.HandlerFunc{
		"GET /expenses": h.list, "GET /expenses/new": h.newExpense, "POST /expenses": h.createExpense,
		"GET /expenses/{id}/edit": h.editExpense, "POST /expenses/{id}/edit": h.updateExpense,
		"GET /expenses/{id}/delete": h.deletePage, "POST /expenses/{id}/delete": h.deleteExpense,
		"GET /stats": h.statsPage, "GET /api/stats/by-category": h.statsCategory, "GET /api/stats/by-month": h.statsMonth,
		"GET /expenses/export": h.exportCSV,
	} {
		mux.Handle(route, h.requireAuth(fn))
	}
	static, _ := fs.Sub(web.Files, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { h.fail(w, r, 404) })
	return h.logging(h.recovery(h.security(h.session(h.csrf(mux)))))
}
func (h *Handler) render(w http.ResponseWriter, r *http.Request, name string, status int, p Page) {
	p.User = currentUser(r)
	p.CSRF, _ = r.Context().Value(csrfKey).(string)
	var b bytes.Buffer
	if err := h.templates[name].ExecuteTemplate(&b, "layout", p); err != nil {
		h.log.Error("template execution failed", "template", name, "error", err)
		http.Error(w, "Не удалось отобразить страницу", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write(b.Bytes())
}
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, status int) {
	message := map[int]string{400: "Проверьте параметры запроса.", 403: "Форма устарела. Обновите страницу и повторите действие.", 404: "Кажется, здесь ничего нет.", 405: "Этот метод запроса не поддерживается.", 413: "Слишком большой запрос.", 429: "Слишком много попыток. Попробуйте через 15 минут.", 500: "Не удалось выполнить запрос. Попробуйте ещё раз чуть позже."}[status]
	if strings.HasPrefix(r.URL.Path, "/api/") {
		h.json(w, status, map[string]string{"error": message})
		return
	}
	name := "error"
	if status == 404 {
		name = "404"
	}
	if status == 500 {
		name = "500"
	}
	h.render(w, r, name, status, Page{Title: "Ошибка", Status: status, Error: message})
}
func (h *Handler) internal(w http.ResponseWriter, r *http.Request, err error) {
	h.log.Error("request failed", "path", r.URL.Path, "error", err)
	h.fail(w, r, 500)
}
func (h *Handler) notFoundOrError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		h.fail(w, r, 404)
	} else {
		h.internal(w, r, err)
	}
}
func (h *Handler) json(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.log.Error("JSON write failed", "error", err)
	}
}
