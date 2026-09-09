package inventory

import "testing"

func TestSelectNamed(t *testing.T) {
	for _, tt := range []struct {
		name  string
		items []string
		want  int
	}{
		{"exact", []string{"Sales", "sales", "sales-old"}, 1},
		{"missing", []string{"Sales", "sales-old"}, 0},
		{"duplicate", []string{"sales", "sales"}, 0},
		{"empty", nil, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			name := "sales"
			r := &Reader{Name: &name, inv: &Inventory{Resources: []string{"catalogs"}}}
			got := selectNamed(r, tt.items, func(item string) string { return item })
			if len(got) != tt.want || r.inv.Complete() != (tt.want == 1) {
				t.Fatalf("selected %v, issues %+v", got, r.inv.Issues)
			}
		})
	}
}
