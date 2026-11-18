package plugin

import (
	"testing"
)

func TestTimeFilter(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		want     string
	}{
		{"empty existing filter", "", "ts:>=1700000000 && ts:<1700003600"},
		{"existing filter wrapped in parens", "status:=active", "(status:=active) && ts:>=1700000000 && ts:<1700003600"},
		{"whitespace-only existing filter treated as empty", "  ", "ts:>=1700000000 && ts:<1700003600"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := withTimeFilter(tt.existing, "ts", 1700000000, 1700003600)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
