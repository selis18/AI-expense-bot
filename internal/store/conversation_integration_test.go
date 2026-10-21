package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/ai-expense-bot/internal/dialog"
	"example.com/ai-expense-bot/internal/expense"
	"example.com/ai-expense-bot/internal/httpapi"
	"example.com/ai-expense-bot/internal/llm"
	"github.com/go-resty/resty/v2"
)

func TestPostgresHistoryRetentionAndExpiredSelection(t *testing.T) {
	s, ctx := newTestStore(t)
	if err := s.Register(ctx, 42, "member"); err != nil {
		t.Fatal(err)
	}
	for id := int64(1); id <= 8; id++ {
		text := fmt.Sprintf("message %d", id)
		if _, err := s.Save(ctx, id, 42, nil, text); err != nil {
			t.Fatal(err)
		}
		if err := s.RecordTurn(ctx, id, 42, text, text); err != nil {
			t.Fatal(err)
		}
	}
	history, err := s.History(ctx, 42)
	if err != nil || len(history) != 6 || history[0].Prompt != "message 3" || history[5].Prompt != "message 8" {
		t.Fatalf("retained history = %+v, %v", history, err)
	}
	if _, err := s.Pool.Exec(ctx, "UPDATE conversation_turns SET created_at=now()-interval '31 minutes' WHERE user_id=42"); err != nil {
		t.Fatal(err)
	}
	history, err = s.History(ctx, 42)
	if err != nil || len(history) != 0 {
		t.Fatalf("expired history = %+v, %v", history, err)
	}
	for _, id := range []int64{9, 10} {
		_, err := s.Execute(ctx, id, 42, expense.Action{Name: "create_expense", Input: expense.Input{Amount: "100", Category: "еда", SpentAt: "2026-10-21"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Execute(ctx, 11, 42, expense.Action{Name: "delete_expense", Selection: expense.Selection{Category: "еда"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, "UPDATE dialog_sessions SET expires_at=now()-interval '1 minute' WHERE user_id=42"); err != nil {
		t.Fatal(err)
	}
	reply, handled, err := s.Resume(ctx, 12, 42, "1")
	if err != nil || !handled || !strings.Contains(reply, "устарел") {
		t.Fatalf("expired selection = %q, %v, %v", reply, handled, err)
	}
	repo, err := s.UserExpenses(42)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := repo.Summarize(ctx, expense.Filter{})
	if err != nil || summary.Count != 2 {
		t.Fatalf("expenses after expiry = %+v, %v", summary, err)
	}
}

func TestPostgresConversationSelection(t *testing.T) {
	s, ctx := newTestStore(t)
	if err := s.Register(ctx, 42, "member"); err != nil {
		t.Fatal(err)
	}
	action := expense.Action{Name: "create_expense", Input: expense.Input{Amount: "100", Category: "транспорт", SpentAt: "2026-10-21", Comment: "такси"}}
	for _, id := range []int64{1, 2} {
		if _, err := s.Execute(ctx, id, 42, action); err != nil {
			t.Fatal(err)
		}
	}
	amount := "200"
	reply, err := s.Execute(ctx, 3, 42, expense.Action{Name: "update_expense", Selection: expense.Selection{Comment: "такси"}, Patch: expense.Patch{Amount: &amount}})
	if err != nil || !strings.Contains(reply, "несколько") {
		t.Fatalf("selection request = %q, %v", reply, err)
	}
	reply, handled, err := s.Resume(ctx, 4, 42, "вторую")
	if err != nil || !handled || !strings.Contains(reply, "200,00") {
		t.Fatalf("selection = %q, %v", reply, err)
	}
	records, err := s.FamilyExpenses().List(ctx, expense.Filter{}, expense.Page{})
	if err != nil {
		t.Fatal(err)
	}
	if records[1].Amount != 20000 || records[0].Amount != 10000 {
		t.Fatalf("wrong expense updated: %+v", records)
	}
	amount = "300"
	if _, err := s.Execute(ctx, 5, 42, expense.Action{Name: "update_expense", Selection: expense.Selection{Latest: true}, Patch: expense.Patch{Amount: &amount}}); err != nil {
		t.Fatal(err)
	}
	selected, err := s.FamilyExpenses().Get(ctx, records[1].ID)
	if err != nil || selected.Amount != 30000 {
		t.Fatalf("latest ignored selection: %+v, %v", selected, err)
	}
	if err := s.RecordTurn(ctx, 5, 42, "последнее было 300", "Изменил"); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordTurn(ctx, 5, 42, "повтор", "повтор"); err != nil {
		t.Fatal(err)
	}
	history, err := s.History(ctx, 42)
	if err != nil || len(history) != 1 || history[0].Prompt != "последнее было 300" {
		t.Fatalf("history = %+v, %v", history, err)
	}
	other, err := s.History(ctx, 43)
	if err != nil || len(other) != 0 {
		t.Fatalf("foreign history = %+v, %v", other, err)
	}
}

func TestPostgresConcurrentReplay(t *testing.T) {
	s, ctx := newTestStore(t)
	if err := s.Register(ctx, 42, "member"); err != nil {
		t.Fatal(err)
	}
	action := expense.Action{Name: "create_expense", Input: expense.Input{Amount: "100", Category: "еда", SpentAt: "2026-10-21"}}
	var group sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		group.Add(1)
		go func() { defer group.Done(); _, err := s.Execute(ctx, 99, 42, action); results <- err }()
	}
	group.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	total, err := s.FamilyExpenses().Summarize(ctx, expense.Filter{})
	if err != nil || total.Count != 1 {
		t.Fatalf("concurrent replay total = %+v, %v", total, err)
	}
}

func TestPostgresHTTPAndDialog(t *testing.T) {
	s, ctx := newTestStore(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []map[string]any `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"Сколько стоил кофе?"}}]}`))
			return
		}
		if len(payload.Messages) != 4 {
			t.Errorf("message count = %d; want system, previous user/assistant, current user", len(payload.Messages))
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":[{"id":"one","type":"function","function":{"name":"create_expense","arguments":"{\"amount\":\"350\",\"category\":\"еда\",\"spent_at\":\"2026-10-21\",\"comment\":\"кофе\"}"}}]}}]}`))
	}))
	defer server.Close()
	model := &llm.Client{HTTP: resty.NewWithClient(server.Client()), Endpoint: server.URL, Key: "test", Model: "test"}
	service := dialog.Service{Store: s, LLM: model, Location: time.UTC}
	if reply, err := service.Handle(ctx, 10, 42, "Alex", "кофе"); err != nil || !strings.Contains(reply, "Сколько") {
		t.Fatalf("clarification = %q, %v", reply, err)
	}
	if reply, err := service.Handle(ctx, 11, 42, "Alex", "350"); err != nil || !strings.HasPrefix(reply, "Добавил") {
		t.Fatalf("creation = %q, %v", reply, err)
	}
	if _, err := service.Handle(ctx, 11, 42, "Alex", "350"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("LLM calls = %d; want 2", calls)
	}
	router := httpapi.New(s.FamilyExpenses(), s)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/expenses", nil).WithContext(context.Background()))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"amount":"350.00"`) {
		t.Fatalf("REST response = %d %s", response.Code, response.Body)
	}
}
