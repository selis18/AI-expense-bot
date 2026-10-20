package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"example.com/ai-expense-bot/internal/expense"
	"github.com/jackc/pgx/v5"
)

type Expenses struct {
	store  *Store
	userID int64
	family bool
}

func (s *Store) UserExpenses(userID int64) (*Expenses, error) {
	if userID <= 0 {
		return nil, errors.New("user ID must be positive")
	}
	return &Expenses{store: s, userID: userID}, nil
}

func (s *Store) FamilyExpenses() *Expenses {
	return &Expenses{store: s, family: true}
}

const expenseColumns = `id, user_id, amount, category, spent_at, comment, created_at, updated_at`

func scanExpense(row pgx.Row) (expense.Record, error) {
	var record expense.Record
	err := row.Scan(
		&record.ID, &record.UserID, &record.Amount, &record.Category,
		&record.SpentAt, &record.Comment, &record.CreatedAt, &record.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return expense.Record{}, expense.ErrNotFound
	}
	return record, err
}

func (r *Expenses) predicate(filter expense.Filter) (string, []any, error) {
	if r.store == nil || (!r.family && r.userID <= 0) {
		return "", nil, errors.New("expense access scope is not configured")
	}
	if err := filter.Validate(); err != nil {
		return "", nil, err
	}
	clauses := []string{"TRUE"}
	var args []any
	add := func(expression string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(expression, len(args)))
	}
	if !r.family {
		add("user_id = $%d", r.userID)
	}
	if filter.UserID != 0 {
		add("user_id = $%d", filter.UserID)
	}
	if filter.From != "" {
		add("spent_at >= $%d::date", filter.From)
	}
	if filter.Through != "" {
		add("spent_at <= $%d::date", filter.Through)
	}
	if filter.Category != "" {
		add("category = $%d", filter.Category)
	}
	if filter.Comment != "" {
		add("strpos(lower(comment), lower($%d::text)) > 0", filter.Comment)
	}
	return strings.Join(clauses, " AND "), args, nil
}

func (r *Expenses) Create(ctx context.Context, ownerID int64, input expense.Input) (expense.Record, error) {
	if _, _, err := r.predicate(expense.Filter{}); err != nil {
		return expense.Record{}, err
	}
	if ownerID <= 0 || (!r.family && ownerID != r.userID) {
		return expense.Record{}, expense.ErrNotFound
	}
	amount, err := input.Validate()
	if err != nil {
		return expense.Record{}, err
	}
	row := r.store.Pool.QueryRow(ctx, `
		INSERT INTO expenses(user_id, amount, category, spent_at, comment)
		VALUES ($1, $2, $3, $4::date, $5)
		RETURNING `+expenseColumns,
		ownerID, amount, input.Category, input.SpentAt, input.Comment)
	return scanExpense(row)
}

func (r *Expenses) Get(ctx context.Context, id int64) (expense.Record, error) {
	where, args, err := r.predicate(expense.Filter{})
	if err != nil {
		return expense.Record{}, err
	}
	args = append(args, id)
	query := "SELECT " + expenseColumns + " FROM expenses WHERE " + where +
		fmt.Sprintf(" AND id = $%d", len(args))
	return scanExpense(r.store.Pool.QueryRow(ctx, query, args...))
}

func (r *Expenses) List(ctx context.Context, filter expense.Filter, page expense.Page) ([]expense.Record, error) {
	where, args, err := r.predicate(filter)
	if err != nil {
		return nil, err
	}
	page, err = page.Normalize()
	if err != nil {
		return nil, err
	}
	args = append(args, page.Limit, page.Offset)
	query := "SELECT " + expenseColumns + " FROM expenses WHERE " + where +
		fmt.Sprintf(" ORDER BY spent_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := r.store.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]expense.Record, 0)
	for rows.Next() {
		record, err := scanExpense(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (r *Expenses) Update(ctx context.Context, id int64, patch expense.Patch) (expense.Record, error) {
	where, args, err := r.predicate(expense.Filter{})
	if err != nil {
		return expense.Record{}, err
	}
	if err := patch.Validate(); err != nil {
		return expense.Record{}, err
	}
	var assignments []string
	add := func(column, cast string, value any) {
		args = append(args, value)
		assignments = append(assignments, fmt.Sprintf("%s = $%d%s", column, len(args), cast))
	}
	if patch.Amount != nil {
		amount, err := expense.ParseAmount(*patch.Amount)
		if err != nil {
			return expense.Record{}, err
		}
		add("amount", "", amount)
	}
	if patch.Category != nil {
		add("category", "", *patch.Category)
	}
	if patch.SpentAt != nil {
		add("spent_at", "::date", *patch.SpentAt)
	}
	if patch.Comment != nil {
		add("comment", "", *patch.Comment)
	}
	assignments = append(assignments, "updated_at = clock_timestamp()")
	args = append(args, id)
	query := "UPDATE expenses SET " + strings.Join(assignments, ", ") +
		" WHERE " + where + fmt.Sprintf(" AND id = $%d RETURNING ", len(args)) + expenseColumns
	return scanExpense(r.store.Pool.QueryRow(ctx, query, args...))
}

func (r *Expenses) Delete(ctx context.Context, id int64) error {
	where, args, err := r.predicate(expense.Filter{})
	if err != nil {
		return err
	}
	args = append(args, id)
	query := "DELETE FROM expenses WHERE " + where + fmt.Sprintf(" AND id = $%d", len(args))
	result, err := r.store.Pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return expense.ErrNotFound
	}
	return nil
}

func (r *Expenses) Summarize(ctx context.Context, filter expense.Filter) (expense.Summary, error) {
	where, args, err := r.predicate(filter)
	if err != nil {
		return expense.Summary{}, err
	}
	query := `
		SELECT COALESCE(category, ''), count(*), COALESCE(sum(amount), 0)::text
		FROM expenses WHERE ` + where + `
		GROUP BY GROUPING SETS ((category), ())`
	rows, err := r.store.Pool.Query(ctx, query, args...)
	if err != nil {
		return expense.Summary{}, err
	}
	defer rows.Close()
	summary := expense.Summary{Total: "0", ByCategory: make(map[string]string)}
	for rows.Next() {
		var category, total string
		var count int64
		if err := rows.Scan(&category, &count, &total); err != nil {
			return expense.Summary{}, err
		}
		if category == "" {
			summary.Count, summary.Total = count, total
		} else {
			summary.ByCategory[category] = total
		}
	}
	return summary, rows.Err()
}
