package store

import (
	"context"
	"errors"
	"testing"

	"example.com/ai-expense-bot/internal/expense"
)

func TestUnconfiguredScopeDeniesAccess(t *testing.T) {
	var repository Expenses
	ctx := context.Background()
	if _, err := repository.Get(ctx, 1); err == nil {
		t.Error("Get() accepted an unconfigured scope")
	}
	if _, err := repository.List(ctx, expense.Filter{}, expense.Page{}); err == nil {
		t.Error("List() accepted an unconfigured scope")
	}
	if _, err := repository.Summarize(ctx, expense.Filter{}); err == nil {
		t.Error("Summarize() accepted an unconfigured scope")
	}
	if _, err := repository.Create(ctx, 1, expense.Input{}); err == nil {
		t.Error("Create() accepted an unconfigured scope")
	}
	if _, err := repository.Update(ctx, 1, expense.Patch{}); err == nil {
		t.Error("Update() accepted an unconfigured scope")
	}
	if err := repository.Delete(ctx, 1); err == nil {
		t.Error("Delete() accepted an unconfigured scope")
	}
}

func TestUserScopeRejectsInvalidOwner(t *testing.T) {
	store := &Store{}
	for _, id := range []int64{0, -1} {
		if _, err := store.UserExpenses(id); err == nil {
			t.Errorf("UserExpenses(%d) succeeded; want error", id)
		}
	}
	repository, err := store.UserExpenses(42)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Create(context.Background(), 43, expense.Input{}); !errors.Is(err, expense.ErrNotFound) {
		t.Errorf("Create(other owner) = %v; want ErrNotFound", err)
	}
}
