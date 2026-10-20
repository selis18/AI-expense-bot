// Package expense defines validation and monetary rules for family expenses.
package expense

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var moneyPattern = regexp.MustCompile(`^[0-9]+([.,][0-9]{1,2})?$`)
var categories = map[string]bool{"еда": true, "транспорт": true, "жильё": true, "развлечения": true, "здоровье": true, "покупки": true, "другое": true}

// Input describes a proposed expense. Amount is a decimal string in rubles.
type Input struct {
	Amount   string `json:"amount"`
	Category string `json:"category"`
	SpentAt  string `json:"spent_at"`
	Comment  string `json:"comment"`
}

// ParseAmount converts decimal rubles to kopecks without floating-point arithmetic.
func ParseAmount(value string) (int64, error) {
	if !moneyPattern.MatchString(value) {
		return 0, errors.New("сумма должна быть положительным числом с максимум двумя знаками после запятой")
	}
	parts := strings.Split(strings.ReplaceAll(value, ",", "."), ".")
	frac := "00"
	if len(parts) == 2 {
		frac = (parts[1] + "0")[:2]
	}
	n, err := strconv.ParseInt(parts[0]+frac, 10, 64)
	if err != nil || n <= 0 {
		return 0, errors.New("сумма вне допустимого диапазона")
	}
	return n, nil
}

// Validate checks the expense fields and returns the amount in kopecks.
func (i Input) Validate() (int64, error) {
	amount, err := ParseAmount(i.Amount)
	if err != nil {
		return 0, err
	}
	if !categories[i.Category] {
		return 0, errors.New("неизвестная категория")
	}
	date, err := time.Parse("2006-01-02", i.SpentAt)
	if err != nil || date.Year() < 1 {
		return 0, errors.New("дата должна иметь формат YYYY-MM-DD")
	}
	if utf8.RuneCountInString(i.Comment) > 1000 {
		return 0, errors.New("комментарий слишком длинный")
	}
	return amount, nil
}

// Confirmation formats a saved expense; amount must be expressed in kopecks.
func Confirmation(i Input, amount int64) string {
	return fmt.Sprintf("Добавил: %s — %d,%02d ₽, %s. %s", i.Category, amount/100, amount%100, i.SpentAt, i.Comment)
}
