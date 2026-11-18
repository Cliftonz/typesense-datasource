package typesense

type SearchResult struct {
	Found        int          `json:"found"`
	Hits         []Hit        `json:"hits"`
	FacetCounts  []FacetField `json:"facet_counts"`
	GroupedHits  []GroupedHit `json:"grouped_hits"`
	Code         int          `json:"code"`
	Error        string       `json:"error"`
	SearchCutoff bool         `json:"search_cutoff"`
}

type Hit struct {
	Document map[string]any `json:"document"`
}

type FacetField struct {
	FieldName string       `json:"field_name"`
	Counts    []FacetCount `json:"counts"`
	Stats     FacetStats   `json:"stats"`
}

type FacetCount struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type FacetStats struct {
	Min         *float64 `json:"min"`
	Max         *float64 `json:"max"`
	Sum         *float64 `json:"sum"`
	Avg         *float64 `json:"avg"`
	TotalValues *int     `json:"total_values"`
}

type GroupedHit struct {
	GroupKey []any `json:"group_key"`
	Found    int   `json:"found"`
	Hits     []Hit `json:"hits"`
}
