package frames

import (
	"reflect"
	"testing"

	"github.com/cliftonz/typesense-datasource/pkg/typesense"
)

func facetFloat(v float64) *float64 { return &v }

func facetInt64(v int64) *int64 { return &v }

func facetInt(v int) *int { return &v }

func sampleFacets() []typesense.FacetField {
	return []typesense.FacetField{
		{
			FieldName: "price",
			Counts:    []typesense.FacetCount{{Value: "10", Count: 3}, {Value: "5", Count: 1}},
			Stats:     typesense.FacetStats{Min: facetFloat(5), Max: facetFloat(10), Sum: facetFloat(35), Avg: facetFloat(8.75), TotalValues: facetInt(2)},
		},
		{
			FieldName: "brand",
			Counts:    []typesense.FacetCount{{Value: "acme", Count: 7}, {Value: "zeta", Count: 2}},
			Stats:     typesense.FacetStats{},
		},
	}
}

func TestFacetFramesOnePerFieldInTypesenseOrder(t *testing.T) {
	frames := FacetFrames(sampleFacets())

	if len(frames) != 3 {
		t.Fatalf("frame count = %d, want 3", len(frames))
	}
	cases := []struct {
		name   string
		values []any
		counts []int64
	}{
		{"price", []any{facetInt64(10), facetInt64(5)}, []int64{3, 1}},
		{"brand", []any{"acme", "zeta"}, []int64{7, 2}},
	}
	for i, c := range cases {
		f := frames[i]
		if f.Name != c.name {
			t.Errorf("frame %d name = %q, want %q", i, f.Name, c.name)
		}
		if f.Fields[0].Name != "value" || f.Fields[1].Name != "count" {
			t.Errorf("frame %q columns = %q, %q", c.name, f.Fields[0].Name, f.Fields[1].Name)
		}
		var values []any
		var counts []int64
		for r := 0; r < f.Rows(); r++ {
			values = append(values, f.Fields[0].At(r))
			counts = append(counts, f.Fields[1].At(r).(int64))
		}
		if !reflect.DeepEqual(values, c.values) || !reflect.DeepEqual(counts, c.counts) {
			t.Errorf("frame %q rows = %v %v, want %v %v", c.name, values, counts, c.values, c.counts)
		}
	}
}

func TestFacetStatsFrameOneRowPerFieldWithNullsForOmittedStats(t *testing.T) {
	frames := FacetFrames(sampleFacets())
	stats := frames[len(frames)-1]

	if stats.Name != "facet stats" {
		t.Fatalf("stats frame name = %q", stats.Name)
	}
	var columns []string
	for _, f := range stats.Fields {
		columns = append(columns, f.Name)
	}
	if want := []string{"field", "min", "max", "sum", "avg", "total_values"}; !reflect.DeepEqual(columns, want) {
		t.Fatalf("columns = %v, want %v", columns, want)
	}
	want := [][]any{
		{"price", facetFloat(5), facetFloat(10), facetFloat(35), facetFloat(8.75), facetInt64(2)},
		{"brand", (*float64)(nil), (*float64)(nil), (*float64)(nil), (*float64)(nil), (*int64)(nil)},
	}
	if stats.Rows() != len(want) {
		t.Fatalf("stats rows = %d, want %d", stats.Rows(), len(want))
	}
	for r, row := range want {
		for c, v := range row {
			if got := stats.Fields[c].At(r); !reflect.DeepEqual(got, v) {
				t.Errorf("row %d column %s = %v, want %v", r, columns[c], got, v)
			}
		}
	}
}

func TestFacetFramesEmptyInputGivesNoFrames(t *testing.T) {
	if frames := FacetFrames(nil); len(frames) != 0 {
		t.Fatalf("frame count = %d, want 0", len(frames))
	}
}
