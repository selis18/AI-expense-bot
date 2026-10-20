package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

func TestExtractBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, args, finish string
		status             int
		wantError          bool
	}{
		{"valid", `{"amount":"350","category":"еда","spent_at":"2026-10-16","comment":"кофе"}`, "tool_calls", 200, false},
		{"owner injection", `{"amount":"350","category":"еда","spent_at":"2026-10-16","comment":"кофе","user_id":2}`, "tool_calls", 200, true},
		{"negative amount", `{"amount":"-350","category":"еда","spent_at":"2026-10-16","comment":"кофе"}`, "tool_calls", 200, true},
		{"truncated", "{}", "length", 200, true},
		{"unavailable", "{}", "", 503, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("missing authorization")
				}
				var request map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if strings.Contains(string(request["tools"]), "user_id") {
					t.Error("tools expose identity")
				}
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": tc.finish, "message": map[string]any{"tool_calls": []any{map[string]any{"type": "function", "function": map[string]any{"name": "create_expense", "arguments": tc.args}}}}}}})
			}))
			defer server.Close()
			c := Client{HTTP: resty.NewWithClient(server.Client()), Endpoint: server.URL, Key: "test-key", Model: "test"}
			got, err := c.Extract(context.Background(), "кофе 350", time.Now())
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.wantError && (got == nil || got.Amount != "350") {
				t.Fatalf("unexpected expense: %+v", got)
			}
		})
	}
}
