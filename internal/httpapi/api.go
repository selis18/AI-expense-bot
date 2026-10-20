package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"example.com/ai-expense-bot/internal/expense"
	"github.com/go-chi/chi/v5"
)

type Expenses interface {
	Create(context.Context, int64, expense.Input) (expense.Record, error)
	Get(context.Context, int64) (expense.Record, error)
	List(context.Context, expense.Filter, expense.Page) ([]expense.Record, error)
	Update(context.Context, int64, expense.Patch) (expense.Record, error)
	Delete(context.Context, int64) error
}

type Users interface {
	ListUsers(context.Context) ([]expense.User, error)
}

type API struct {
	expenses Expenses
	users    Users
}

func New(expenses Expenses, users Users) http.Handler {
	api := &API{expenses: expenses, users: users}
	router := chi.NewRouter()
	router.Get("/users", api.listUsers)
	router.Get("/expenses", api.listExpenses)
	router.Post("/expenses", api.createExpense)
	router.Get("/expenses/{id}", api.getExpense)
	router.Patch("/expenses/{id}", api.updateExpense)
	router.Delete("/expenses/{id}", api.deleteExpense)
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Route not found")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	})
	return router
}

type expenseResponse struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Amount    string    `json:"amount"`
	Category  string    `json:"category"`
	SpentAt   string    `json:"spent_at"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func response(record expense.Record) expenseResponse {
	return expenseResponse{
		ID: record.ID, UserID: record.UserID,
		Amount:   fmt.Sprintf("%d.%02d", record.Amount/100, record.Amount%100),
		Category: record.Category, SpentAt: record.SpentAt.Format("2006-01-02"),
		Comment: record.Comment, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
	}
}

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.users.ListUsers(r.Context())
	if err != nil {
		writeStorageError(w, err)
		return
	}
	if users == nil {
		users = []expense.User{}
	}
	writeJSON(w, http.StatusOK, struct {
		Items []expense.User `json:"items"`
	}{Items: users})
}

func (a *API) listExpenses(w http.ResponseWriter, r *http.Request) {
	filter, page, err := parseQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	records, err := a.expenses.List(r.Context(), filter, page)
	if err != nil {
		writeStorageError(w, err)
		return
	}
	items := make([]expenseResponse, 0, len(records))
	for _, record := range records {
		items = append(items, response(record))
	}
	writeJSON(w, http.StatusOK, struct {
		Items  []expenseResponse `json:"items"`
		Limit  int               `json:"limit"`
		Offset int               `json:"offset"`
	}{Items: items, Limit: page.Limit, Offset: page.Offset})
}

func (a *API) createExpense(w http.ResponseWriter, r *http.Request) {
	var input struct {
		UserID int64 `json:"user_id"`
		expense.Input
	}
	if !decodeBody(w, r, &input) {
		return
	}
	if input.UserID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_expense", "user_id must be positive")
		return
	}
	if _, err := input.Input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_expense", err.Error())
		return
	}
	record, err := a.expenses.Create(r.Context(), input.UserID, input.Input)
	if err != nil {
		writeStorageError(w, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/expenses/%d", record.ID))
	writeJSON(w, http.StatusCreated, response(record))
}

func (a *API) getExpense(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	record, err := a.expenses.Get(r.Context(), id)
	if err != nil {
		writeStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response(record))
}

func (a *API) updateExpense(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	var input struct {
		Amount   *string `json:"amount"`
		Category *string `json:"category"`
		SpentAt  *string `json:"spent_at"`
		Comment  *string `json:"comment"`
	}
	if !decodeBody(w, r, &input) {
		return
	}
	patch := expense.Patch{
		Amount: input.Amount, Category: input.Category,
		SpentAt: input.SpentAt, Comment: input.Comment,
	}
	if err := patch.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_expense", err.Error())
		return
	}
	record, err := a.expenses.Update(r.Context(), id, patch)
	if err != nil {
		writeStorageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response(record))
}

func (a *API) deleteExpense(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	if err := a.expenses.Delete(r.Context(), id); err != nil {
		writeStorageError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func requestID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_id", "id must be a positive integer")
		return 0, false
	}
	return id, true
}

func decodeBody(w http.ResponseWriter, r *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Use application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "Request body exceeds 16 KiB")
		} else {
			writeError(w, http.StatusBadRequest, "invalid_json", "Cannot read request body")
		}
		return false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Expected one JSON object")
		return false
	}
	for _, value := range fields {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			writeError(w, http.StatusBadRequest, "invalid_json", "Fields must not be null")
			return false
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid fields or field types")
		return false
	}
	return true
}

func parseQuery(r *http.Request) (expense.Filter, expense.Page, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return expense.Filter{}, expense.Page{}, errors.New("invalid query encoding")
	}
	allowed := map[string]bool{
		"user_id": true, "from": true, "through": true, "category": true,
		"comment": true, "limit": true, "offset": true,
	}
	for key, entries := range values {
		if !allowed[key] || len(entries) != 1 {
			return expense.Filter{}, expense.Page{}, errors.New("unknown or repeated query parameter")
		}
	}
	filter := expense.Filter{
		From: values.Get("from"), Through: values.Get("through"),
		Category: values.Get("category"), Comment: values.Get("comment"),
	}
	if values.Has("user_id") {
		id, err := strconv.ParseInt(values.Get("user_id"), 10, 64)
		if err != nil || id <= 0 {
			return filter, expense.Page{}, errors.New("user_id must be a positive integer")
		}
		filter.UserID = id
	}
	page := expense.Page{Limit: 50}
	for key, destination := range map[string]*int{"limit": &page.Limit, "offset": &page.Offset} {
		if values.Has(key) {
			value, err := strconv.Atoi(values.Get(key))
			if err != nil || value < 0 || (key == "limit" && value == 0) {
				return filter, page, errors.New("invalid pagination")
			}
			*destination = value
		}
	}
	if err := filter.Validate(); err != nil {
		return filter, page, err
	}
	page, err = page.Normalize()
	return filter, page, err
}

func writeStorageError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, expense.ErrNotFound):
		writeError(w, http.StatusNotFound, "expense_not_found", "Expense not found")
	case errors.Is(err, expense.ErrUnknownUser):
		writeError(w, http.StatusBadRequest, "unknown_user", "Select an existing user")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Could not complete the request")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
