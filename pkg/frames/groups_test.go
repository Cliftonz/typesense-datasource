package frames

import (
	"encoding/json"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/cliftonz/typesense-datasource/pkg/typesense"
)

func TestGroupsFrameSingleStringKey(t *testing.T) {
	frame := GroupsFrame("groups", []string{"brand"}, []typesense.GroupedHit{
		{GroupKey: []any{"acme"}, Found: 3},
		{GroupKey: []any{"zeta"}, Found: 1},
	})

	if frame.Name != "groups" {
		t.Fatalf("name = %q, want groups", frame.Name)
	}
	brand := frame.Fields[0]
	if brand.Name != "brand" || brand.Len() != 2 {
		t.Fatalf("first field = %s with %d rows, want brand with 2", brand.Name, brand.Len())
	}
	if got := *brand.At(0).(*string); got != "acme" {
		t.Errorf("brand[0] = %q, want acme", got)
	}
	if got := *brand.At(1).(*string); got != "zeta" {
		t.Errorf("brand[1] = %q, want zeta", got)
	}
}

func TestGroupsFrameNumericKeyBecomesInt64Column(t *testing.T) {
	frame := GroupsFrame("groups", []string{"year"}, []typesense.GroupedHit{
		{GroupKey: []any{json.Number("2024")}, Found: 2},
		{GroupKey: []any{json.Number("2025")}, Found: 5},
	})

	year := frame.Fields[0]
	if got := *year.At(0).(*int64); got != 2024 {
		t.Errorf("year[0] = %d, want 2024", got)
	}
	if got := *year.At(1).(*int64); got != 2025 {
		t.Errorf("year[1] = %d, want 2025", got)
	}
}

func TestGroupsFrameTwoGroupByFieldsGiveKeyColumnsInOrder(t *testing.T) {
	frame := GroupsFrame("groups", []string{"brand", "year"}, []typesense.GroupedHit{
		{GroupKey: []any{"acme", json.Number("2024")}, Found: 2},
	})

	if frame.Fields[0].Name != "brand" || frame.Fields[1].Name != "year" {
		t.Fatalf("key columns = %s, %s, want brand, year", frame.Fields[0].Name, frame.Fields[1].Name)
	}
	if got := *frame.Fields[0].At(0).(*string); got != "acme" {
		t.Errorf("brand[0] = %q, want acme", got)
	}
	if got := *frame.Fields[1].At(0).(*int64); got != 2024 {
		t.Errorf("year[0] = %d, want 2024", got)
	}
}

func TestGroupsFrameFoundColumnHoldsGroupCounts(t *testing.T) {
	frame := GroupsFrame("groups", []string{"brand"}, []typesense.GroupedHit{
		{GroupKey: []any{"acme"}, Found: 3},
		{GroupKey: []any{"zeta"}, Found: 1},
	})

	if len(frame.Fields) != 2 {
		t.Fatalf("field count = %d, want 2", len(frame.Fields))
	}
	found := frame.Fields[1]
	if found.Name != "found" {
		t.Fatalf("second field = %s, want found", found.Name)
	}
	if got := found.At(0).(int64); got != 3 {
		t.Errorf("found[0] = %d, want 3", got)
	}
	if got := found.At(1).(int64); got != 1 {
		t.Errorf("found[1] = %d, want 1", got)
	}
}

func TestGroupsFrameEmptyGroupsHasStringKeyColumnsAndFoundButNoRows(t *testing.T) {
	frame := GroupsFrame("groups", []string{"brand", "year"}, nil)

	names := []string{}
	for _, f := range frame.Fields {
		names = append(names, f.Name)
	}
	if len(names) != 3 || names[0] != "brand" || names[1] != "year" || names[2] != "found" {
		t.Fatalf("columns = %v, want [brand year found]", names)
	}
	for _, key := range frame.Fields[:2] {
		if key.Type() != data.FieldTypeNullableString {
			t.Errorf("%s type = %s, want %s", key.Name, key.Type(), data.FieldTypeNullableString)
		}
	}
	rows, err := frame.RowLen()
	if err != nil || rows != 0 {
		t.Errorf("rows = %d, err = %v, want 0 rows", rows, err)
	}
}
