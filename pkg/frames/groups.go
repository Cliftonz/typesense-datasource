package frames

import (
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/cliftonz/typesense-datasource/pkg/typesense"
)

func GroupsFrame(name string, groupBy []string, groups []typesense.GroupedHit) *data.Frame {
	frame := data.NewFrame(name)
	for i, field := range groupBy {
		values := make([]any, len(groups))
		for j, g := range groups {
			values[j] = g.GroupKey[i]
		}
		frame.Fields = append(frame.Fields, typedNullableField(field, values))
	}
	found := make([]int64, len(groups))
	for i, g := range groups {
		found[i] = int64(g.Found)
	}
	frame.Fields = append(frame.Fields, data.NewField("found", nil, found))
	return frame
}
