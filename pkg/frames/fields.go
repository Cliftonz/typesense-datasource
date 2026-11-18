package frames

import (
	"encoding/json"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func typedNullableField(name string, values []any) *data.Field {
	switch {
	case everyNonNullValue(values, isWholeNumber):
		return data.NewField(name, nil, convertNonNullValues(values, func(v any) int64 {
			n, err := v.(json.Number).Int64()
			if err != nil {
				panic(err)
			}
			return n
		}))
	case everyNonNullValue(values, isFiniteNumber):
		return data.NewField(name, nil, convertNonNullValues(values, func(v any) float64 {
			f, err := v.(json.Number).Float64()
			if err != nil {
				panic(err)
			}
			return f
		}))
	case everyNonNullValue(values, isBool):
		return data.NewField(name, nil, convertNonNullValues(values, func(v any) bool {
			return v.(bool)
		}))
	default:
		return data.NewField(name, nil, convertNonNullValues(values, func(v any) string {
			if s, ok := v.(string); ok {
				return s
			}
			var text strings.Builder
			encoder := json.NewEncoder(&text)
			encoder.SetEscapeHTML(false)
			if err := encoder.Encode(v); err != nil {
				panic(err)
			}
			return strings.TrimSuffix(text.String(), "\n")
		}))
	}
}

func isWholeNumber(v any) bool {
	n, ok := v.(json.Number)
	if !ok {
		return false
	}
	_, err := n.Int64()
	return err == nil
}

func isFiniteNumber(v any) bool {
	n, ok := v.(json.Number)
	if !ok {
		return false
	}
	_, err := n.Float64()
	return err == nil
}

func isBool(v any) bool {
	_, ok := v.(bool)
	return ok
}

func everyNonNullValue(values []any, pred func(any) bool) bool {
	seen := false
	for _, v := range values {
		if v == nil {
			continue
		}
		if !pred(v) {
			return false
		}
		seen = true
	}
	return seen
}

func convertNonNullValues[T any](values []any, convert func(any) T) []*T {
	cells := make([]*T, len(values))
	for i, v := range values {
		if v != nil {
			c := convert(v)
			cells[i] = &c
		}
	}
	return cells
}
