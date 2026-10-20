package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/go-resty/resty/v2"
)

type Handler interface {
	Handle(context.Context, int64, int64, string, string) (string, error)
}

type Bot struct {
	Token   string
	HTTP    *resty.Client
	Handler Handler
}
type update struct {
	ID      int64 `json:"update_id"`
	Message *struct {
		Text string `json:"text"`
		From *struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			IsBot     bool   `json:"is_bot"`
		} `json:"from"`
		Chat struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
	} `json:"message"`
}

func (b *Bot) call(ctx context.Context, method string, payload any, out any) error {
	response, err := b.HTTP.R().SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		SetDoNotParseResponse(true).
		Post("https://api.telegram.org/bot" + b.Token + "/" + method)
	if err != nil {
		return errors.New("Telegram unavailable")
	}
	defer response.RawBody().Close()
	if response.StatusCode() != 200 {
		return fmt.Errorf("Telegram status %d", response.StatusCode())
	}
	var envelope struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	if err = json.NewDecoder(io.LimitReader(response.RawBody(), 2<<20)).Decode(&envelope); err != nil {
		return errors.New("invalid Telegram response")
	}
	if !envelope.OK {
		return errors.New("Telegram operation rejected")
	}
	if out != nil {
		return json.Unmarshal(envelope.Result, out)
	}
	return nil
}

func (b *Bot) Run(ctx context.Context) {
	var offset int64
	for ctx.Err() == nil {
		var updates []update
		err := b.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 25, "allowed_updates": []string{"message"}}, &updates)
		if err != nil {
			slog.Warn("telegram polling failed")
			if !pause(ctx) {
				return
			}
			continue
		}
		for _, u := range updates {
			m := u.Message
			if m == nil || m.From == nil || m.From.IsBot || m.Chat.Type != "private" || m.Chat.ID != m.From.ID || m.Text == "" {
				offset = u.ID + 1
				continue
			}
			actionCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			reply, err := b.Handler.Handle(actionCtx, u.ID, m.From.ID, m.From.FirstName, m.Text)
			if err == nil {
				err = b.call(actionCtx, "sendMessage", map[string]any{"chat_id": m.Chat.ID, "text": reply}, nil)
			}
			cancel()
			if err != nil {
				slog.Warn("telegram update pending retry", "update_id", u.ID)
				if !pause(ctx) {
					return
				}
				break
			}
			offset = u.ID + 1
		}
	}
}

func pause(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(3 * time.Second):
		return true
	}
}
