# Changelog

## 1.0.0 (Unreleased)

First release of the Typesense data source for Grafana.

- Connect to open-source Typesense v27 or later. The API key is stored encrypted.
- **Save & test** checks the connection, reports how many collections the key can list, and runs a search on the default collection when one is set.
- Builder mode: pick a collection from a list and fill in query, filter, sort, facet, and group fields.
- Raw mode: write a Typesense `multi_search` request body as JSON.
- Table output of search hits, facet counts, and groups.
- Time series output with three bucket modes: Hits, Range facet, and Group by.
- Count per time bucket, with optional sum, average, minimum, and maximum of a numeric field in Hits and Range facet modes.
- Time buckets follow the dashboard time zone, and empty buckets are shown as 0.
- Template variables work in every builder field and in the raw body.
- `$__fromTime` and `$__toTime` macros insert the dashboard time range in raw mode.
- Queries can be used in Grafana alert rules.
