package frames

import (
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func TestTypedNullableFieldWithNoValuesIsString(t *testing.T) {
	field := typedNullableField("k", nil)

	if field.Type() != data.FieldTypeNullableString {
		t.Errorf("type = %v, want %v", field.Type(), data.FieldTypeNullableString)
	}
}
