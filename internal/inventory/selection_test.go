package inventory

import (
	"slices"
	"testing"
)

func TestSelectResources(t *testing.T) {
	cases := []struct {
		name    string
		input   []string
		shared  bool
		want    []string
		invalid bool
	}{
		{"default", nil, false, ResourceNames, false},
		{"shared", nil, true, ResourceNames[:3], false},
		{"order", []string{"warehouses", "catalogs", "warehouses"}, false,
			[]string{"catalogs", "warehouses"}, false},
		{"unknown", []string{"jobs"}, false, nil, true},
		{"wrong scope", []string{"warehouses"}, true, nil, true},
		{"empty", []string{}, false, nil, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SelectResources(tt.input, tt.shared)
			if (err != nil) != tt.invalid || !slices.Equal(got, tt.want) {
				t.Fatalf("selection %v, error %v", got, err)
			}
		})
	}
}
