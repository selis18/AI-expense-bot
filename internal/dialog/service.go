// Package dialog coordinates expense tools and persisted Telegram results.
package dialog

import (
	"context"
	"time"

	"example.com/ai-expense-bot/internal/expense"
)

// Repository stores expenses and results keyed by Telegram update ID.
type Repository interface {
	Register(context.Context, int64, string) error
	Result(context.Context, int64, int64) (string, bool, error)
	Save(context.Context, int64, int64, *expense.Input, string) (string, error)
}

// Extractor proposes an expense without selecting its owner or saving it.
type Extractor interface {
	Extract(context.Context, string, time.Time) (*expense.Input, error)
}

// Service handles the single-expense dialog supported by the first block.
type Service struct {
	Store    Repository
	LLM      Extractor
	Location *time.Location
}

// Handle processes a message with the trusted identity supplied by Telegram.
func (s *Service) Handle(ctx context.Context, updateID, userID int64, name, text string) (string, error) {
	if err := s.Store.Register(ctx, userID, name); err != nil {
		return "", err
	}
	result, found, err := s.Store.Result(ctx, updateID, userID)
	if err != nil || found {
		return result, err
	}
	input, err := s.LLM.Extract(ctx, text, time.Now().In(s.Location))
	if err != nil {
		return "Не удалось обработать сообщение. Попробуйте отправить его позже.", nil
	}
	return s.Store.Save(ctx, updateID, userID, input, "Пока я умею добавлять одну трату за сообщение. Укажите описание и сумму, например: кофе 350.")
}
