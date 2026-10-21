package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"example.com/ai-expense-bot/internal/expense"
	"github.com/jackc/pgx/v5"
)

type pendingDelete struct {
	Expense    expense.Record   `json:"expense"`
	Choices    []expense.Record `json:"choices,omitempty"`
	Action     *expense.Action  `json:"action,omitempty"`
	SelectedID int64            `json:"selected_id,omitempty"`
}

func (s *Store) Execute(ctx context.Context, updateID, userID int64, action expense.Action) (string, error) {
	if err := action.Validate(); err != nil {
		return "", err
	}
	return s.withUpdate(ctx, updateID, userID, func(tx pgx.Tx, repo *Expenses) (string, error) {
		state, active, err := loadSession(ctx, tx, userID)
		if err != nil {
			return "", err
		}
		if active && state.SelectedID > 0 && action.Selection.Latest {
			action.Selection = expense.Selection{ID: state.SelectedID}
		}
		return executeAction(ctx, tx, repo, action)
	})
}

func executeAction(ctx context.Context, tx pgx.Tx, repo *Expenses, action expense.Action) (string, error) {
	userID := repo.userID
	if _, err := tx.Exec(ctx, "DELETE FROM dialog_sessions WHERE user_id=$1", userID); err != nil {
		return "", err
	}
	switch action.Name {
	case "create_expense":
		record, err := repo.Create(ctx, userID, action.Input)
		if err != nil {
			return "", err
		}
		if err := saveSession(ctx, tx, userID, pendingDelete{SelectedID: record.ID}); err != nil {
			return "", err
		}
		return "Добавил: " + describe(record), nil
	case "list_expenses":
		records, err := repo.List(ctx, action.Selection.Filter(), expense.Page{Limit: 11})
		if err != nil {
			return "", err
		}
		state := pendingDelete{}
		if len(records) == 1 {
			state.SelectedID = records[0].ID
		}
		if len(records) > 10 {
			state.Choices = records[:10]
		} else {
			state.Choices = records
		}
		if err := saveSession(ctx, tx, userID, state); err != nil {
			return "", err
		}
		return describeList(records), nil
	case "summarize_expenses":
		summary, err := repo.Summarize(ctx, action.Selection.Filter())
		if err != nil {
			return "", err
		}
		return describeSummary(summary), nil
	case "update_expense", "delete_expense":
		records, err := repo.selectTarget(ctx, action.Selection)
		if err != nil {
			return "", err
		}
		if len(records) == 0 {
			return "Трата не найдена.", nil
		}
		if len(records) > 1 {
			choices := records
			if len(choices) > 10 {
				choices = choices[:10]
			}
			if err := saveSession(ctx, tx, userID, pendingDelete{Choices: choices, Action: &action}); err != nil {
				return "", err
			}
			return "Найдено несколько трат. Напишите номер из списка или уточните запрос.\n" + describeList(records), nil
		}
		record, err := scanExpense(tx.QueryRow(ctx, "SELECT "+expenseColumns+" FROM expenses WHERE id=$1 AND user_id=$2 FOR UPDATE", records[0].ID, userID))
		if errors.Is(err, expense.ErrNotFound) {
			return "Трата не найдена.", nil
		}
		if err != nil {
			return "", err
		}
		if action.Name == "update_expense" {
			updated, err := repo.Update(ctx, record.ID, action.Patch)
			if err != nil {
				return "", err
			}
			if err := saveSession(ctx, tx, userID, pendingDelete{SelectedID: updated.ID}); err != nil {
				return "", err
			}
			return "Изменил: " + describe(updated), nil
		}
		return savePendingDelete(ctx, tx, userID, record)
	}
	return "", errors.New("unknown action")
}

