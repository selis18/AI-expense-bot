package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"example.com/ai-expense-bot/internal/expense"
	"github.com/go-chi/chi/v5"
)

type fakeStore struct {
	record expense.Record
	filter expense.Filter
	page   expense.Page
	patch  expense.Patch
	calls  int
	err    error
}

func (s *fakeStore) Create(_ context.Context, userID int64, input expense.Input) (expense.Record, error) {
	s.calls++
	if s.err != nil {
		return expense.Record{}, s.err
	}
	amount, err := input.Validate()
	if err != nil {
		return expense.Record{}, err
	}
	date, err := time.Parse("2006-01-02", input.SpentAt)
	if err != nil {
		return expense.Record{}, err
	}
	s.record = expense.Record{
		ID: 1, UserID: userID, Amount: amount, Category: input.Category,
		SpentAt: date, Comment: input.Comment,
	}
	return s.record, nil
}

func (s *fakeStore) Get(_ context.Context, id int64) (expense.Record, error) {
	s.calls++
	if s.err != nil {
		return expense.Record{}, s.err
	}
	if s.record.ID != id {
		return expense.Record{}, expense.ErrNotFound
	}
	return s.record, nil
}

func (s *fakeStore) List(_ context.Context, filter expense.Filter, page expense.Page) ([]expense.Record, error) {
	s.calls++
	s.filter, s.page = filter, page
	if s.err != nil {
		return nil, s.err
	}
	if s.record.ID == 0 {
		return nil, nil
	}
	return []expense.Record{s.record}, nil
}

func (s *fakeStore) Update(ctx context.Context, id int64, patch expense.Patch) (expense.Record, error) {
	record, err := s.Get(ctx, id)
	if err != nil {
		return expense.Record{}, err
	}
	s.patch = patch
	if patch.Comment != nil {
		record.Comment = *patch.Comment
	}
	s.record = record
	return record, nil
}

func (s *fakeStore) Delete(ctx context.Context, id int64) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	s.record = expense.Record{}
	return nil
}

func (s *fakeStore) ListUsers(context.Context) ([]expense.User, error) {
	s.calls++
	return []expense.User{{ID: 42, DisplayName: "Member"}}, s.err
}

func testRouter(s *fakeStore) http.Handler {
	router := chi.NewRouter()
	router.Mount("/api", New(s, s))
	return router
}

