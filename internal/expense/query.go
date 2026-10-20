package expense

import (
	"errors"
	"time"
	"unicode/utf8"
)

var ErrNotFound = errors.New("expense not found")

type Record struct {
	ID        int64
	UserID    int64
	Amount    int64
	Category  string
	SpentAt   time.Time
	Comment   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Filter struct {
	UserID   int64
	From     string
	Through  string
	Category string
	Comment  string
}

func (f Filter) Validate() error {
	if f.UserID < 0 {
		return errors.New("user ID must not be negative")
	}
	for _, date := range []string{f.From, f.Through} {
		if date != "" {
			if err := validateDate(date); err != nil {
				return err
			}
		}
	}
	if f.From != "" && f.Through != "" && f.From > f.Through {
		return errors.New("start date must not follow end date")
	}
	if f.Category != "" && !categories[f.Category] {
		return errors.New("unknown category")
	}
	if utf8.RuneCountInString(f.Comment) > 1000 {
		return errors.New("comment filter is too long")
	}
	return nil
}

type Page struct {
	Limit  int
	Offset int
}

func (p Page) Normalize() (Page, error) {
	if p.Limit == 0 {
		p.Limit = 50
	}
	if p.Limit < 1 || p.Limit > 100 || p.Offset < 0 {
		return Page{}, errors.New("page limit must be 1–100 and offset must not be negative")
	}
	return p, nil
}

type Patch struct {
	Amount   *string
	Category *string
	SpentAt  *string
	Comment  *string
}

func (p Patch) Validate() error {
	if p.Amount == nil && p.Category == nil && p.SpentAt == nil && p.Comment == nil {
		return errors.New("no fields to update")
	}
	if p.Amount != nil {
		if _, err := ParseAmount(*p.Amount); err != nil {
			return err
		}
	}
	if p.Category != nil && !categories[*p.Category] {
		return errors.New("unknown category")
	}
	if p.SpentAt != nil {
		if err := validateDate(*p.SpentAt); err != nil {
			return err
		}
	}
	if p.Comment != nil && utf8.RuneCountInString(*p.Comment) > 1000 {
		return errors.New("comment is too long")
	}
	return nil
}

type Summary struct {
	Count      int64
	Total      string
	ByCategory map[string]string
}

func validateDate(value string) error {
	date, err := time.Parse("2006-01-02", value)
	if err != nil || date.Year() < 1 {
		return errors.New("date must use YYYY-MM-DD")
	}
	return nil
}
