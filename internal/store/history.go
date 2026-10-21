package store

import (
	"context"

	"example.com/ai-expense-bot/internal/expense"
)

func (s *Store) History(ctx context.Context, userID int64) ([]expense.Turn, error) {
	rows, err := s.Pool.Query(ctx, `SELECT prompt,response FROM (
 SELECT update_id,prompt,response FROM conversation_turns
 WHERE user_id=$1 AND created_at>now()-interval '30 minutes'
 ORDER BY update_id DESC LIMIT 6) recent ORDER BY update_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	turns := make([]expense.Turn, 0)
	for rows.Next() {
		var turn expense.Turn
		if err := rows.Scan(&turn.Prompt, &turn.Response); err != nil {
			return nil, err
		}
		turns = append(turns, turn)
	}
	return turns, rows.Err()
}

func (s *Store) RecordTurn(ctx context.Context, updateID, userID int64, prompt, response string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "INSERT INTO conversation_turns(update_id,user_id,prompt,response) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING", updateID, userID, prompt, response); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM conversation_turns WHERE user_id=$1 AND update_id NOT IN
 (SELECT update_id FROM conversation_turns WHERE user_id=$1 ORDER BY update_id DESC LIMIT 6)`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
