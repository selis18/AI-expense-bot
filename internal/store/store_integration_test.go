package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"example.com/ai-expense-bot/internal/expense"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Uses a separate schema; never drops application tables.
func TestPostgresAtomicReplay(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("expense_test_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_, _ = admin.Exec(cleanup, "DROP SCHEMA "+identifier+" CASCADE")
	}()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := Store{Pool: pool}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.Register(ctx, 42, "test"); err != nil {
		t.Fatal(err)
	}
	input := expense.Input{Amount: "350", Category: "еда", SpentAt: "2026-10-16", Comment: "кофе"}
	first, err := s.Save(ctx, 10, 42, &input, "")
	if err != nil {
		t.Fatal(err)
	}
	input.Amount = "450"
	second, err := s.Save(ctx, 10, 42, &input, "")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("replayed operation changed response")
	}
	var count, amount int64
	if err = pool.QueryRow(ctx, "SELECT count(*),sum(amount) FROM expenses WHERE user_id=42").Scan(&count, &amount); err != nil {
		t.Fatal(err)
	}
	if count != 1 || amount != 35000 {
		t.Fatalf("count=%d amount=%d", count, amount)
	}
	if _, found, err := s.Result(ctx, 10, 43); err != nil || found {
		t.Fatalf("other owner can see result: %v", err)
	}
	// Force the expense INSERT to fail after the update result INSERT.
	if _, err = pool.Exec(ctx, "ALTER TABLE expenses ADD CONSTRAINT test_amount_limit CHECK(amount < 40000)"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Save(ctx, 11, 42, &input, ""); err == nil {
		t.Fatal("expected database rejection")
	}
	if _, found, err := s.Result(ctx, 11, 42); err != nil || found {
		t.Fatalf("partial result survived rollback: %v", err)
	}
}
