package frames

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/cliftonz/typesense-datasource/pkg/typesense"
)

type absent struct{}

func hit(doc map[string]any) typesense.Hit {
	return typesense.Hit{Document: doc}
}

func cells(f *data.Field) []any {
	out := make([]any, f.Len())
	for i := range out {
		if !f.NilAt(i) {
			out[i], _ = f.ConcreteAt(i)
		}
	}
	return out
}

func TestHitsFrameColumnsAreAlphabeticalAcrossAllHits(t *testing.T) {
	frame := HitsFrame("A", []typesense.Hit{
		hit(map[string]any{"zeta": "z", "alpha": "a"}),
		hit(map[string]any{"mid": "m"}),
	})

	var got []string
	for _, f := range frame.Fields {
		got = append(got, f.Name)
	}
	if want := []string{"alpha", "mid", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("columns = %v, want %v", got, want)
	}
	if frame.Name != "A" {
		t.Errorf("frame name = %q, want A", frame.Name)
	}
}

func TestHitsFrameFromNoHitsHasNoFields(t *testing.T) {
	frame := HitsFrame("A", nil)

	if len(frame.Fields) != 0 {
		t.Errorf("fields = %d, want 0", len(frame.Fields))
	}
}

func TestHitsFrameEmptyDocumentIsRowOfNulls(t *testing.T) {
	frame := HitsFrame("A", []typesense.Hit{
		hit(map[string]any{"a": "x", "b": true}),
		hit(map[string]any{}),
	})

	for _, f := range frame.Fields {
		if f.Len() != 2 || !f.NilAt(1) {
			t.Errorf("field %s: len = %d, want 2 with null at row 1", f.Name, f.Len())
		}
	}
}

func TestHitsFrameColumnTyping(t *testing.T) {
	cases := []struct {
		name     string
		values   []any
		wantType data.FieldType
		want     []any
	}{
		{"missing and null become null", []any{"a", nil, absent{}}, data.FieldTypeNullableString, []any{"a", nil, nil}},
		{"whole numbers are exact int64", []any{json.Number("9007199254740993"), nil, json.Number("-4")}, data.FieldTypeNullableInt64, []any{int64(9007199254740993), nil, int64(-4)}},
		{"decimals are float64", []any{json.Number("1.5"), nil, json.Number("-0.25")}, data.FieldTypeNullableFloat64, []any{1.5, nil, -0.25}},
		{"whole numbers mixed with decimals are float64", []any{json.Number("2"), json.Number("2.5")}, data.FieldTypeNullableFloat64, []any{2.0, 2.5}},
		{"numbers mixed with strings are string", []any{json.Number("1.50"), "x", nil}, data.FieldTypeNullableString, []any{"1.50", "x", nil}},
		{"bools are bool", []any{true, nil, false}, data.FieldTypeNullableBool, []any{true, nil, false}},
		{"nested objects and arrays are JSON text", []any{map[string]any{"b": json.Number("1"), "a": []any{"x"}}, []any{json.Number("2.0"), nil}}, data.FieldTypeNullableString, []any{`{"a":["x"],"b":1}`, `[2.0,null]`}},
		{"objects mixed with scalars are JSON text", []any{map[string]any{}, json.Number("3"), true, nil}, data.FieldTypeNullableString, []any{`{}`, "3", "true", nil}},
		{"null in every hit is string", []any{nil, nil}, data.FieldTypeNullableString, []any{nil, nil}},
		{"strings mixed with objects stay raw", []any{"x", []any{"y"}}, data.FieldTypeNullableString, []any{"x", `["y"]`}},
		{"bools mixed with numbers and strings are text", []any{true, json.Number("1"), "x", false}, data.FieldTypeNullableString, []any{"true", "1", "x", "false"}},
		{"whole numbers beyond int64 and exponents are float64", []any{json.Number("10000000000000000000"), json.Number("1e3")}, data.FieldTypeNullableFloat64, []any{1e19, 1000.0}},
		{"numbers overflowing float64 keep their text", []any{json.Number("1e400"), json.Number("2")}, data.FieldTypeNullableString, []any{"1e400", "2"}},
		{"JSON text keeps HTML characters literal", []any{map[string]any{"q": "<a&b>"}}, data.FieldTypeNullableString, []any{`{"q":"<a&b>"}`}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var hits []typesense.Hit
			for _, v := range c.values {
				if v == (absent{}) {
					hits = append(hits, hit(map[string]any{}))
					continue
				}
				hits = append(hits, hit(map[string]any{"col": v}))
			}

			field := HitsFrame("A", hits).Fields[0]

			if field.Type() != c.wantType {
				t.Errorf("type = %v, want %v", field.Type(), c.wantType)
			}
			if got := cells(field); !reflect.DeepEqual(got, c.want) {
				t.Errorf("cells = %#v, want %#v", got, c.want)
			}
		})
	}
}
