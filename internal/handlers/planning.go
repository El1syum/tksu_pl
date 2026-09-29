package handlers

import (
	"github.com/El1syum/tksu_pl/internal/models"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) setTheme(w http.ResponseWriter, r *http.Request) {
	theme := r.PostForm.Get("theme")
	if theme != "light" && theme != "dark" && theme != "system" {
		h.fail(w, r, 400)
		return
	}
	h.cookie(w, "theme", theme, 365*24*3600)
	destination := "/"
	if u, err := url.Parse(r.PostForm.Get("return_to")); err == nil && !u.IsAbs() && u.Host == "" && strings.HasPrefix(u.Path, "/") && !strings.HasPrefix(u.Path, "//") && !strings.Contains(u.Path, "\\") {
		destination = u.RequestURI()
	}
	http.Redirect(w, r, destination, 303)
}
func (h *Handler) budgetView(w http.ResponseWriter, r *http.Request, p Page, status int) {
	var err error
	p.Title = "Бюджеты"
	p.Active = "budgets"
	if p.Categories, err = h.store.Expenses.Categories(r.Context()); err != nil {
		h.internal(w, r, err)
		return
	}
	if p.Budgets, err = h.store.Budgets.List(r.Context(), currentUser(r).ID, p.Month); err != nil {
		h.internal(w, r, err)
		return
	}
	h.render(w, r, "budgets", status, p)
}
func (h *Handler) budgetsPage(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	if !models.ValidMonth(month) {
		h.fail(w, r, 400)
		return
	}
	p := Page{Month: month}
	p.Notice = map[string]string{"saved": "Лимит сохранён", "deleted": "Лимит удалён. Расходы сохранены."}[r.URL.Query().Get("notice")]
	if raw := r.URL.Query().Get("edit"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			h.fail(w, r, 400)
			return
		}
		items, err := h.store.Budgets.List(r.Context(), currentUser(r).ID, month)
		if err != nil {
			h.internal(w, r, err)
			return
		}
		found := false
		for _, b := range items {
			if b.CategoryID == id {
				p.Form = Form{CategoryID: raw, Amount: models.Decimal(b.Amount)}
				found = true
				break
			}
		}
		if !found {
			h.fail(w, r, 404)
			return
		}
	}
	h.budgetView(w, r, p, 200)
}
func (h *Handler) saveBudget(w http.ResponseWriter, r *http.Request) {
	month := r.PostForm.Get("month")
	if !models.ValidMonth(month) {
		h.fail(w, r, 400)
		return
	}
	p := Page{Month: month, Form: Form{Amount: r.PostForm.Get("amount"), CategoryID: r.PostForm.Get("category_id")}, Errors: map[string]string{}}
	amount, err := models.ParseAmount(p.Form.Amount)
	if err != nil {
		p.Errors["amount"] = err.Error()
	}
	id, _ := strconv.ParseInt(p.Form.CategoryID, 10, 64)
	cats, err := h.store.Expenses.Categories(r.Context())
	if err != nil {
		h.internal(w, r, err)
		return
	}
	if !validCategory(cats, id) {
		p.Errors["category_id"] = "Выберите категорию из списка"
	}
	if len(p.Errors) > 0 {
		h.budgetView(w, r, p, 422)
		return
	}
	if err = h.store.Budgets.Set(r.Context(), currentUser(r).ID, id, month, amount); err != nil {
		h.internal(w, r, err)
		return
	}
	http.Redirect(w, r, "/budgets?month="+month+"&notice=saved", 303)
}
func (h *Handler) deleteBudget(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	month := r.PostForm.Get("month")
	if err != nil || id < 1 || !models.ValidMonth(month) {
		h.fail(w, r, 400)
		return
	}
	if err = h.store.Budgets.Delete(r.Context(), currentUser(r).ID, id, month); err != nil {
		h.notFoundOrError(w, r, err)
		return
	}
	http.Redirect(w, r, "/budgets?month="+month+"&notice=deleted", 303)
}

