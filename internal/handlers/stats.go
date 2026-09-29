package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"github.com/El1syum/tksu_pl/internal/models"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) statsPage(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	h.render(w, r, "stats", 200, Page{Title: "Статистика", Active: "stats", Year: now.Year(), From: now.Format("2006-01") + "-01", To: time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")})
}
func (h *Handler) statsCategory(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilter(r)
	if err != nil {
		h.json(w, 400, map[string]string{"error": err.Error()})
		return
	}
	data, err := h.store.Expenses.ByCategory(r.Context(), currentUser(r).ID, f)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	h.json(w, 200, data)
}
func (h *Handler) statsMonth(w http.ResponseWriter, r *http.Request) {
	year := time.Now().Year()
	if s := r.URL.Query().Get("year"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 1900 || v > 9999 {
			h.json(w, 400, map[string]string{"error": "Год должен быть от 1900 до 9999"})
			return
		}
		year = v
	}
	data, err := h.store.Expenses.ByMonth(r.Context(), currentUser(r).ID, year)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	h.json(w, 200, data)
}

// Prevent spreadsheet formulas in user-controlled CSV cells, including whitespace-prefixed values.
func csvText(s string) string {
	trimmed := strings.TrimLeft(s, " \t\r\n")
	if strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") || strings.HasPrefix(s, "\n") || strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "@") {
		return "'" + s
	}
	return s
}
func (h *Handler) exportCSV(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilter(r)
	if err != nil {
		h.fail(w, r, 400)
		return
	}
	expenses, err := h.store.Expenses.GetAll(r.Context(), currentUser(r).ID, f, 0, 0)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	var b bytes.Buffer
	b.WriteString("\xef\xbb\xbf")
	writer := csv.NewWriter(&b)
	writer.Comma = ';'
	writer.UseCRLF = true
	writer.Write([]string{"Дата", "Категория", "Описание", "Сумма (RUB)"})
	for _, e := range expenses {
		writer.Write([]string{e.Date, csvText(e.Category), csvText(e.Description), strings.ReplaceAll(models.Decimal(e.Amount), ".", ",")})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		h.internal(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="expenses-%s.csv"`, time.Now().Format("2006-01-02")))
	w.Write(b.Bytes())
}
