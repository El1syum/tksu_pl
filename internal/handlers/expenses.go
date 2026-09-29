package handlers

import (
	"errors"
	"github.com/El1syum/tksu_pl/internal/models"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func parseFilter(r *http.Request) (models.Filter, error) {
	q := r.URL.Query()
	f := models.Filter{From: q.Get("from"), To: q.Get("to")}
	if s := q.Get("category"); s != "" {
		id, e := strconv.ParseInt(s, 10, 64)
		if e != nil || id < 1 {
			return f, errors.New("Выберите категорию из списка")
		}
		f.CategoryID = id
	}
	if f.From != "" && !models.ValidDate(f.From) || f.To != "" && !models.ValidDate(f.To) {
		return f, errors.New("Введите даты в формате ГГГГ-ММ-ДД (с 1900 года)")
	}
	if f.From != "" && f.To != "" && f.From > f.To {
		return f, errors.New("Начало периода не может быть позже окончания")
	}
	return f, nil
}
func filterValues(f models.Filter) url.Values {
	v := url.Values{}
	if f.CategoryID > 0 {
		v.Set("category", strconv.FormatInt(f.CategoryID, 10))
	}
	if f.From != "" {
		v.Set("from", f.From)
	}
	if f.To != "" {
		v.Set("to", f.To)
	}
	return v
}
func validCategory(cats []models.Category, id int64) bool {
	for _, c := range cats {
		if c.ID == id {
			return true
		}
	}
	return false
}
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	cats, err := h.store.Expenses.Categories(r.Context())
	if err != nil {
		h.internal(w, r, err)
		return
	}
	f, filterErr := parseFilter(r)
	p := Page{Title: "Мои расходы", Active: "expenses", Categories: cats, CategoryID: f.CategoryID, From: f.From, To: f.To, Page: 1, Pages: 1, Filtered: f.CategoryID != 0 || f.From != "" || f.To != ""}
	if f.CategoryID > 0 && !validCategory(cats, f.CategoryID) {
		filterErr = errors.New("Такой категории нет")
	}
	if filterErr != nil {
		p.Error = filterErr.Error()
		h.render(w, r, "expenses", 400, p)
		return
	}
	if s := r.URL.Query().Get("page"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			p.Error = "Некорректный номер страницы"
			h.render(w, r, "expenses", 400, p)
			return
		}
		p.Page = n
	}
	uid := currentUser(r).ID
	totals, err := h.store.Expenses.Totals(r.Context(), uid, f)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	p.Total = totals.Amount
	p.Count = totals.Count
	if p.Count > 0 {
		p.Average = p.Total / int64(p.Count)
	}
	p.Pages = max(1, (p.Count+24)/25)
	p.Page = min(p.Page, p.Pages)
	now := time.Now()
	month, err := h.store.Expenses.Totals(r.Context(), uid, models.Filter{From: now.Format("2006-01") + "-01", To: time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")})
	if err != nil {
		h.internal(w, r, err)
		return
	}
	p.MonthTotal = month.Amount
	p.Expenses, err = h.store.Expenses.GetAll(r.Context(), uid, f, 25, (p.Page-1)*25)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	v := filterValues(f)
	p.FilterQuery = v.Encode()
	if p.Page > 1 {
		v.Set("page", strconv.Itoa(p.Page-1))
		p.PrevURL = "/expenses?" + v.Encode()
	}
	if p.Page < p.Pages {
		v.Set("page", strconv.Itoa(p.Page+1))
		p.NextURL = "/expenses?" + v.Encode()
	}
	p.Notice = map[string]string{"created": "Трата добавлена", "updated": "Изменения сохранены", "deleted": "Трата удалена"}[r.URL.Query().Get("notice")]
	h.render(w, r, "expenses", 200, p)
}
func (h *Handler) expensePage(w http.ResponseWriter, r *http.Request, p Page, status int) {
	cats, err := h.store.Expenses.Categories(r.Context())
	if err != nil {
		h.internal(w, r, err)
		return
	}
	p.Categories = cats
	p.Active = "expenses"
	if p.Edit {
		p.Title = "Редактировать трату"
	} else {
		p.Title = "Новая трата"
	}
	h.render(w, r, "expense_form", status, p)
}
func (h *Handler) newExpense(w http.ResponseWriter, r *http.Request) {
	h.expensePage(w, r, Page{Action: "/expenses", Form: Form{Date: time.Now().Format("2006-01-02")}}, 200)
}
func (h *Handler) findExpense(w http.ResponseWriter, r *http.Request) (models.Expense, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		h.fail(w, r, 404)
		return models.Expense{}, false
	}
	e, err := h.store.Expenses.GetByID(r.Context(), currentUser(r).ID, id)
	if err != nil {
		h.notFoundOrError(w, r, err)
		return e, false
	}
	return e, true
}
func (h *Handler) editExpense(w http.ResponseWriter, r *http.Request) {
	e, ok := h.findExpense(w, r)
	if !ok {
		return
	}
	h.expensePage(w, r, Page{Edit: true, Action: r.URL.Path, Form: Form{Amount: models.Decimal(e.Amount), Description: e.Description, Date: e.Date, CategoryID: strconv.FormatInt(e.CategoryID, 10)}}, 200)
}
func (h *Handler) validateExpense(r *http.Request) (models.Expense, Form, map[string]string, error) {
	f := Form{Amount: strings.TrimSpace(r.PostForm.Get("amount")), Description: strings.TrimSpace(r.PostForm.Get("description")), Date: r.PostForm.Get("date"), CategoryID: r.PostForm.Get("category_id")}
	errs := map[string]string{}
	amount, err := models.ParseAmount(f.Amount)
	if err != nil {
		errs["amount"] = err.Error()
	}
	if !utf8.ValidString(f.Description) || utf8.RuneCountInString(f.Description) < 1 || utf8.RuneCountInString(f.Description) > 300 {
		errs["description"] = "Добавьте описание длиной от 1 до 300 символов"
	}
	if !models.ValidDate(f.Date) {
		errs["date"] = "Укажите существующую дату в формате ГГГГ-ММ-ДД (с 1900 года)"
	}
	id, _ := strconv.ParseInt(f.CategoryID, 10, 64)
	cats, err := h.store.Expenses.Categories(r.Context())
	if err != nil {
		return models.Expense{}, f, errs, err
	}
	if !validCategory(cats, id) {
		errs["category_id"] = "Выберите категорию из списка"
	}
	return models.Expense{UserID: currentUser(r).ID, Amount: amount, Description: f.Description, Date: f.Date, CategoryID: id}, f, errs, nil
}
func (h *Handler) createExpense(w http.ResponseWriter, r *http.Request) {
	e, f, errs, err := h.validateExpense(r)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	if len(errs) > 0 {
		h.expensePage(w, r, Page{Action: "/expenses", Form: f, Errors: errs}, 422)
		return
	}
	if _, err = h.store.Expenses.Create(r.Context(), e); err != nil {
		h.internal(w, r, err)
		return
	}
	http.Redirect(w, r, "/expenses?notice=created", 303)
}
func (h *Handler) updateExpense(w http.ResponseWriter, r *http.Request) {
	old, ok := h.findExpense(w, r)
	if !ok {
		return
	}
	e, f, errs, err := h.validateExpense(r)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	if len(errs) > 0 {
		h.expensePage(w, r, Page{Edit: true, Action: r.URL.Path, Form: f, Errors: errs}, 422)
		return
	}
	e.ID = old.ID
	if err = h.store.Expenses.Update(r.Context(), e); err != nil {
		h.notFoundOrError(w, r, err)
		return
	}
	http.Redirect(w, r, "/expenses?notice=updated", 303)
}
func (h *Handler) deletePage(w http.ResponseWriter, r *http.Request) {
	e, ok := h.findExpense(w, r)
	if !ok {
		return
	}
	h.render(w, r, "delete", 200, Page{Title: "Удалить трату", Active: "expenses", Expense: e, Action: r.URL.Path})
}
func (h *Handler) deleteExpense(w http.ResponseWriter, r *http.Request) {
	e, ok := h.findExpense(w, r)
	if !ok {
		return
	}
	if err := h.store.Expenses.Delete(r.Context(), currentUser(r).ID, e.ID); err != nil {
		h.notFoundOrError(w, r, err)
		return
	}
	http.Redirect(w, r, "/expenses?notice=deleted", 303)
}
