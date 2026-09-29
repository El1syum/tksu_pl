package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"github.com/El1syum/tksu_pl/internal/config"
	"github.com/El1syum/tksu_pl/internal/models"
	"github.com/El1syum/tksu_pl/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) *Handler {
	t.Helper()
	s, err := repository.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.DB.Close() })
	h, err := New(s, config.Config{SessionSecret: strings.Repeat("s", 32), PublicURL: "http://example.com"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

type client struct {
	t       *testing.T
	handler http.Handler
	cookies map[string]*http.Cookie
}

func (c *client) request(method, path string, form url.Values, want int) *httptest.ResponseRecorder {
	c.t.Helper()
	if form == nil {
		form = url.Values{}
	}
	if method == "POST" {
		if token, ok := c.cookies["csrf"]; ok {
			form.Set("csrf", token.Value)
		}
	}
	r := httptest.NewRequest(method, "http://example.com"+path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range c.cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	c.handler.ServeHTTP(w, r)
	for _, cookie := range w.Result().Cookies() {
		if cookie.MaxAge < 0 {
			delete(c.cookies, cookie.Name)
		} else {
			c.cookies[cookie.Name] = cookie
		}
	}
	if w.Code != want {
		c.t.Fatalf("%s %s: status %d want %d; body=%s", method, path, w.Code, want, w.Body.String())
	}
	return w
}
func makeClient(t *testing.T, h *Handler, email string) *client {
	t.Helper()
	c := &client{t, h.Routes(), map[string]*http.Cookie{}}
	c.request("GET", "/register", nil, 200)
	c.request("POST", "/register", url.Values{"email": {email}, "password": {"correct-horse-2026"}, "password_confirm": {"correct-horse-2026"}}, 303)
	c.request("GET", "/login", nil, 200)
	c.request("POST", "/login", url.Values{"email": {email}, "password": {"correct-horse-2026"}}, 303)
	c.request("GET", "/expenses", nil, 200)
	return c
}
func expenseForm(amount, description, date, category string) url.Values {
	return url.Values{"amount": {amount}, "description": {description}, "date": {date}, "category_id": {category}}
}
func TestFullJourneyAndIsolation(t *testing.T) {
	h := fixture(t)
	a := makeClient(t, h, "alice@example.com")
	b := makeClient(t, h, "bob@example.com")
	user, err := h.store.Users.ByEmail(context.Background(), "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("correct-horse-2026")) != nil || user.PasswordHash == "correct-horse-2026" {
		t.Fatal("password not hashed")
	}
	empty := a.request("GET", "/expenses", nil, 200)
	if !strings.Contains(empty.Body.String(), "У каждой истории есть начало") {
		t.Fatal("missing empty state")
	}
	invalid := a.request("POST", "/expenses", expenseForm("-100", "keep my input", "2026-02-30", "999"), 422)
	for _, s := range []string{"keep my input", "aria-invalid", "-100"} {
		if !strings.Contains(invalid.Body.String(), s) {
			t.Fatalf("invalid form missing %s", s)
		}
	}
	a.request("POST", "/expenses", expenseForm("10,25", "<script>alert(1)</script>", "2026-01-01", "1"), 303)
	a.request("POST", "/expenses", expenseForm("20.50", "=HYPERLINK(\"example\")", "2026-02-01", "2"), 303)
	b.request("POST", "/expenses", expenseForm("999", "BOB PRIVATE", "2026-01-01", "1"), 303)
	list := a.request("GET", "/expenses", nil, 200)
	if strings.Contains(list.Body.String(), "BOB PRIVATE") || strings.Contains(list.Body.String(), "<script>alert(1)</script>") {
		t.Fatal("privacy or XSS failure")
	}
	filtered := a.request("GET", "/expenses?category=1&from=2026-01-01&to=2026-01-01", nil, 200)
	if strings.Contains(filtered.Body.String(), "HYPERLINK") {
		t.Fatal("filter failure")
	}
	a.request("GET", "/expenses?from=2026-12-01&to=2026-01-01", nil, 400)
	a.request("GET", "/expenses?category=99", nil, 400)
	a.request("GET", "/expenses/3/edit", nil, 404)
	a.request("GET", "/expenses/3/delete", nil, 404)
	a.request("POST", "/expenses/3/edit", expenseForm("5", "stolen", "2026-01-01", "1"), 404)
	a.request("POST", "/expenses/3/delete", nil, 404)
	a.request("GET", "/expenses/1/edit", nil, 200)
	a.request("POST", "/expenses/1/edit", expenseForm("12.30", "Обед", "2026-01-31", "1"), 303)
	a.request("GET", "/stats", nil, 200)
	cats := a.request("GET", "/api/stats/by-category", nil, 200)
	if !strings.HasPrefix(cats.Header().Get("Content-Type"), "application/json") {
		t.Fatal("incorrect content type")
	}
	var categories []models.CategoryStat
	if err := json.Unmarshal(cats.Body.Bytes(), &categories); err != nil || len(categories) != 2 || categories[0].Amount != 2050 {
		t.Fatalf("stats: %s %v", cats.Body.String(), err)
	}
	months := a.request("GET", "/api/stats/by-month?year=2026", nil, 200)
	var monthly []models.MonthStat
	if err := json.Unmarshal(months.Body.Bytes(), &monthly); err != nil || len(monthly) != 12 || monthly[0].Amount != 1230 || monthly[1].Amount != 2050 {
		t.Fatalf("months: %s %v", months.Body.String(), err)
	}
	a.request("GET", "/api/stats/by-month?year=oops", nil, 400)
	a.request("GET", "/api/stats/by-category?from=bad", nil, 400)
	export := a.request("GET", "/expenses/export", nil, 200)
	if !strings.Contains(export.Header().Get("Content-Disposition"), "attachment;") {
		t.Fatal("missing CSV disposition")
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(export.Body.String(), "\xef\xbb\xbf")))
	reader.Comma = ';'
	records, err := reader.ReadAll()
	if err != nil || len(records) != 3 || records[1][2] != "'=HYPERLINK(\"example\")" || records[2][3] != "12,30" {
		t.Fatalf("CSV %#v %v", records, err)
	}
	filteredCSV := a.request("GET", "/expenses/export?category=1", nil, 200)
	if strings.Contains(filteredCSV.Body.String(), "HYPERLINK") || strings.Contains(filteredCSV.Body.String(), "BOB PRIVATE") {
		t.Fatal("CSV filter/privacy failure")
	}
	a.request("GET", "/expenses/1/delete", nil, 200)
	a.request("POST", "/expenses/1/delete", nil, 303)
	a.request("GET", "/expenses/1/edit", nil, 404)
	token := a.cookies["session"].Value
	a.request("POST", "/logout", nil, 303)
	a.request("GET", "/expenses", nil, 303)
	a.request("GET", "/api/stats/by-month", nil, 401)
	a.cookies["session"] = &http.Cookie{Name: "session", Value: token}
	a.request("GET", "/expenses", nil, 303)
}
func TestCSRFExpiryAndErrors(t *testing.T) {
	h := fixture(t)
	for _, method := range []string{"POST", "HEAD", "DELETE"} {
		w := httptest.NewRecorder()
		h.Routes().ServeHTTP(w, httptest.NewRequest(method, "http://example.com/ping", nil))
		if w.Code != 405 || w.Header().Get("Allow") != "GET" {
			t.Fatalf("unauthenticated %s /ping: %d", method, w.Code)
		}
	}
	c := makeClient(t, h, "security@example.com")
	r := httptest.NewRequest("POST", "http://example.com/expenses", strings.NewReader("amount=1"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range c.cookies {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("CSRF missing: %d", w.Code)
	}
	form := expenseForm("1", "Origin attack", "2026-01-01", "1")
	form.Set("csrf", c.cookies["csrf"].Value)
	r = httptest.NewRequest("POST", "http://example.com/expenses", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://evil.example")
	for _, cookie := range c.cookies {
		r.AddCookie(cookie)
	}
	w = httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("origin: %d", w.Code)
	}
	r = httptest.NewRequest("POST", "http://example.com/expenses", strings.NewReader("description="+strings.Repeat("x", 20000)))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range c.cookies {
		r.AddCookie(cookie)
	}
	w = httptest.NewRecorder()
	h.Routes().ServeHTTP(w, r)
	if w.Code != 413 {
		t.Fatalf("body limit: %d", w.Code)
	}
	c.request("GET", "/missing", nil, 404)
	c.request("GET", "/ping", nil, 200)
	c.request("POST", "/ping", nil, 405)
	panicHandler := h.recovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("test panic") }))
	w = httptest.NewRecorder()
	panicHandler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 500 || !strings.Contains(w.Body.String(), "Нужна небольшая пауза") {
		t.Fatal("panic not recovered")
	}
	c.request("GET", "/ping", nil, 200)
	u, err := h.store.Users.ByEmail(context.Background(), "security@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token := randomToken()
	if err = h.store.Sessions.Create(context.Background(), h.digest("session:"+token), u.ID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	c.cookies["session"] = &http.Cookie{Name: "session", Value: token}
	c.request("GET", "/expenses", nil, 303)
}
func TestLoginValidation(t *testing.T) {
	h := fixture(t)
	c := makeClient(t, h, "test@example.com")
	c.request("POST", "/logout", nil, 303)
	c.request("GET", "/register", nil, 200)
	c.request("POST", "/register", url.Values{"email": {"test@example.com"}, "password": {"correct-horse-2026"}, "password_confirm": {"correct-horse-2026"}}, 422)
	c.request("POST", "/register", url.Values{"email": {"bad"}, "password": {"short"}, "password_confirm": {"different"}}, 422)
	c.request("POST", "/login", url.Values{"email": {"test@example.com"}, "password": {"wrong"}}, 422)
	c.request("POST", "/login", url.Values{"email": {"unknown@example.com"}, "password": {"wrong"}}, 422)
}
func TestPaginationAndCSVInjection(t *testing.T) {
	h := fixture(t)
	c := makeClient(t, h, "pages@example.com")
	u, err := h.store.Users.ByEmail(context.Background(), "pages@example.com")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 26; i++ {
		if _, err = h.store.Expenses.Create(context.Background(), models.Expense{UserID: u.ID, CategoryID: 1, Amount: 100, Description: "Pagination item", Date: "2026-01-01"}); err != nil {
			t.Fatal(err)
		}
	}
	page := c.request("GET", "/expenses?category=1&page=2", nil, 200)
	if strings.Count(page.Body.String(), "class=\"expense-name\"") != 1 || !strings.Contains(page.Body.String(), "category=1") {
		t.Fatal("pagination failed")
	}
	for _, s := range []string{"=1+1", "  @SUM(1)", "+cmd", "-cmd", "\tformula", "\rformula"} {
		if !strings.HasPrefix(csvText(s), "'") {
			t.Errorf("unsafe CSV %q", s)
		}
	}
}
