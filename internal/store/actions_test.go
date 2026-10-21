package store

import (
	"strings"
	"testing"
	"time"

	"example.com/ai-expense-bot/internal/expense"
)

func TestDescribeSummaryLargeTotal(t *testing.T) {
	got := describeSummary(expense.Summary{Count: 2, Total: "18446744073709551614", ByCategory: map[string]string{"еда": "1"}})
	if !strings.Contains(got, "184467440737095516,14 ₽") || !strings.Contains(got, "еда: 0,01 ₽") {
		t.Fatalf("summary = %q; wrong decimal representation", got)
	}
}

func TestPendingDeleteDetectsChanges(t *testing.T) {
	original := expense.Record{ID: 1, UserID: 42, Amount: 35000, Comment: "кофе", UpdatedAt: time.Now()}
	changed := original
	changed.Amount = 45000
	if sameExpense(original, changed) {
		t.Fatal("amount change was not detected")
	}
	changed = original
	changed.UpdatedAt = original.UpdatedAt.Add(time.Second)
	if sameExpense(original, changed) {
		t.Fatal("record version change was not detected")
	}
	if !sameExpense(original, original) {
		t.Fatal("identical record rejected")
	}
}