func (s *Store) Resume(ctx context.Context, updateID, userID int64, text string) (string, bool, error) {
	answer := strings.ToLower(strings.TrimSpace(text))
	if number := selectionNumber(answer); number > 0 {
		state, _, err := loadSession(ctx, s.Pool, userID)
		if err != nil {
			return "", true, err
		}
		if len(state.Choices) == 0 {
			return "", false, nil
		}
		response, err := s.withUpdate(ctx, updateID, userID, func(tx pgx.Tx, repo *Expenses) (string, error) {
			state, active, err := loadSession(ctx, tx, userID)
			if err != nil {
				return "", err
			}
			if !active {
				return "Список устарел. Повторите запрос.", nil
			}
			if number > len(state.Choices) {
				return "Такого номера нет. Выберите номер из списка.", nil
			}
			selected := state.Choices[number-1]
			if state.Action == nil {
				current, err := repo.Get(ctx, selected.ID)
				if errors.Is(err, expense.ErrNotFound) {
					return "Трата не найдена.", nil
				}
				if err != nil {
					return "", err
				}
				if err := saveSession(ctx, tx, userID, pendingDelete{SelectedID: current.ID}); err != nil {
					return "", err
				}
				return "Выбрана: " + describe(current), nil
			}
			action := *state.Action
			action.Selection = expense.Selection{ID: selected.ID}
			return executeAction(ctx, tx, repo, action)
		})
		return response, true, err
	}
	if answer != "да" && answer != "нет" && answer != "отмена" {
		return "", false, nil
	}
	response, err := s.withUpdate(ctx, updateID, userID, func(tx pgx.Tx, repo *Expenses) (string, error) {
		var raw []byte
		var expires time.Time
		err := tx.QueryRow(ctx, "SELECT context,expires_at FROM dialog_sessions WHERE user_id=$1 FOR UPDATE", userID).Scan(&raw, &expires)
		if errors.Is(err, pgx.ErrNoRows) {
			return "Нет удаления, ожидающего подтверждения. Напишите новый запрос.", nil
		}
		if err != nil {
			return "", err
		}
		if _, err := tx.Exec(ctx, "DELETE FROM dialog_sessions WHERE user_id=$1", userID); err != nil {
			return "", err
		}
		if answer != "да" {
			return "Действие отменено.", nil
		}
		if time.Now().After(expires) {
			return "Подтверждение истекло. Повторите запрос на удаление.", nil
		}
		var pending pendingDelete
		if err := json.Unmarshal(raw, &pending); err != nil {
			return "", err
		}
		if pending.Expense.ID == 0 {
			return "Нет удаления, ожидающего подтверждения. Напишите новый запрос.", nil
		}
		record, err := scanExpense(tx.QueryRow(ctx, "SELECT "+expenseColumns+" FROM expenses WHERE id=$1 AND user_id=$2 FOR UPDATE", pending.Expense.ID, userID))
		if errors.Is(err, expense.ErrNotFound) {
			return "Трата не найдена.", nil
		}
		if err != nil {
			return "", err
		}
		if !sameExpense(record, pending.Expense) {
			return savePendingDelete(ctx, tx, userID, record)
		}
		if err := repo.Delete(ctx, record.ID); err != nil {
			return "", err
		}
		return "Удалил: " + describe(record), nil
	})
	return response, true, err
}

func (s *Store) withUpdate(ctx context.Context, updateID, userID int64, run func(pgx.Tx, *Expenses) (string, error)) (string, error) {
	if userID <= 0 {
		return "", errors.New("invalid user ID")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var lockedID int64
	if err := tx.QueryRow(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", userID).Scan(&lockedID); err != nil {
		return "", err
	}
	var response string
	err = tx.QueryRow(ctx, "SELECT result FROM processed_updates WHERE update_id=$1 AND user_id=$2", updateID, userID).Scan(&response)
	if err == nil {
		return response, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	repo := &Expenses{store: s, userID: userID, tx: tx}
	response, err = run(tx, repo)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO processed_updates(update_id,user_id,result) VALUES($1,$2,$3)", updateID, userID, response); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return response, nil
}

func (r *Expenses) selectTarget(ctx context.Context, target expense.Selection) ([]expense.Record, error) {
	if target.ID > 0 || target.Latest {
		query := "SELECT " + expenseColumns + " FROM expenses WHERE user_id=$1"
		args := []any{r.userID}
		if target.ID > 0 {
			query += " AND id=$2"
			args = append(args, target.ID)
		} else {
			query += " ORDER BY created_at DESC,id DESC LIMIT 1"
		}
		record, err := scanExpense(r.database().QueryRow(ctx, query, args...))
		if errors.Is(err, expense.ErrNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return []expense.Record{record}, nil
	}
	return r.List(ctx, target.Filter(), expense.Page{Limit: 11})
}

func savePendingDelete(ctx context.Context, tx pgx.Tx, userID int64, record expense.Record) (string, error) {
	if err := saveSession(ctx, tx, userID, pendingDelete{Expense: record, SelectedID: record.ID}); err != nil {
		return "", err
	}
	return "Удалить: " + describe(record) + "? Ответьте «да» или «нет» в течение 10 минут.", nil
}

func sameExpense(a, b expense.Record) bool {
	return a.ID == b.ID && a.UserID == b.UserID && a.Amount == b.Amount && a.Category == b.Category && a.SpentAt.Equal(b.SpentAt) && a.Comment == b.Comment && a.UpdatedAt.Equal(b.UpdatedAt)
}

func describe(record expense.Record) string {
	comment := []rune(record.Comment)
	if len(comment) > 100 {
		comment = append(comment[:100], '…')
	}
	return fmt.Sprintf("#%d %s — %d,%02d ₽, %s. %s", record.ID, record.Category, record.Amount/100, record.Amount%100, record.SpentAt.Format("2006-01-02"), string(comment))
}

func describeList(records []expense.Record) string {
	if len(records) == 0 {
		return "Трат не найдено."
	}
	var lines []string
	for i, record := range records {
		if i == 10 {
			lines = append(lines, "Показаны первые 10 трат. Уточните период или категорию.")
			break
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, describe(record)))
	}
	return strings.Join(lines, "\n")
}

func describeSummary(summary expense.Summary) string {
	format := func(value string) string {
		for len(value) < 3 {
			value = "0" + value
		}
		return value[:len(value)-2] + "," + value[len(value)-2:] + " ₽"
	}
	lines := []string{fmt.Sprintf("Всего: %s. Трат: %d.", format(summary.Total), summary.Count)}
	for _, category := range []string{"еда", "транспорт", "жильё", "развлечения", "здоровье", "покупки", "другое"} {
		if value, ok := summary.ByCategory[category]; ok {
			lines = append(lines, category+": "+format(value))
		}
	}
	return strings.Join(lines, "\n")
}
