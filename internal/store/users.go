package store

import (
	"context"

	"example.com/ai-expense-bot/internal/expense"
)

func (s *Store) ListUsers(ctx context.Context) ([]expense.User, error) {
	rows, err := s.Pool.Query(ctx, "SELECT id, display_name FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]expense.User, 0)
	for rows.Next() {
		var user expense.User
		if err := rows.Scan(&user.ID, &user.DisplayName); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}
