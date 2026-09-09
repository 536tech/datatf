package contract

import (
	"encoding/json"
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func TestToCtyPreservesJSONValues(t *testing.T) {
	input := map[string]any{
		"number": json.Number("9007199254740993"),
		"items":  []any{nil, true, "${literal}", []any{}, map[string]any{}},
	}
	want := cty.ObjectVal(map[string]cty.Value{
		"number": cty.NumberIntVal(9007199254740993),
		"items": cty.TupleVal([]cty.Value{
			cty.NullVal(cty.DynamicPseudoType), cty.True, cty.StringVal("${literal}"),
			cty.EmptyTupleVal, cty.EmptyObjectVal,
		}),
	})
	got, err := toCty(input)
	if err != nil || !got.RawEquals(want) {
		t.Fatalf("JSON values changed: %s, error %v", got.GoString(), err)
	}
	if _, err := toCty(make(chan int)); err == nil {
		t.Fatal("unsupported JSON input must fail")
	}
}
