package expense

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

type Selection struct {
	ID       int64  `json:"id,omitempty"`
	Latest   bool   `json:"latest,omitempty"`
	From     string `json:"from,omitempty"`
	Through  string `json:"through,omitempty"`
	Category string `json:"category,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

func (s Selection) Filter() Filter {
	return Filter{From: s.From, Through: s.Through, Category: s.Category, Comment: s.Comment}
}

func (s Selection) Validate(target bool) error {
	if err := s.Filter().Validate(); err != nil {
		return err
	}
	if s.ID < 0 {
		return errors.New("invalid expense ID")
	}
	hasFilter := s.From != "" || s.Through != "" || s.Category != "" || s.Comment != ""
	if s.ID > 0 && (s.Latest || hasFilter) {
		return errors.New("ID cannot be combined with filters")
	}
	if s.Latest && hasFilter {
		return errors.New("latest cannot be combined with filters")
	}
	if target && s.ID == 0 && !s.Latest && !hasFilter {
		return errors.New("specify an expense to change")
	}
	return nil
}

type Action struct {
	Name      string
	Input     Input
	Selection Selection
	Patch     Patch
	Message   string
}

type Turn struct {
	Prompt   string
	Response string
}

func DecodeAction(name, arguments string) (Action, error) {
	action := Action{Name: name}
	switch name {
	case "create_expense":
		if err := decodeArguments(arguments, &action.Input); err != nil {
			return Action{}, err
		}
	case "list_expenses", "summarize_expenses", "delete_expense":
		if err := decodeArguments(arguments, &action.Selection); err != nil {
			return Action{}, err
		}
	case "update_expense":
		var input struct {
			Target   Selection `json:"target"`
			Amount   *string   `json:"amount,omitempty"`
			Category *string   `json:"category,omitempty"`
			SpentAt  *string   `json:"spent_at,omitempty"`
			Comment  *string   `json:"comment,omitempty"`
		}
		if err := decodeArguments(arguments, &input); err != nil {
			return Action{}, err
		}
		action.Selection = input.Target
		action.Patch = Patch{Amount: input.Amount, Category: input.Category, SpentAt: input.SpentAt, Comment: input.Comment}
	default:
		return Action{}, errors.New("unknown tool")
	}
	return action, action.Validate()
}

func (a Action) Validate() error {
	switch a.Name {
	case "create_expense":
		_, err := a.Input.Validate()
		return err
	case "list_expenses", "summarize_expenses":
		if a.Selection.ID != 0 || a.Selection.Latest {
			return errors.New("use filters for lists and totals")
		}
		return a.Selection.Validate(false)
	case "update_expense":
		if err := a.Patch.Validate(); err != nil {
			return err
		}
		return a.Selection.Validate(true)
	case "delete_expense":
		return a.Selection.Validate(true)
	default:
		return errors.New("unknown tool")
	}
}

func decodeArguments(value string, target any) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value), &object); err != nil || object == nil {
		return errors.New("expected argument object")
	}
	for _, raw := range object {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return errors.New("null arguments are not allowed")
		}
	}
	decoder := json.NewDecoder(bytes.NewBufferString(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid tool arguments")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("trailing arguments")
	}
	return nil
}