func request(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestExpenseRoutes(t *testing.T) {
	store := &fakeStore{}
	router := testRouter(store)
	created := request(router, http.MethodPost, "/api/expenses",
		`{"user_id":42,"amount":"350.50","category":"еда","spent_at":"2026-10-20","comment":"кофе"}`)
	if created.Code != http.StatusCreated || created.Header().Get("Location") != "/api/expenses/1" {
		t.Fatalf("POST: status %d, location %q, body %s", created.Code, created.Header().Get("Location"), created.Body)
	}
	var record expenseResponse
	if err := json.Unmarshal(created.Body.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Amount != "350.50" || record.SpentAt != "2026-10-20" || record.UserID != 42 {
		t.Fatalf("POST record = %+v; wrong money, date or owner", record)
	}
	read := request(router, http.MethodGet, "/api/expenses/1", "")
	if read.Code != http.StatusOK {
		t.Fatalf("GET: status %d, body %s", read.Code, read.Body)
	}
	changed := request(router, http.MethodPatch, "/api/expenses/1", `{"comment":""}`)
	if changed.Code != http.StatusOK || store.record.Comment != "" || store.record.Amount != 35050 ||
		store.patch.Amount != nil {
		t.Fatalf("PATCH: status %d, record %+v, patch %+v", changed.Code, store.record, store.patch)
	}
	deleted := request(router, http.MethodDelete, "/api/expenses/1", "")
	if deleted.Code != http.StatusNoContent || deleted.Body.Len() != 0 {
		t.Fatalf("DELETE: status %d, body %s", deleted.Code, deleted.Body)
	}
	missing := request(router, http.MethodGet, "/api/expenses/1", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("GET deleted: status %d; want 404", missing.Code)
	}
}

func TestListQuery(t *testing.T) {
	store := &fakeStore{}
	path := "/api/expenses?" + url.Values{
		"user_id": {"42"}, "from": {"2026-10-01"}, "through": {"2026-10-20"},
		"category": {"еда"}, "comment": {"кофе"}, "limit": {"10"}, "offset": {"20"},
	}.Encode()
	w := request(testRouter(store), http.MethodGet, path, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET list: status %d, body %s", w.Code, w.Body)
	}
	want := expense.Filter{UserID: 42, From: "2026-10-01", Through: "2026-10-20", Category: "еда", Comment: "кофе"}
	if store.filter != want || store.page != (expense.Page{Limit: 10, Offset: 20}) {
		t.Fatalf("query = %+v, %+v; want %+v, limit 10 offset 20", store.filter, store.page, want)
	}
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("empty list must be an array: %s", w.Body)
	}
	users := request(testRouter(store), http.MethodGet, "/api/users", "")
	if users.Code != http.StatusOK || !strings.Contains(users.Body.String(), `"display_name":"Member"`) {
		t.Fatalf("GET users: status %d, body %s", users.Code, users.Body)
	}
}

func TestInvalidRequestsNeverReachStorage(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "invalid ID", method: http.MethodDelete, path: "/api/expenses/0"},
		{name: "overflow ID", method: http.MethodGet, path: "/api/expenses/9223372036854775808"},
		{name: "invalid limit", method: http.MethodGet, path: "/api/expenses?limit=101"},
		{name: "zero limit", method: http.MethodGet, path: "/api/expenses?limit=0"},
		{name: "negative offset", method: http.MethodGet, path: "/api/expenses?offset=-1"},
		{name: "duplicate filter", method: http.MethodGet, path: "/api/expenses?user_id=42&user_id=43"},
		{name: "invalid date", method: http.MethodGet, path: "/api/expenses?from=2026-02-29"},
		{name: "unknown query", method: http.MethodGet, path: "/api/expenses?all=true"},
		{name: "malformed query", method: http.MethodGet, path: "/api/expenses?category=%zz"},
		{name: "empty patch", method: http.MethodPatch, path: "/api/expenses/1", body: `{}`},
		{name: "null patch", method: http.MethodPatch, path: "/api/expenses/1", body: `{"amount":null}`},
		{name: "owner change", method: http.MethodPatch, path: "/api/expenses/1", body: `{"user_id":43}`},
		{name: "numeric money", method: http.MethodPatch, path: "/api/expenses/1", body: `{"amount":350.5}`},
		{name: "negative money", method: http.MethodPatch, path: "/api/expenses/1", body: `{"amount":"-1"}`},
		{name: "two objects", method: http.MethodPatch, path: "/api/expenses/1", body: `{"amount":"1"} {}`},
		{name: "array", method: http.MethodPost, path: "/api/expenses", body: `[]`},
		{name: "missing owner", method: http.MethodPost, path: "/api/expenses", body: `{"amount":"1","category":"еда","spent_at":"2026-10-20"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeStore{}
			w := request(testRouter(store), test.method, test.path, test.body)
			if w.Code != http.StatusBadRequest || store.calls != 0 {
				t.Fatalf("status %d, calls %d; want 400 and no storage calls, body %s", w.Code, store.calls, w.Body)
			}
		})
	}
}

func TestRequestBodyLimits(t *testing.T) {
	store := &fakeStore{}
	router := testRouter(store)
	r := httptest.NewRequest(http.MethodPost, "/api/expenses", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("missing content type: %d; want 415", w.Code)
	}
	w = request(router, http.MethodPost, "/api/expenses", `{"comment":"`+strings.Repeat("a", 17<<10)+`"}`)
	if w.Code != http.StatusRequestEntityTooLarge || store.calls != 0 {
		t.Fatalf("large body: status %d, calls %d; want 413 and no calls", w.Code, store.calls)
	}
}

func TestStorageErrors(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{err: expense.ErrNotFound, status: 404, code: "expense_not_found"},
		{err: expense.ErrUnknownUser, status: 400, code: "unknown_user"},
		{err: errors.New("postgres://secret:password@host"), status: 500, code: "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			store := &fakeStore{err: test.err}
			w := request(testRouter(store), http.MethodPost, "/api/expenses",
				`{"user_id":42,"amount":"1","category":"еда","spent_at":"2026-10-20"}`)
			if w.Code != test.status || !strings.Contains(w.Body.String(), test.code) ||
				strings.Contains(w.Body.String(), "password") {
				t.Fatalf("status %d, body %s; want %d %s without internals", w.Code, w.Body, test.status, test.code)
			}
		})
	}
}
