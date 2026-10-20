// Package store persists expenses and processing results in PostgreSQL.
package store

import (
	"context"
	_ "embed"
	"errors"

	"example.com/ai-expense-bot/internal/expense"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

// Store uses a shared connection pool owned by the application.
type Store struct{ Pool *pgxpool.Pool }

// Migrate applies the initial schema once, under a transaction-scoped lock.
func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback is a no-op after a successful commit.
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(57245001)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=1)").Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if _, err = tx.Exec(ctx, schema); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Register creates a Telegram user or refreshes their display name.
func (s *Store) Register(ctx context.Context, userID int64, name string) error {
	_, err := s.Pool.Exec(ctx, "INSERT INTO users(id,display_name) VALUES($1,$2) ON CONFLICT(id) DO UPDATE SET display_name=EXCLUDED.display_name", userID, name)
	return err
}

// Result retrieves a saved response only for its owner; the bool reports a match.
func (s *Store) Result(ctx context.Context, updateID, userID int64) (string, bool, error) {
	var result string
	err := s.Pool.QueryRow(ctx, "SELECT result FROM processed_updates WHERE update_id=$1 AND user_id=$2", updateID, userID).Scan(&result)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return result, err == nil, err
}

// Save atomically records an optional expense and the response for this update.
// The update ID is the operation key: block 1 permits at most one mutation per message.
func (s *Store) Save(ctx context.Context, updateID, userID int64, input *expense.Input, response string) (string, error) {
	var amount int64
	var err error
	if input != nil {
		amount, err = input.Validate()
		if err != nil {
			return "", err
		}
		response = expense.Confirmation(*input, amount)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var inserted int64
	err = tx.QueryRow(ctx, "INSERT INTO processed_updates(update_id,user_id,result) VALUES($1,$2,$3) ON CONFLICT DO NOTHING RETURNING update_id", updateID, userID, response).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, "SELECT result FROM processed_updates WHERE update_id=$1 AND user_id=$2", updateID, userID).Scan(&response)
		return response, err
	}
	if err != nil {
		return "", err
	}
	if input != nil {
		_, err = tx.Exec(ctx, "INSERT INTO expenses(user_id,amount,category,spent_at,comment) VALUES($1,$2,$3,$4::date,$5)", userID, amount, input.Category, input.SpentAt, input.Comment)
		if err != nil {
			return "", err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return response, nil
}
