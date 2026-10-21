package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

func TestSelectToolsOverHTTP(t *testing.T) {
	tests := []struct{ name, args string }{
		{name: "list_expenses", args: `{"from":"2026-10-01","through":"2026-10-21"}`},
		{name: "summarize_expenses", args: `{"category":"еда"}`},
		{name: "update_expense", args: `{"target":{"latest":true},"amount":"450"}`},
		{name: "delete_expense", args: `{"comment":"такси"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload struct {
					Tools []json.RawMessage `json:"tools"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if len(payload.Tools) != 5 {
					t.Errorf("tools count = %d; want 5", len(payload.Tools))
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "tool_calls", "message": map[string]any{"tool_calls": []any{map[string]any{"type": "function", "function": map[string]any{"name": test.name, "arguments": test.args}}}}}}})
			}))
			defer server.Close()
			client := Client{HTTP: resty.NewWithClient(server.Client()), Endpoint: server.URL, Key: "test", Model: "test"}
			action, err := client.Select(context.Background(), "test", time.Now())
			if err != nil || action == nil || action.Name != test.name {
				t.Fatalf("Select() = %+v, %v; want %s", action, err, test.name)
			}
		})
	}
}

func TestSelectTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	client := Client{HTTP: resty.NewWithClient(server.Client()).SetTimeout(30 * time.Millisecond), Endpoint: server.URL, Key: "test", Model: "test"}
	if action, err := client.Select(context.Background(), "кофе 350", time.Now()); err == nil || action != nil {
		t.Fatalf("Select() = %+v, %v; want timeout", action, err)
	}
}

func TestSelectRejectsMultipleCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"type":"function","function":{"name":"list_expenses","arguments":"{}"}},{"type":"function","function":{"name":"list_expenses","arguments":"{}"}}]}}]}`))
	}))
	defer server.Close()
	client := Client{HTTP: resty.NewWithClient(server.Client()), Endpoint: server.URL, Key: "test", Model: "test"}
	if action, err := client.Select(context.Background(), "test", time.Now()); err == nil || action != nil {
		t.Fatalf("Select() = %+v, %v; want rejection", action, err)
	}
}
