package expense

import "testing"

func TestFilterValidate(t *testing.T) {
	tests := []struct {
		name    string
		filter  Filter
		wantErr bool
	}{
		{name: "empty"},
		{name: "inclusive day", filter: Filter{From: "2026-10-20", Through: "2026-10-20"}},
		{name: "leap day", filter: Filter{From: "2024-02-29"}},
		{name: "invalid day", filter: Filter{From: "2026-02-29"}, wantErr: true},
		{name: "year zero", filter: Filter{From: "0000-01-01"}, wantErr: true},
		{name: "reverse period", filter: Filter{From: "2026-10-20", Through: "2026-10-19"}, wantErr: true},
		{name: "unknown category", filter: Filter{Category: "travel"}, wantErr: true},
		{name: "negative owner", filter: Filter{UserID: -1}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.filter.Validate(); (err != nil) != test.wantErr {
				t.Errorf("Filter.Validate() = %v; want error: %v", err, test.wantErr)
			}
		})
	}
}

func TestPatchValidate(t *testing.T) {
	empty, zero, fractional, invalidDate := "", "0", "0.01", "2026-02-29"
	tests := []struct {
		name    string
		patch   Patch
		wantErr bool
	}{
		{name: "no fields", wantErr: true},
		{name: "clear comment", patch: Patch{Comment: &empty}},
		{name: "one kopeck", patch: Patch{Amount: &fractional}},
		{name: "zero amount", patch: Patch{Amount: &zero}, wantErr: true},
		{name: "empty category", patch: Patch{Category: &empty}, wantErr: true},
		{name: "invalid date", patch: Patch{SpentAt: &invalidDate}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.patch.Validate(); (err != nil) != test.wantErr {
				t.Errorf("Patch.Validate() = %v; want error: %v", err, test.wantErr)
			}
		})
	}
}

func TestPageNormalize(t *testing.T) {
	page, err := (Page{}).Normalize()
	if err != nil || page.Limit != 50 || page.Offset != 0 {
		t.Fatalf("Page{}.Normalize() = %+v, %v; want limit 50, offset 0", page, err)
	}
	for _, page := range []Page{{Limit: -1}, {Limit: 101}, {Offset: -1}} {
		if _, err := page.Normalize(); err == nil {
			t.Errorf("Page %+v accepted; want validation error", page)
		}
	}
}
