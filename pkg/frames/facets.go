package frames

import (
	"encoding/json"

	"github.com/cliftonz/typesense-datasource/pkg/typesense"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func FacetFrames(facets []typesense.FacetField) data.Frames {
	if len(facets) == 0 {
		return nil
	}
	var frames data.Frames
	stats := data.NewFrame("facet stats",
		data.NewField("field", nil, []string{}),
		data.NewField("min", nil, []*float64{}),
		data.NewField("max", nil, []*float64{}),
		data.NewField("sum", nil, []*float64{}),
		data.NewField("avg", nil, []*float64{}),
		data.NewField("total_values", nil, []*int64{}),
	)
	for _, facet := range facets {
		counts := make([]int64, len(facet.Counts))
		for i, c := range facet.Counts {
			counts[i] = int64(c.Count)
		}
		frames = append(frames, data.NewFrame(facet.FieldName,
			facetValueField(facet),
			data.NewField("count", nil, counts),
		))
		s := facet.Stats
		stats.AppendRow(facet.FieldName, s.Min, s.Max, s.Sum, s.Avg, nullableInt64(s.TotalValues))
	}
	return append(frames, stats)
}

func facetValueField(facet typesense.FacetField) *data.Field {
	if facet.Stats.Min != nil {
		values := make([]any, len(facet.Counts))
		for i, c := range facet.Counts {
			values[i] = json.Number(c.Value)
		}
		return typedNullableField("value", values)
	}
	values := make([]string, len(facet.Counts))
	for i, c := range facet.Counts {
		values[i] = c.Value
	}
	return data.NewField("value", nil, values)
}

func nullableInt64(v *int) *int64 {
	if v == nil {
		return nil
	}
	n := int64(*v)
	return &n
}
