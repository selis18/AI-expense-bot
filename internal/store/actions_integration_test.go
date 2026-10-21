package store

import (
	"strings"
	"testing"

	"example.com/ai-expense-bot/internal/expense"
)

func TestPostgresToolActions(t *testing.T) {
	s, ctx := newTestStore(t)
	for _, id := range []int64{42, 43} {
		if err := s.Register(ctx, id, "member"); err != nil {
			t.Fatal(err)
		}
	}
	create := expense.Action{Name: "create_expense", Input: expense.Input{Amount: "350", Category: "еда", SpentAt: "2026-10-21", Comment: "кофе"}}
	first, err := s.Execute(ctx, 100, 42, create)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Execute(ctx, 100, 42, create)
	if err != nil || second != first {
		t.Fatalf("replay = %q, %v", second, err)
	}
	own, _ := s.UserExpenses(42)
	rows, err := own.List(ctx, expense.Filter{}, expense.Page{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("created rows = %+v, %v", rows, err)
	}
	id := rows[0].ID
	amount := "450"
	update := expense.Action{Name: "update_expense", Selection: expense.Selection{ID: id}, Patch: expense.Patch{Amount: &amount}}
	if _, err := s.Execute(ctx, 101, 42, update); err != nil {
		t.Fatal(err)
	}
	record, err := own.Get(ctx, id)
	if err != nil || record.Amount != 45000 {
		t.Fatalf("updated record = %+v, %v", record, err)
	}
	if response, err := s.Execute(ctx, 102, 43, update); err != nil || response != "Трата не найдена." {
		t.Fatalf("other owner update = %q, %v", response, err)
	}
	total, err := s.Execute(ctx, 103, 42, expense.Action{Name: "summarize_expenses"})
	if err != nil || !strings.Contains(total, "450,00") {
		t.Fatalf("total = %q, %v", total, err)
	}
	deletion := expense.Action{Name: "delete_expense", Selection: expense.Selection{ID: id}}
	prompt, err := s.Execute(ctx, 104, 42, deletion)
	if err != nil || !strings.Contains(prompt, "Ответьте") {
		t.Fatalf("delete request = %q, %v", prompt, err)
	}
	if _, err := own.Get(ctx, id); err != nil {
		t.Fatal("deleted before confirmation", err)
	}
	if reply, _, err := s.Resume(ctx, 105, 43, "да"); err != nil || !strings.Contains(reply, "Нет удаления") {
		t.Fatalf("foreign confirmation = %q, %v", reply, err)
	}
	amount = "500"
	if _, err := s.FamilyExpenses().Update(ctx, id, expense.Patch{Amount: &amount}); err != nil {
		t.Fatal(err)
	}
	reply, handled, err := s.Resume(ctx, 106, 42, "да")
	if err != nil || !handled || !strings.Contains(reply, "500,00") {
		t.Fatalf("changed expense confirmation = %q, %v", reply, err)
	}
	if _, err := own.Get(ctx, id); err != nil {
		t.Fatal("deleted changed expense without confirmation", err)
	}
	reply, _, err = s.Resume(ctx, 107, 42, "да")
	if err != nil || !strings.HasPrefix(reply, "Удалил:") {
		t.Fatalf("confirmed deletion = %q, %v", reply, err)
	}
	replay, _, err := s.Resume(ctx, 107, 42, "да")
	if err != nil || replay != reply {
		t.Fatalf("delete replay = %q, %v", replay, err)
	}
}

func TestPostgresAmbiguousAndExpiredDelete(t *testing.T) {
	s, ctx := newTestStore(t)
	if err := s.Register(ctx, 42, "member"); err != nil {
		t.Fatal(err)
	}
	create := expense.Action{Name: "create_expense", Input: expense.Input{Amount: "100", Category: "транспорт", SpentAt: "2026-10-21", Comment: "такси"}}
	for _, id := range []int64{1, 2} {
		if _, err := s.Execute(ctx, id, 42, create); err != nil {
			t.Fatal(err)
		}
	}
	reply, err := s.Execute(ctx, 3, 42, expense.Action{Name: "delete_expense", Selection: expense.Selection{Comment: "такси"}})
	if err != nil || !strings.Contains(reply, "несколько") {
		t.Fatalf("ambiguous delete = %q, %v", reply, err)
	}
	if _, err := s.Execute(ctx, 4, 42, expense.Action{Name: "delete_expense", Selection: expense.Selection{Latest: true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, "UPDATE dialog_sessions SET expires_at=now()-interval '1 minute' WHERE user_id=42"); err != nil {
		t.Fatal(err)
	}
	reply, _, err = s.Resume(ctx, 5, 42, "да")
	if err != nil || !strings.Contains(reply, "истекло") {
		t.Fatalf("expired confirmation = %q, %v", reply, err)
	}
	repo, _ := s.UserExpenses(42)
	records, err := repo.List(ctx, expense.Filter{}, expense.Page{})
	if err != nil || len(records) != 2 {
		t.Fatalf("records = %+v, %v", records, err)
	}
}
