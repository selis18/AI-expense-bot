package dialog

import (
	"context"
	"time"

	"example.com/ai-expense-bot/internal/expense"
)

type Repository interface {
	Register(context.Context, int64, string) error
	Result(context.Context, int64, int64) (string, bool, error)
	Save(context.Context, int64, int64, *expense.Input, string) (string, error)
}

type Extractor interface {
	Extract(context.Context, string, time.Time) (*expense.Input, error)
}

type Service struct {
	Store    Repository
	LLM      Extractor
	Location *time.Location
}

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
