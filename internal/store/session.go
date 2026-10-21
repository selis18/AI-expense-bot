package store

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func loadSession(ctx context.Context, db database, userID int64) (pendingDelete, bool, error) {
	var raw []byte
	var expires time.Time
	err := db.QueryRow(ctx, "SELECT context,expires_at FROM dialog_sessions WHERE user_id=$1", userID).Scan(&raw, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return pendingDelete{}, false, nil
	}
	if err != nil {
		return pendingDelete{}, false, err
	}
	var state pendingDelete
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, false, err
	}
	return state, time.Now().Before(expires), nil
}

func saveSession(ctx context.Context, tx pgx.Tx, userID int64, state pendingDelete) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO dialog_sessions(user_id,context,expires_at) VALUES($1,$2,now()+interval '10 minutes')
 ON CONFLICT(user_id) DO UPDATE SET context=EXCLUDED.context,expires_at=EXCLUDED.expires_at,updated_at=now()`, userID, raw)
	return err
}

func selectionNumber(text string) int {
	value := strings.Trim(strings.TrimSpace(text), " .!?")
	if number, err := strconv.Atoi(value); err == nil && number > 0 {
		return number
	}
	for number, words := range [][]string{
		{"первую", "первый", "первая", "первое"}, {"вторую", "второй", "вторая", "второе"},
		{"третью", "третий", "третья", "третье"}, {"четвёртую", "четвертую", "четвертый", "четвертую"},
		{"пятую", "пятый"}, {"шестую", "шестой"}, {"седьмую", "седьмой"}, {"восьмую", "восьмой"}, {"девятую", "девятый"}, {"десятую", "десятый"},
	} {
		for _, word := range words {
			if value == word {
				return number + 1
			}
		}
	}
	return 0
}
