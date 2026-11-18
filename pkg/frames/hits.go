package frames

import (
	"sort"

	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/cliftonz/typesense-datasource/pkg/typesense"
)

func HitsFrame(name string, hits []typesense.Hit) *data.Frame {
	keySet := map[string]bool{}
	for _, h := range hits {
		for k := range h.Document {
			keySet[k] = true
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	frame := data.NewFrame(name)
	for _, k := range keys {
		values := make([]any, len(hits))
		for i, h := range hits {
			values[i] = h.Document[k]
		}
		frame.Fields = append(frame.Fields, typedNullableField(k, values))
	}
	return frame
}
