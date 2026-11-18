package plugin

import (
	"fmt"
	"strings"
)

func withTimeFilter(existing, field string, start, end int64) string {
	timeFilter := fmt.Sprintf("%s:>=%d && %s:<%d", field, start, field, end)
	existing = strings.TrimSpace(existing)
	if existing == "" {
		return timeFilter
	}
	return "(" + existing + ") && " + timeFilter
}
