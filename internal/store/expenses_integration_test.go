package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"example.com/ai-expense-bot/internal/expense"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("expense_test_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+identifier+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store := &Store{Pool: pool}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return store, ctx
}

func TestPostgresExpenseAccess(t *testing.T) {
	store, ctx := newTestStore(t)
	for _, id := range []int64{42, 43} {
		if err := store.Register(ctx, id, "member"); err != nil {
			t.Fatal(err)
		}
	}
	users, err := store.ListUsers(ctx)
	if err != nil || len(users) != 2 || users[0].ID != 42 || users[1].ID != 43 {
		t.Fatalf("ListUsers() = %+v, %v; want users 42 and 43", users, err)
	}
	alice, err := store.UserExpenses(42)
	if err != nil {
		t.Fatal(err)
	}
	bob, err := store.UserExpenses(43)
	if err != nil {
		t.Fatal(err)
	}
	family := store.FamilyExpenses()
	input := expense.Input{Amount: "350.50", Category: "еда", SpentAt: "2026-10-20", Comment: "кофе"}
	if _, err := family.Create(ctx, 999, input); !errors.Is(err, expense.ErrUnknownUser) {
		t.Fatalf("Create(unknown user) = %v; want ErrUnknownUser", err)
	}
	own, err := alice.Create(ctx, 42, input)
	if err != nil {
		t.Fatal(err)
	}
	other, err := bob.Create(ctx, 43, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := alice.Get(ctx, other.ID); !errors.Is(err, expense.ErrNotFound) {
		t.Errorf("Get(other) = %v; want ErrNotFound", err)
	}
	if _, err := alice.Get(ctx, 999999); !errors.Is(err, expense.ErrNotFound) {
		t.Errorf("Get(missing) = %v; want ErrNotFound", err)
	}
	amount := "450.25"
	if _, err := alice.Update(ctx, other.ID, expense.Patch{Amount: &amount}); !errors.Is(err, expense.ErrNotFound) {
		t.Errorf("Update(other) = %v; want ErrNotFound", err)
	}
	if err := alice.Delete(ctx, other.ID); !errors.Is(err, expense.ErrNotFound) {
		t.Errorf("Delete(other) = %v; want ErrNotFound", err)
	}
	rows, err := alice.List(ctx, expense.Filter{UserID: 43}, expense.Page{})
	if err != nil || len(rows) != 0 {
		t.Fatalf("List(other filter) = %+v, %v; want empty", rows, err)
	}
	summary, err := alice.Summarize(ctx, expense.Filter{UserID: 43})
	if err != nil || summary.Count != 0 || summary.Total != "0" {
		t.Fatalf("Summarize(other filter) = %+v, %v; want zero", summary, err)
	}
	preserved, err := bob.Get(ctx, other.ID)
	if err != nil || preserved.Amount != 35050 {
		t.Fatalf("Get(after denied writes) = %+v, %v; want 35050", preserved, err)
	}
	empty := ""
	updated, err := alice.Update(ctx, own.ID, expense.Patch{Amount: &amount, Comment: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Amount != 45025 || updated.Comment != "" || updated.Category != own.Category ||
		!updated.SpentAt.Equal(own.SpentAt) || !updated.CreatedAt.Equal(own.CreatedAt) {
		t.Fatalf("partial update changed unexpected fields: %+v", updated)
	}
	panelRecord, err := family.Create(ctx, 43, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := family.Update(ctx, panelRecord.ID, expense.Patch{Amount: &amount}); err != nil {
		t.Fatal(err)
	}
	if err := family.Delete(ctx, panelRecord.ID); err != nil {
		t.Fatal(err)
	}
	if err := alice.Delete(ctx, own.ID); err != nil {
		t.Fatal(err)
	}
	if err := alice.Delete(ctx, own.ID); !errors.Is(err, expense.ErrNotFound) {
		t.Errorf("Delete(missing) = %v; want ErrNotFound", err)
	}
	rows, err = family.List(ctx, expense.Filter{}, expense.Page{})
	if err != nil || len(rows) != 1 || rows[0].ID != other.ID {
		t.Fatalf("family records = %+v, %v; want Bob's expense", rows, err)
	}
}

func TestPostgresExpenseFiltersAndTotals(t *testing.T) {
	store, ctx := newTestStore(t)
	if err := store.Register(ctx, 42, "member"); err != nil {
		t.Fatal(err)
	}
	repository, err := store.UserExpenses(42)
	if err != nil {
		t.Fatal(err)
	}
	inputs := []expense.Input{
		{Amount: "1.01", Category: "еда", SpentAt: "2026-10-19", Comment: "кофе"},
		{Amount: "2.02", Category: "еда", SpentAt: "2026-10-20", Comment: "скидка 10%"},
		{Amount: "3.03", Category: "транспорт", SpentAt: "2026-10-20", Comment: "такси"},
	}
	var ids []int64
	for _, input := range inputs {
		record, err := repository.Create(ctx, 42, input)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, record.ID)
	}
	for offset := range ids {
		rows, err := repository.List(ctx, expense.Filter{}, expense.Page{Limit: 1, Offset: offset})
		if err != nil || len(rows) != 1 || rows[0].ID != ids[len(ids)-1-offset] {
			t.Fatalf("page %d = %+v, %v; want stable descending order", offset, rows, err)
		}
	}
	filter := expense.Filter{From: "2026-10-20", Through: "2026-10-20", Category: "еда"}
	rows, err := repository.List(ctx, filter, expense.Page{})
	if err != nil || len(rows) != 1 || rows[0].ID != ids[1] {
		t.Fatalf("filtered records = %+v, %v; want second record", rows, err)
	}
	rows, err = repository.List(ctx, expense.Filter{Comment: "%"}, expense.Page{})
	if err != nil || len(rows) != 1 || rows[0].ID != ids[1] {
		t.Fatalf("literal comment filter = %+v, %v; want second record", rows, err)
	}
	summary, err := repository.Summarize(ctx, expense.Filter{})
	if err != nil || summary.Count != 3 || summary.Total != "606" ||
		summary.ByCategory["еда"] != "303" || summary.ByCategory["транспорт"] != "303" {
		t.Fatalf("summary = %+v, %v; want 606 kopecks across three expenses", summary, err)
	}
	summary, err = repository.Summarize(ctx, filter)
	if err != nil || summary.Total != "202" || summary.Count != 1 {
		t.Fatalf("filtered summary = %+v, %v; want 202 kopecks", summary, err)
	}
	for range 2 {
		_, err := repository.Create(ctx, 42, expense.Input{
			Amount: "92233720368547758.07", Category: "другое", SpentAt: "2026-10-20",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	summary, err = repository.Summarize(ctx, expense.Filter{Category: "другое"})
	if err != nil || summary.Total != "18446744073709551614" {
		t.Fatalf("large summary = %+v, %v; want exact total above int64", summary, err)
	}
}
