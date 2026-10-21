package llm

func tools() []any {
	text := func(description string) map[string]any {
		return map[string]any{"type": "string", "description": description}
	}
	category := map[string]any{"type": "string", "enum": []string{"еда", "транспорт", "жильё", "развлечения", "здоровье", "покупки", "другое"}}
	object := func(properties map[string]any, required []string) map[string]any {
		return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
	}
	tool := func(name, description string, parameters map[string]any) any {
		return map[string]any{"type": "function", "function": map[string]any{"name": name, "description": description, "parameters": parameters}}
	}
	filters := map[string]any{"from": text("First date YYYY-MM-DD, inclusive"), "through": text("Last date YYYY-MM-DD, inclusive"), "category": category, "comment": text("Literal substring in expense comment")}
	target := map[string]any{"id": map[string]any{"type": "integer", "minimum": 1}, "latest": map[string]any{"type": "boolean", "description": "Most recently created expense"}}
	for key, value := range filters {
		target[key] = value
	}
	return []any{
		tool("create_expense", "Add exactly one new expense", object(map[string]any{"amount": text("Positive RUB amount as decimal string"), "category": category, "spent_at": text("Date YYYY-MM-DD"), "comment": text("Expense description")}, []string{"amount", "category", "spent_at", "comment"})),
		tool("list_expenses", "Show expenses matching filters", object(filters, []string{})),
		tool("summarize_expenses", "Compute exact total and category totals", object(filters, []string{})),
		tool("update_expense", "Change one expense; server resolves target and asks if ambiguous", object(map[string]any{"target": object(target, []string{}), "amount": text("New RUB amount as decimal string"), "category": category, "spent_at": text("New date YYYY-MM-DD"), "comment": text("New comment, empty string clears it")}, []string{"target"})),
		tool("delete_expense", "Request deletion of one expense; server requires separate confirmation", object(target, []string{})),
	}
}
