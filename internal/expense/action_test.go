package expense

import "testing"

func TestDecodeAction(t *testing.T) {
	tests := []struct {
		name, arguments string
		wantError       bool
	}{
		{name: "list_expenses", arguments: `{"from":"2026-10-01"}`},
		{name: "summarize_expenses", arguments: `{"category":"еда"}`},
		{name: "update_expense", arguments: `{"target":{"latest":true},"amount":"450"}`},
		{name: "delete_expense", arguments: `{"comment":"такси"}`},
		{name: "delete_expense", arguments: `{}`, wantError: true},
		{name: "delete_expense", arguments: `{"id":1,"latest":true}`, wantError: true},
		{name: "update_expense", arguments: `{"target":{"id":1}}`, wantError: true},
		{name: "list_expenses", arguments: `{"user_id":43}`, wantError: true},
		{name: "update_expense", arguments: `{"target":{"user_id":43},"amount":"1"}`, wantError: true},
		{name: "update_expense", arguments: `{"target":{"id":1},"amount":null}`, wantError: true},
		{name: "delete_expense", arguments: `{"id":1,"confirmed":true}`, wantError: true},
		{name: "list_expenses", arguments: `{"from":"2026-02-29"}`, wantError: true},
		{name: "delete_all", arguments: `{}`, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name+test.arguments, func(t *testing.T) {
			_, err := DecodeAction(test.name, test.arguments)
			if (err != nil) != test.wantError {
				t.Fatalf("DecodeAction() error = %v; want error %v", err, test.wantError)
			}
		})
	}
}