func (h *Handler) recurringList(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.Recurring.List(r.Context(), currentUser(r).ID)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	p := Page{Title: "Регулярные траты", Active: "recurring", Recurring: items}
	p.Notice = map[string]string{"created": "Расписание создано", "updated": "Расписание обновлено", "paused": "Расписание приостановлено", "resumed": "Расписание возобновлено. Пропущенные во время паузы даты не начисляются.", "deleted": "Расписание удалено. Созданные расходы сохранены."}[r.URL.Query().Get("notice")]
	h.render(w, r, "recurring", 200, p)
}
func (h *Handler) recurringForm(w http.ResponseWriter, r *http.Request, p Page, status int) {
	cats, err := h.store.Expenses.Categories(r.Context())
	if err != nil {
		h.internal(w, r, err)
		return
	}
	p.Categories = cats
	p.Active = "recurring"
	p.Title = "Новое расписание"
	if p.Edit {
		p.Title = "Изменить расписание"
	}
	h.render(w, r, "recurring_form", status, p)
}
func (h *Handler) recurringNew(w http.ResponseWriter, r *http.Request) {
	h.recurringForm(w, r, Page{Action: "/recurring", Form: Form{Date: time.Now().Format("2006-01-02"), Frequency: "monthly"}}, 200)
}
func (h *Handler) findRecurring(w http.ResponseWriter, r *http.Request) (models.Recurring, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		h.fail(w, r, 404)
		return models.Recurring{}, false
	}
	v, err := h.store.Recurring.Get(r.Context(), currentUser(r).ID, id)
	if err != nil {
		h.notFoundOrError(w, r, err)
		return v, false
	}
	return v, true
}
func (h *Handler) recurringEdit(w http.ResponseWriter, r *http.Request) {
	v, ok := h.findRecurring(w, r)
	if !ok {
		return
	}
	h.recurringForm(w, r, Page{Edit: true, Action: r.URL.Path, Form: Form{Amount: models.Decimal(v.Amount), CategoryID: strconv.FormatInt(v.CategoryID, 10), Description: v.Description, Date: max(v.NextDate, time.Now().Format("2006-01-02")), EndDate: v.EndDate, Frequency: v.Frequency}}, 200)
}
func (h *Handler) validateRecurring(r *http.Request) (models.Recurring, Form, map[string]string, error) {
	e, f, errs, err := h.validateExpense(r)
	if err != nil {
		return models.Recurring{}, f, errs, err
	}
	f.Frequency = r.PostForm.Get("frequency")
	f.EndDate = r.PostForm.Get("end_date")
	if models.FrequencyLabel(f.Frequency) == "" {
		errs["frequency"] = "Выберите периодичность из списка"
	}
	if models.ValidDate(f.Date) && f.Date < time.Now().Format("2006-01-02") {
		errs["date"] = "Первая или следующая дата не может быть раньше сегодняшней"
	}
	if f.EndDate != "" && (!models.ValidDate(f.EndDate) || f.EndDate < f.Date) {
		errs["end_date"] = "Дата окончания должна быть не раньше следующей траты"
	}
	return models.Recurring{UserID: e.UserID, CategoryID: e.CategoryID, Amount: e.Amount, Description: e.Description, Frequency: f.Frequency, AnchorDate: e.Date, NextDate: e.Date, EndDate: f.EndDate, Enabled: true}, f, errs, nil
}
func (h *Handler) generateUser(w http.ResponseWriter, r *http.Request) bool {
	if _, err := h.store.Recurring.GenerateDue(r.Context(), time.Now(), currentUser(r).ID); err != nil {
		h.internal(w, r, err)
		return false
	}
	return true
}
func (h *Handler) recurringCreate(w http.ResponseWriter, r *http.Request) {
	v, f, errs, err := h.validateRecurring(r)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	if len(errs) > 0 {
		h.recurringForm(w, r, Page{Action: "/recurring", Form: f, Errors: errs}, 422)
		return
	}
	if _, err = h.store.Recurring.Create(r.Context(), v); err != nil {
		h.internal(w, r, err)
		return
	}
	if !h.generateUser(w, r) {
		return
	}
	http.Redirect(w, r, "/recurring?notice=created", 303)
}
func (h *Handler) recurringUpdate(w http.ResponseWriter, r *http.Request) {
	old, ok := h.findRecurring(w, r)
	if !ok {
		return
	}
	v, f, errs, err := h.validateRecurring(r)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	if len(errs) > 0 {
		h.recurringForm(w, r, Page{Edit: true, Action: r.URL.Path, Form: f, Errors: errs}, 422)
		return
	}
	v.ID = old.ID
	if v.Frequency == old.Frequency && v.NextDate == old.NextDate {
		v.AnchorDate = old.AnchorDate
	}
	if err = h.store.Recurring.Update(r.Context(), v); err != nil {
		h.notFoundOrError(w, r, err)
		return
	}
	if !h.generateUser(w, r) {
		return
	}
	http.Redirect(w, r, "/recurring?notice=updated", 303)
}
func (h *Handler) recurringPause(w http.ResponseWriter, r *http.Request) {
	h.recurringToggle(w, r, false)
}
func (h *Handler) recurringResume(w http.ResponseWriter, r *http.Request) {
	h.recurringToggle(w, r, true)
}
func (h *Handler) recurringToggle(w http.ResponseWriter, r *http.Request, enabled bool) {
	v, ok := h.findRecurring(w, r)
	if !ok {
		return
	}
	if enabled && !v.Enabled && v.ResumeDate(time.Now().Format("2006-01-02")) == "" {
		h.fail(w, r, 400)
		return
	}
	if err := h.store.Recurring.SetEnabled(r.Context(), currentUser(r).ID, v.ID, enabled, time.Now().Format("2006-01-02")); err != nil {
		h.notFoundOrError(w, r, err)
		return
	}
	notice := "paused"
	if enabled {
		notice = "resumed"
		if !h.generateUser(w, r) {
			return
		}
	}
	http.Redirect(w, r, "/recurring?notice="+notice, 303)
}
func (h *Handler) recurringDeletePage(w http.ResponseWriter, r *http.Request) {
	v, ok := h.findRecurring(w, r)
	if !ok {
		return
	}
	h.render(w, r, "recurring_delete", 200, Page{Title: "Удалить расписание", Active: "recurring", Rule: v, Action: r.URL.Path})
}
func (h *Handler) recurringDelete(w http.ResponseWriter, r *http.Request) {
	v, ok := h.findRecurring(w, r)
	if !ok {
		return
	}
	if err := h.store.Recurring.Delete(r.Context(), currentUser(r).ID, v.ID); err != nil {
		h.notFoundOrError(w, r, err)
		return
	}
	http.Redirect(w, r, "/recurring?notice=deleted", 303)
}
