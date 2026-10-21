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
	Execute(context.Context, int64, int64, expense.Action) (string, error)
	Resume(context.Context, int64, int64, string) (string, bool, error)
}

type Extractor interface {
	Select(context.Context, string, time.Time) (*expense.Action, error)
}

type historyStore interface {
	History(context.Context, int64) ([]expense.Turn, error)
	RecordTurn(context.Context, int64, int64, string, string) error
}

type contextExtractor interface {
	SelectWithHistory(context.Context, string, time.Time, []expense.Turn) (*expense.Action, error)
}

type Service struct {
	Store    Repository
	LLM      Extractor
	Location *time.Location
}

func (s *Service) Handle(ctx context.Context, updateID, userID int64, name, text string) (reply string, resultErr error) {
	if history, ok := s.Store.(historyStore); ok {
		defer func() {
			if resultErr == nil && reply != "" {
				resultErr = history.RecordTurn(ctx, updateID, userID, text, reply)
			}
		}()
	}
	if err := s.Store.Register(ctx, userID, name); err != nil {
		return "", err
	}
	result, found, err := s.Store.Result(ctx, updateID, userID)
	if err != nil || found {
		return result, err
	}
	if response, handled, err := s.Store.Resume(ctx, updateID, userID, text); handled || err != nil {
		return response, err
	}
	var action *expense.Action
	if model, ok := s.LLM.(contextExtractor); ok {
		var turns []expense.Turn
		if history, ok := s.Store.(historyStore); ok {
			turns, err = history.History(ctx, userID)
			if err != nil {
				return "", err
			}
		}
		action, err = model.SelectWithHistory(ctx, text, time.Now().In(s.Location), turns)
	} else {
		action, err = s.LLM.Select(ctx, text, time.Now().In(s.Location))
	}
	if err != nil {
		return s.Store.Save(ctx, updateID, userID, nil, "Не удалось обработать сообщение. Попробуйте отправить его позже.")
	}
	if action != nil && action.Name == "clarify" {
		return s.Store.Save(ctx, updateID, userID, nil, action.Message)
	}
	if action == nil {
		return s.Store.Save(ctx, updateID, userID, nil, "Уточните запрос: что добавить, показать, изменить или удалить? Для новой траты укажите описание и сумму.")
	}
	return s.Store.Execute(ctx, updateID, userID, *action)
}
