# HTTP API

The API serves the family panel at `/api`. It accesses all family expenses without authentication, as required by the assignment. Compose exposes it on localhost only.

Requests with a body must use `Content-Type: application/json`. The maximum body size is 16 KiB. Unknown fields, `null` values, and invalid field types are rejected.

Amounts are decimal strings in rubles in requests and responses, for example `"350.50"`. Dates use `YYYY-MM-DD`; timestamps use RFC 3339. IDs are positive integers.

## Routes

| Method | Path | Result |
| --- | --- | --- |
| GET | `/api/users` | `200`, list of known Telegram users |
| GET | `/api/expenses` | `200`, filtered page |
| GET | `/api/expenses/{id}` | `200`, one expense |
| POST | `/api/expenses` | `201`, saved expense and `Location` header |
| PATCH | `/api/expenses/{id}` | `200`, updated expense |
| DELETE | `/api/expenses/{id}` | `204`, no response body |

## Users

Users appear after contacting the bot. The panel selects an existing user when creating an expense.

```json
{"items": [{"id": 42, "display_name": "Alex"}]}
```

## List expenses

```text
GET /api/expenses?user_id=42&from=2026-10-01&through=2026-10-20&limit=50&offset=0
```

| Parameter | Meaning |
| --- | --- |
| `user_id` | Owner |
| `from` | First date, inclusive |
| `through` | Last date, inclusive |
| `category` | Exact category |
| `comment` | Case-insensitive literal substring |
| `limit` | Page size, 1–100, default 50 |
| `offset` | Records to skip, default 0 |

Unknown or repeated parameters are rejected. Results are ordered by `spent_at DESC, id DESC`. Offset pagination can shift when records change between requests.

```json
{
  "items": [
    {
      "id": 1,
      "user_id": 42,
      "amount": "350.50",
      "category": "еда",
      "spent_at": "2026-10-20",
      "comment": "кофе",
      "created_at": "2026-10-20T09:00:00Z",
      "updated_at": "2026-10-20T09:00:00Z"
    }
  ],
  "limit": 50,
  "offset": 0
}
```

An empty list has `"items": []`. The response does not include a total count.

## Create an expense

```text
POST /api/expenses
```

```json
{
  "user_id": 42,
  "amount": "350.50",
  "category": "еда",
  "spent_at": "2026-10-20",
  "comment": "кофе"
}
```

All fields except `comment` are required. Amounts must be positive, have at most two decimal places, and fit in signed 64-bit kopecks. A decimal comma is accepted on input; responses use a decimal point. Comments may contain up to 1,000 Unicode code points.

Categories: `еда`, `транспорт`, `жильё`, `развлечения`, `здоровье`, `покупки`, `другое`.

The response is the saved expense object. Repeated POST requests create separate expenses; Telegram update deduplication does not apply to this API.

## Update an expense

```text
PATCH /api/expenses/1
```

```json
{"amount": "450.00", "comment": ""}
```

Only supplied fields change. Allowed fields are `amount`, `category`, `spent_at`, and `comment`. An empty string clears the comment. Empty patches and ownership changes are rejected.

## Errors

```json
{"error": {"code": "expense_not_found", "message": "Expense not found"}}
```

| Status | Codes |
| --- | --- |
| 400 | `invalid_id`, `invalid_query`, `invalid_json`, `invalid_expense`, `unknown_user` |
| 404 | `expense_not_found`, `not_found` |
| 405 | `method_not_allowed` |
| 413 | `body_too_large` |
| 415 | `unsupported_media_type` |
| 500 | `internal_error` |

Clients should use `code`, not parse `message`. Internal database errors are not returned to clients.
