package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBudgetsRecurringAndThemeHTTP(t *testing.T) {
	h := fixture(t)
	a := makeClient(t, h, "planning@example.com")
	b := makeClient(t, h, "other-planning@example.com")
	today := time.Now().Format("2006-01-02")
	month := time.Now().Format("2006-01")
	a.request("GET", "/", nil, 200)
	a.request("GET", "/budgets", nil, 200)
	a.request("GET", "/budgets?month=2026-13", nil, 400)
	a.request("POST", "/budgets", url.Values{"month": {month}, "category_id": {"1"}, "amount": {"0"}}, 422)
	a.request("POST", "/budgets", url.Values{"month": {month}, "category_id": {"1"}, "amount": {"100"}}, 303)
	a.request("POST", "/expenses", expenseForm("125", "Exceeded budget", today, "1"), 303)
	body := a.request("GET", "/expenses", nil, 200).Body.String()
	if !strings.Contains(body, "бюджет превышен на 25,00") {
		t.Fatal("missing over-budget warning")
	}
	if strings.Contains(b.request("GET", "/expenses", nil, 200).Body.String(), "бюджет превышен") {
		t.Fatal("foreign budget warning")
	}
	b.request("GET", "/budgets?edit=1", nil, 404)
	b.request("POST", "/budgets/1/delete", url.Values{"month": {month}}, 404)
	a.request("GET", "/budgets?edit=1", nil, 200)
	a.request("POST", "/budgets", url.Values{"month": {month}, "category_id": {"1"}, "amount": {"200"}}, 303)
	if strings.Contains(a.request("GET", "/expenses", nil, 200).Body.String(), "бюджет превышен") {
		t.Fatal("budget warning not recalculated")
	}
	a.request("GET", "/recurring", nil, 200)
	a.request("GET", "/recurring/new", nil, 200)
	f := expenseForm("50", "Recurring HTTP", today, "1")
	f.Set("frequency", "monthly")
	a.request("POST", "/recurring", f, 303)
	if !strings.Contains(a.request("GET", "/expenses", nil, 200).Body.String(), "Recurring HTTP") {
		t.Fatal("today occurrence missing")
	}
	for _, path := range []string{"/recurring/1/edit", "/recurring/1/delete"} {
		b.request("GET", path, nil, 404)
	}
	for _, path := range []string{"/recurring/1/pause", "/recurring/1/resume", "/recurring/1/delete", "/recurring/1/edit"} {
		b.request("POST", path, f, 404)
	}
	a.request("POST", "/recurring/1/pause", nil, 303)
	a.request("POST", "/recurring/1/resume", nil, 303)
	a.request("GET", "/recurring/1/edit", nil, 200)
	f.Set("frequency", "invalid")
	a.request("POST", "/recurring/1/edit", f, 422)
	f.Set("frequency", "weekly")
	a.request("POST", "/recurring/1/edit", f, 303)
	body = a.request("GET", "/expenses", nil, 200).Body.String()
	if strings.Count(body, "<span>Recurring HTTP") != 1 {
		t.Fatal("same-day schedule edit duplicated expense")
	}
	a.request("GET", "/recurring/1/delete", nil, 200)
	a.request("POST", "/recurring/1/delete", nil, 303)
	if !strings.Contains(a.request("GET", "/expenses", nil, 200).Body.String(), "Recurring HTTP") {
		t.Fatal("schedule removal deleted history")
	}
	f.Set("end_date", today)
	a.request("POST", "/recurring", f, 303)
	a.request("POST", "/recurring/1/resume", nil, 400)
	if !strings.Contains(a.request("GET", "/recurring", nil, 200).Body.String(), "Завершено") {
		t.Fatal("completed schedule should have explicit status")
	}
	a.request("POST", "/budgets/1/delete", url.Values{"month": {month}}, 303)
	theme := a.request("POST", "/theme", url.Values{"theme": {"dark"}, "return_to": {"/stats"}}, 303)
	if theme.Header().Get("Location") != "/stats" {
		t.Fatal("theme return path")
	}
	if !strings.Contains(a.request("GET", "/stats", nil, 200).Body.String(), `data-theme="dark"`) {
		t.Fatal("theme not preserved")
	}
	if !a.cookies["theme"].HttpOnly {
		t.Fatal("theme cookie flags")
	}
	a.request("POST", "/theme", url.Values{"theme": {"invalid"}}, 400)
	for _, path := range []string{"https://evil.example", "//evil.example", "/%2f/evil.example", "/\\evil.example"} {
		w := a.request("POST", "/theme", url.Values{"theme": {"system"}, "return_to": {path}}, 303)
		if w.Header().Get("Location") != "/" {
			t.Fatal("open redirect", path)
		}
	}
}
func TestPlanningAuthentication(t *testing.T) {
	h := fixture(t)
	c := &client{t, h.Routes(), map[string]*http.Cookie{}}
	for _, path := range []string{"/budgets", "/recurring", "/recurring/new", "/recurring/1/edit", "/recurring/1/delete"} {
		c.request("GET", path, nil, 303)
	}
	home := c.request("GET", "/", nil, 200)
	if !strings.Contains(home.Body.String(), "ДОБРО ПОЖАЛОВАТЬ") {
		t.Fatal("missing welcome route")
	}
	c.request("POST", "/theme", url.Values{"theme": {"light"}, "return_to": {"/login"}}, 303)
	if !strings.Contains(c.request("GET", "/login", nil, 200).Body.String(), `data-theme="light"`) {
		t.Fatal("theme should work before login")
	}
}
