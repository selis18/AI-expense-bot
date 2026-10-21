package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

type testHandler struct{ users []int64 }

func (h *testHandler) Handle(_ context.Context, _ int64, user int64, _, _ string) (string, error) {
	h.users = append(h.users, user)
	return "Сохранено", nil
}

func TestRejectedReplyDoesNotBlockNextUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	handler := &testHandler{}
	sent := 0
	offset := int64(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "getUpdates") {
			var input struct {
				Offset int64 `json:"offset"`
			}
			_ = json.NewDecoder(r.Body).Decode(&input)
			offset = input.Offset
			if offset > 0 {
				cancel()
				_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":1,"message":{"text":"кофе 1","from":{"id":42},"chat":{"id":42,"type":"private"}}},{"update_id":2,"message":{"text":"кофе 2","from":{"id":43},"chat":{"id":43,"type":"private"}}}]}`))
			return
		}
		sent++
		if sent == 1 {
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":403}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()
	bot := Bot{Token: "test", APIBaseURL: server.URL, HTTP: resty.NewWithClient(server.Client()), Handler: handler}
	bot.Run(ctx)
	if len(handler.users) != 2 || sent != 2 || offset != 3 {
		t.Fatalf("users=%v sent=%d offset=%d", handler.users, sent, offset)
	}
}

func TestTelegramRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"parameters":{"retry_after":7}}`))
	}))
	defer server.Close()
	bot := Bot{Token: "test", APIBaseURL: server.URL, HTTP: resty.NewWithClient(server.Client())}
	err := bot.call(context.Background(), "sendMessage", map[string]any{}, nil)
	var rate *apiError
	if !errors.As(err, &rate) || rate.RetryAfter != 7*time.Second {
		t.Fatalf("rate limit = %v", err)
	}
}
