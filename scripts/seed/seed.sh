#!/bin/sh
set -eo pipefail

collections=http://typesense30:8108/collections
api_key_header="X-TYPESENSE-API-KEY: test-key"

status=$(curl -sS -o /dev/null -w '%{http_code}' -X DELETE -H "$api_key_header" "$collections/orders")
case $status in
  2??|404) ;;
  *) echo "delete orders: HTTP $status" >&2; exit 1 ;;
esac

curl -sSf -X POST -H "$api_key_header" -H 'Content-Type: application/json' "$collections" -d '{
  "name": "orders",
  "fields": [
    {"name": "ts", "type": "int64", "facet": true},
    {"name": "hour", "type": "int64", "facet": true},
    {"name": "price", "type": "float", "facet": true},
    {"name": "brand", "type": "string", "facet": true},
    {"name": "category", "type": "string", "facet": true},
    {"name": "title", "type": "string"}
  ]
}' > /dev/null

result=$(awk -v now="$(date +%s)" 'BEGIN {
  srand(42)
  split("Acme Globex Initech Umbrella Hooli Vandelay", brands, " ")
  split("shoes shirts hats bags watches", categories, " ")
  split("60 25 20 45 150", bases, " ")
  for (i = 0; i < 5000; i++) {
    ts = now - int(rand() * 604800)
    b = 1 + int(rand() * 6)
    c = 1 + int(rand() * 5)
    price = bases[c] * (0.5 + rand())
    printf "{\"ts\":%d,\"hour\":%d,\"price\":%.2f,\"brand\":\"%s\",\"category\":\"%s\",\"title\":\"%s %s\"}\n", ts, ts - ts % 3600, price, brands[b], categories[c], brands[b], categories[c]
  }
}' | curl -sSf -X POST -H "$api_key_header" --data-binary @- "$collections/orders/documents/import?action=create")

case $result in
  *'"success":false'*) echo "$result" | grep '"success":false' >&2; exit 1 ;;
esac
