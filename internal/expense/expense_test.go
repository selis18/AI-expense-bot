package expense

import "testing"

func TestParseAmount(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int64
		valid bool
	}{
		{"350", 35000, true}, {"0.01", 1, true}, {"12,5", 1250, true}, {"92233720368547758.07", 9223372036854775807, true},
		{"92233720368547758.08", 0, false}, {"0", 0, false}, {"-1", 0, false}, {"1.234", 0, false}, {"NaN", 0, false}, {"1e3", 0, false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseAmount(tc.input)
			if (err == nil) != tc.valid || got != tc.want {
				t.Fatalf("got %d, %v; want %d, valid %v", got, err, tc.want, tc.valid)
			}
		})
	}
}

func TestValidateCalendarDate(t *testing.T) {
	for _, date := range []string{"2026-02-29", "0000-01-01", "2026-13-01"} {
		if _, err := (Input{Amount: "1", Category: "еда", SpentAt: date}).Validate(); err == nil {
			t.Errorf("accepted %s", date)
		}
	}
	if _, err := (Input{Amount: "1", Category: "еда", SpentAt: "2024-02-29"}).Validate(); err != nil {
		t.Fatal(err)
	}
}
