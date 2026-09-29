package models

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type User struct {
	ID                  int64
	Email, PasswordHash string
}
type Category struct {
	ID                int64
	Name, Color, Icon string
}
type Expense struct {
	RecurringID                              int64
	ID, UserID, Amount, CategoryID           int64
	Description, Date, Category, Color, Icon string
}
type Filter struct {
	CategoryID int64
	From, To   string
}
type Totals struct {
	Amount int64
	Count  int
}
type CategoryStat struct {
	Name   string `json:"name"`
	Color  string `json:"color"`
	Amount int64  `json:"amount"`
	Count  int    `json:"count"`
}
type MonthStat struct {
	Month  int   `json:"month"`
	Amount int64 `json:"amount"`
}

// Amounts are integer kopecks throughout the application; no float arithmetic.
func ParseAmount(s string) (int64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
	parts := strings.Split(s, ".")
	if len(parts) > 2 || parts[0] == "" || len(parts[0]) > 9 {
		return 0, errors.New("Укажите сумму от 0,01 до 999 999 999,99 ₽")
	}
	for _, p := range parts {
		if p == "" {
			return 0, errors.New("Укажите сумму с точностью до копеек")
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return 0, errors.New("Сумма должна быть положительным числом")
			}
		}
	}
	rub, _ := strconv.ParseInt(parts[0], 10, 64)
	var kop int64
	if len(parts) == 2 {
		if len(parts[1]) > 2 {
			return 0, errors.New("Не больше двух знаков после запятой")
		}
		p := parts[1]
		if len(p) == 1 {
			p += "0"
		}
		kop, _ = strconv.ParseInt(p, 10, 64)
	}
	amount := rub*100 + kop
	if amount <= 0 {
		return 0, errors.New("Сумма должна быть больше нуля")
	}
	return amount, nil
}

func Decimal(amount int64) string { return fmt.Sprintf("%d.%02d", amount/100, amount%100) }
func Money(amount int64) string {
	s := strconv.FormatInt(amount/100, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + " " + s[i:]
	}
	return fmt.Sprintf("%s,%02d", s, amount%100)
}
func ValidDate(s string) bool {
	t, err := time.Parse("2006-01-02", s)
	return err == nil && t.Year() >= 1900 && t.Year() <= 9999 && t.Format("2006-01-02") == s
}
