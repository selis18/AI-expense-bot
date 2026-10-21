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

func TestCorrectInvalidArguments(t *testing.T) {
	count := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		count++
		arguments := `{"user_id":99}`
		if count == 2 {
			if !strings.Contains(string(request["messages"]), "invalid_arguments") || !strings.Contains(string(request["messages"]), "reasoning_content") {
				t.Error("missing structured feedback or original reasoning field")
			}
			arguments = `{"category":"еда"}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "tool_calls", "message": map[string]any{"role": "assistant", "reasoning_content": "test", "tool_calls": []any{map[string]any{"id": "call1", "type": "function", "function": map[string]any{"name": "summarize_expenses", "arguments": arguments}}}}}}})
	}))
	defer server.Close()
	client := Client{HTTP: resty.NewWithClient(server.Client()), Endpoint: server.URL, Key: "test", Model: "test"}
	result, err := client.Select(context.Background(), "сколько на еду", time.Now())
	if err != nil || result == nil || result.Name != "summarize_expenses" || count != 2 {
		t.Fatalf("result = %+v, %v, calls=%d", result, err, count)
	}
}
