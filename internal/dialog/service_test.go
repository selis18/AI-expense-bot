package dialog

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/ai-expense-bot/internal/expense"
)

type fakeStore struct {
	result       string
	owner        int64
	writes       int
	fail         bool
	confirmation string
}

func (s *fakeStore) Register(context.Context, int64, string) error { return nil }
func (s *fakeStore) Result(_ context.Context, _ int64, user int64) (string, bool, error) {
	return s.result, s.result != "" && s.owner == user, nil
}
func (s *fakeStore) Save(_ context.Context, _ int64, user int64, input *expense.Input, response string) (string, error) {
	if s.fail {
		return "", errors.New("database unavailable")
	}
	s.owner = user
	s.writes++
	if input != nil {
		amount, err := input.Validate()
		if err != nil {
			return "", err
		}
		response = expense.Confirmation(*input, amount)
	}
	s.result = response
	return response, nil
}

func (s *fakeStore) Execute(ctx context.Context, updateID, userID int64, action expense.Action) (string, error) {
	return s.Save(ctx, updateID, userID, &action.Input, "")
}

func (s *fakeStore) Resume(context.Context, int64, int64, string) (string, bool, error) {
	return s.confirmation, s.confirmation != "", nil
}

type fakeLLM struct{ calls int }

func (m *fakeLLM) Select(context.Context, string, time.Time) (*expense.Action, error) {
	m.calls++
	return &expense.Action{Name: "create_expense", Input: expense.Input{Amount: "350", Category: "еда", SpentAt: "2026-10-16", Comment: "кофе"}}, nil
}

func TestReplayUsesSavedResult(t *testing.T) {
	repo := &fakeStore{}
	model := &fakeLLM{}
	s := Service{Store: repo, LLM: model, Location: time.UTC}
	first, err := s.Handle(context.Background(), 1, 42, "member", "кофе 350")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Handle(context.Background(), 1, 42, "member", "кофе 350")
	if err != nil {
		t.Fatal(err)
	}
	if first != second || repo.writes != 1 || model.calls != 1 || repo.owner != 42 {
		t.Fatalf("replay changed operation: %+v, calls=%d", repo, model.calls)
	}
}
func TestStorageFailureDoesNotConfirm(t *testing.T) {
	s := Service{Store: &fakeStore{fail: true}, LLM: &fakeLLM{}, Location: time.UTC}
	reply, err := s.Handle(context.Background(), 1, 42, "member", "кофе 350")
	if err == nil || reply != "" {
		t.Fatalf("confirmed failed operation: %q %v", reply, err)
	}
}

func TestConfirmationBypassesLLM(t *testing.T) {
	repo := &fakeStore{confirmation: "Удалено"}
	model := &fakeLLM{}
	service := Service{Store: repo, LLM: model, Location: time.UTC}
	response, err := service.Handle(context.Background(), 2, 42, "member", "да")
	if err != nil || response != "Удалено" || model.calls != 0 {
		t.Fatalf("confirmation = %q, %v; LLM calls = %d", response, err, model.calls)
	}
}
